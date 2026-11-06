// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cB-Sigma/sunspec-sim/sim"
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

// image lays out the sunspec-sim image of the fixture: sim's Image.Add
// (cB-Sigma's device.Build) encodes every register from the raw values and
// marks the read-write points writable.
func (fx *fixture) image() (*sim.Image, error) {
	im := sim.NewImage(fx.BaseAddress)
	for i, m := range fx.Models {
		id, err := integer(m["ID"], 1, 65534)
		if err != nil {
			return nil, fmt.Errorf("model %d: ID: %w", i, err)
		}
		values := map[string]any{}
		for name, v := range m {
			if name == "ID" || name == "L" {
				continue
			}
			values[name] = native(v)
		}
		if _, err := im.Add(int(id), values); err != nil {
			return nil, fmt.Errorf("sunspec-sim rejects model %d (%d): %w", i, id, err)
		}
	}
	im.End()
	return im, nil
}

// native turns decoded JSON into the Go values sunspec-sim's Build takes:
// an integer as int64 (uint64 when it does not fit), another number as
// float64, groups as maps and lists of maps.
func native(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := strconv.ParseInt(string(x), 10, 64); err == nil {
			return i
		}
		if u, err := strconv.ParseUint(string(x), 10, 64); err == nil {
			return u
		}
		if f, err := strconv.ParseFloat(string(x), 64); err == nil {
			return f
		}
		return string(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = native(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = native(e)
		}
		return out
	}
	return v
}

// registerMap is the whole map, from the marker through the end model, as
// the image answers reads.
func registerMap(im *sim.Image) ([]uint16, error) {
	n := int(im.Next()) - int(im.Base)
	out := make([]uint16, 0, n)
	for off := 0; off < n; off += 125 {
		addr, err := u16(int64(im.Base) + int64(off))
		if err != nil {
			return nil, err
		}
		q, err := u16(int64(min(125, n-off)))
		if err != nil {
			return nil, err
		}
		regs, err := im.ReadHolding(addr, q)
		if err != nil {
			return nil, fmt.Errorf("reading the map at %d: %w", int(im.Base)+off, err)
		}
		out = append(out, regs...)
	}
	return out, nil
}
