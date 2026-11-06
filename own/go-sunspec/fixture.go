// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/otfabric/go-modbus"
	sunspec "github.com/otfabric/go-sunspec"
	"github.com/otfabric/go-sunspec/registry"
)

// fixture is a fixtures/<name>/fixture.json: models in pysunspec2's device
// JSON, with raw point values (docs/FIXTURES.md).
type fixture struct {
	Name        string
	UnitID      uint8
	BaseAddress uint16
	Models      []map[string]any
	doc         any // as read, for print-fixture
}

func decodeJSON(data []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(into); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("data after the document")
	}
	return nil
}

// maxFixture is the largest fixture read; the baked ones are a few KiB.
const maxFixture = 1 << 20

// readFixture reads the regular file at path, of at most maxFixture bytes,
// through an os.Root of its directory: the file is the one the path names,
// with no symbolic link out of that directory.
func readFixture(path string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	f, err := root.Open(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: not a regular file", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFixture+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFixture {
		return nil, fmt.Errorf("%s: larger than %d bytes", path, maxFixture)
	}
	return data, nil
}

func loadFixture(path string) (*fixture, error) {
	data, err := readFixture(path)
	if err != nil {
		return nil, err
	}
	var doc any
	if err := decodeJSON(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	top, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: not an object", path)
	}
	fx := &fixture{doc: doc}
	if fx.Name, ok = top["name"].(string); !ok {
		return nil, fmt.Errorf("%s: name is missing or has the wrong type", path)
	}
	unit, err := integer(top["unitId"], 1, 255)
	if err != nil {
		return nil, fmt.Errorf("%s: unitId: %w", path, err)
	}
	base, err := integer(top["baseAddress"], 0, 65535)
	if err != nil {
		return nil, fmt.Errorf("%s: baseAddress: %w", path, err)
	}
	if fx.UnitID, err = u8(unit); err != nil {
		return nil, fmt.Errorf("%s: unitId: %w", path, err)
	}
	if fx.BaseAddress, err = u16(base); err != nil {
		return nil, fmt.Errorf("%s: baseAddress: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if filepath.Base(filepath.Dir(abs)) != fx.Name {
		return nil, fmt.Errorf("%s: name %q is not the name of its directory", path, fx.Name)
	}
	models, ok := top["models"].([]any)
	if !ok || len(models) == 0 {
		return nil, fmt.Errorf("%s: no models", path)
	}
	for i, m := range models {
		mm, ok := m.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: model %d is not an object", path, i)
		}
		fx.Models = append(fx.Models, mm)
	}
	return fx, nil
}

func integer(v any, lo, hi int64) (int64, error) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, fmt.Errorf("missing or not a number")
	}
	i, err := strconv.ParseInt(string(n), 10, 64)
	if err != nil || i < lo || i > hi {
		return 0, fmt.Errorf("%s is not an integer from %d to %d", n, lo, hi)
	}
	return i, nil
}

// server lays out the go-sunspec Server of the fixture: go-sunspec encodes
// every register.
func (fx *fixture) server() (*sunspec.Server, error) {
	models := make([]sunspec.ServerModel, len(fx.Models))
	for i, m := range fx.Models {
		n, err := integer(m["ID"], 1, 65534)
		if err != nil {
			return nil, fmt.Errorf("model %d: ID: %w", i, err)
		}
		id, err := u16(n)
		if err != nil {
			return nil, fmt.Errorf("model %d: ID: %w", i, err)
		}
		meta := registry.ByID(id)
		if meta == nil || meta.Group == nil {
			return nil, fmt.Errorf("model %d: go-sunspec has no definition of model %d", i, id)
		}
		values, err := groupValues(meta.Group, m, nil, true)
		if err != nil {
			return nil, fmt.Errorf("model %d (%d): %w", i, id, err)
		}
		models[i] = sunspec.ServerModel{ID: id, Schema: meta, Values: values}
	}
	srv, err := sunspec.NewServer(&sunspec.ServerOptions{UnitID: fx.UnitID, BaseAddress: fx.BaseAddress}, models...)
	if err != nil {
		return nil, fmt.Errorf("go-sunspec rejects the fixture: %w", err)
	}
	return srv, nil
}

// scope holds the scale factors (raw sunssf values) of one group instance and
// the instances that enclose it.
type scope struct {
	sf     map[string]int64
	parent *scope
}

func (s *scope) lookup(name string) (int64, bool) {
	for ; s != nil; s = s.parent {
		if v, ok := s.sf[name]; ok {
			return v, true
		}
	}
	return 0, false
}

// groupValues turns one group instance of the fixture into go-sunspec Values.
// The fixture holds raw values and go-sunspec takes engineering values for a
// scaled point, so a scaled point gets raw * 10^sf, written as an exact
// decimal; the encoder divides it out again. A scale factor go-sunspec does
// not apply (zero, unset, outside -10..10) leaves the raw value as it is.
func groupValues(g *registry.GroupMeta, inst map[string]any, parent *scope, top bool) (*sunspec.Values, error) {
	points := map[string]*registry.PointMeta{}
	for i := range g.Points {
		points[g.Points[i].Name] = &g.Points[i]
	}
	groups := map[string]*registry.GroupMeta{}
	for _, c := range g.Groups {
		groups[c.Name] = c
	}
	sc := &scope{sf: map[string]int64{}, parent: parent}
	for name, v := range inst {
		if pm := points[name]; pm != nil && pm.Type == "sunssf" {
			if i, err := integer(v, -32768, 32767); err == nil {
				sc.sf[name] = i
			}
		}
	}
	values := &sunspec.Values{}
	for name, v := range inst {
		if top && (name == "ID" || name == "L") {
			continue
		}
		if pm := points[name]; pm != nil {
			if v == nil {
				continue
			}
			values.Set(name, engineering(pm, v, sc))
			continue
		}
		child := groups[name]
		if child == nil {
			return nil, fmt.Errorf("%s has no point or group %q", g.Name, name)
		}
		list, ok := v.([]any)
		if !ok {
			list = []any{v}
		}
		for i, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s[%d] is not an object", name, i)
			}
			cv, err := groupValues(child, m, sc, false)
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
			}
			values.Add(name, cv)
		}
	}
	return values, nil
}

func engineering(pm *registry.PointMeta, v any, sc *scope) any {
	n, ok := v.(json.Number)
	if !ok || pm.SF == "" || pm.Type == "sunssf" {
		return v
	}
	var sf int64
	if pm.SFIsLiteral {
		sf = int64(pm.SFLiteral)
	} else if sf, ok = sc.lookup(pm.SF); !ok {
		return v
	}
	if sf == 0 || sf < -10 || sf > 10 {
		return v
	}
	return shift(n, int(sf))
}

// shift multiplies an integer by 10^exp, exactly, as a decimal string.
func shift(n json.Number, exp int) json.Number {
	s := string(n)
	if _, err := strconv.ParseInt(s, 10, 64); err != nil {
		if _, err := strconv.ParseUint(s, 10, 64); err != nil {
			return n // not an integer: go-sunspec judges it
		}
	}
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	if exp > 0 {
		return json.Number(sign + s + strings.Repeat("0", exp))
	}
	digits := -exp
	if len(s) <= digits {
		s = strings.Repeat("0", digits-len(s)+1) + s
	}
	return json.Number(sign + s[:len(s)-digits] + "." + s[len(s)-digits:])
}

// registerMap is the whole map the Server serves, from the base address
// through the end model, read through its own request handler.
func registerMap(srv *sunspec.Server) ([]uint16, error) {
	n := 4
	for _, h := range srv.Models() {
		n += 2 + int(h.Length)
	}
	out := make([]uint16, 0, n)
	for off := 0; off < n; off += 125 {
		addr, err := u16(int64(srv.BaseAddress()) + int64(off))
		if err != nil {
			return nil, err
		}
		q, err := u16(int64(min(125, n-off)))
		if err != nil {
			return nil, err
		}
		regs, err := srv.HandleHoldingRegisters(context.Background(), &modbus.HoldingRegistersRequest{
			UnitID: srv.UnitID(), FunctionCode: 3, Addr: addr, Quantity: q,
		})
		if err != nil {
			return nil, fmt.Errorf("reading the map at %d: %w", int(srv.BaseAddress())+off, err)
		}
		out = append(out, regs...)
	}
	return out, nil
}
