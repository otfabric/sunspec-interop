// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cB-Sigma/sunspec"
	"github.com/cB-Sigma/sunspec/mbtcp"
)

type clientError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type clientDoc struct {
	SchemaVersion string       `json:"schemaVersion"`
	Adapter       string       `json:"adapter"`
	Operation     string       `json:"operation"`
	Host          string       `json:"host"`
	Port          int          `json:"port"`
	UnitID        int          `json:"unitId"`
	OK            bool         `json:"ok"`
	Result        any          `json:"result,omitempty"`
	Error         *clientError `json:"error,omitempty"`
}

// failure is an error with the contract's kind.
type failure struct {
	kind string
	err  error
}

func (f *failure) Error() string { return f.err.Error() }

func failed(format string, args ...any) error {
	return &failure{"failed", fmt.Errorf(format, args...)}
}

func runClient(args []string) int {
	if len(args) == 0 {
		return usage("client needs an operation")
	}
	op := args[0]
	known := []string{"host", "port", "unit-id", "timeout"}
	switch op {
	case "scan":
	case "write", "write-group":
		known = append(known, "model", "instance", "point", "value")
	default:
		return usage("unknown operation " + op)
	}
	f, err := flags(args[1:], known...)
	if err != nil {
		return usage(err.Error())
	}
	var w writeRequest
	if op != "scan" {
		if w, err = parseWrite(f, op == "write-group"); err != nil {
			return usage(err.Error())
		}
	}
	host, ok := f["host"]
	if !ok || host == "" {
		return usage("--host is required")
	}
	port, unit, timeout := uint64(1502), uint64(1), 5.0
	if v, ok := f["port"]; ok {
		if port, err = number(v, 16); err != nil {
			return usage("--port: " + v)
		}
	}
	if v, ok := f["unit-id"]; ok {
		if unit, err = number(v, 8); err != nil {
			return usage("--unit-id: " + v)
		}
	}
	if v, ok := f["timeout"]; ok {
		if timeout, err = strconv.ParseFloat(v, 64); err != nil || !(timeout > 0) || math.IsInf(timeout, 0) || timeout > 3600 {
			return usage("--timeout: " + v)
		}
	}

	doc := clientDoc{SchemaVersion: schema, Adapter: adapterName, Operation: op, Host: host, Port: int(port), UnitID: int(unit)}
	result, err := operate(op, host, int(port), uint8(unit), time.Duration(timeout*float64(time.Second)), w)
	code := exitOK
	if err != nil {
		kind := "failed"
		var fe *failure
		if errors.As(err, &fe) {
			kind = fe.kind
		}
		doc.Error = &clientError{Kind: kind, Message: err.Error()}
		code = exitFailed
		if kind == "connect" {
			code = exitConnect
		}
	} else {
		doc.OK, doc.Result = true, result
	}
	if c := emit(doc); c != exitOK {
		return c
	}
	return code
}

// operate connects with cB-Sigma's mbtcp reader, finds the map with FindAt
// at the contract's base addresses, and runs the operation.
func operate(op, host string, port int, unit uint8, timeout time.Duration, w writeRequest) (any, error) {
	r, err := mbtcp.Dial("tcp://"+net.JoinHostPort(host, strconv.Itoa(port)), unit, timeout)
	if err != nil {
		return nil, &failure{"connect", err}
	}
	defer func() { _ = r.Close() }()
	m, err := sunspec.FindAt(r, baseAddresses...)
	if err != nil {
		return nil, &failure{kindOf(err), err}
	}
	if op == "scan" {
		return scan(r, m), nil
	}
	return write(r, m, w)
}

// kindOf tells a device that was not reached (no answer, or no marker) from
// one that refused.
func kindOf(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, sunspec.ErrNotSunSpec), errors.As(err, &ne), errors.Is(err, os.ErrDeadlineExceeded):
		return "connect"
	case mbtcp.IsException(err):
		return "failed"
	}
	return "connect"
}

// scan reads every model with ReadModel and reports what Layout places and
// Decode decodes.
func scan(r *mbtcp.Reader, m *sunspec.Map) any {
	models := make([]map[string]any, 0, len(m.Models))
	for _, inst := range m.Models {
		entry := map[string]any{"id": inst.ID, "name": inst.Name, "address": int(inst.Addr) - 2, "length": inst.L}
		if inst.Def != nil {
			entry["name"] = inst.Def.Name
		}
		regs, err := sunspec.ReadModel(r, inst)
		if err != nil {
			entry["values"], entry["scaled"], entry["error"] = map[string]any{}, map[string]any{}, err.Error()
		} else if inst.Def == nil {
			entry["values"], entry["scaled"], entry["error"] = map[string]any{}, map[string]any{}, fmt.Sprintf("model %d is not described", inst.ID)
		} else {
			entry["values"], entry["scaled"] = modelDoc(inst, regs)
		}
		models = append(models, entry)
	}
	return map[string]any{"baseAddress": m.Base, "models": models}
}

// modelDoc gives the raw and the scaled value of every point Layout places,
// by Path: a group that occurs once is an object, a repeating one a list. A
// point Decode leaves out as not implemented is null; a scaled value whose
// scale factor is not usable is null in scaled.
func modelDoc(inst sunspec.Instance, regs []uint16) (map[string]any, map[string]any) {
	decoded := map[string]sunspec.Value{}
	for _, v := range sunspec.Decode(inst, regs) {
		decoded[v.Path] = v
	}
	// The header is not a point of Layout: ID and L as Walk read them.
	raw := map[string]any{"ID": inst.ID, "L": inst.L}
	scaled := map[string]any{"ID": inst.ID, "L": inst.L}
	for _, l := range sunspec.Layout(inst, regs) {
		if l.Point.Type == "pad" || l.Offset+l.Point.Size > len(regs) {
			continue
		}
		r, s := pointValues(l, regs, decoded)
		place(raw, l.Path, r)
		place(scaled, l.Path, s)
	}
	return raw, scaled
}

// pointValues is the raw and the scaled value of a located point. Integer
// types (scale factors as int16) are read with sunspec.Raw, which also says
// whether the value is the type's not-implemented one; the other types are
// what Decode gives. A scaled value is Decode's, null when Decode leaves the
// point out because its scale factor is not usable.
func pointValues(l sunspec.Located, regs []uint16, decoded map[string]sunspec.Value) (any, any) {
	p := l.Point
	typ := p.Type
	if typ == "sunssf" {
		typ = "int16" // a scale factor is an int16 with the same not-implemented value
	}
	v, isDecoded := decoded[l.Path]
	words := regs[l.Offset : l.Offset+p.Size]
	n, isInt := sunspec.Raw(typ, words)
	if !isInt {
		if isDecoded && !intType(typ) {
			return v.V, v.V // float, string or address
		}
		return nil, nil // not implemented
	}
	var rv any = n
	if typ == "uint64" || typ == "acc64" || typ == "bitfield64" {
		var u uint64 // the same 64 bits as n, unsigned
		for _, x := range words {
			u = u<<16 | uint64(x)
		}
		rv = u
	}
	if _, scaled, _ := l.Scale(regs); !scaled {
		return rv, rv
	}
	if !isDecoded {
		return rv, nil
	}
	return rv, v.V
}

// intType is true for the types sunspec.Raw reads.
func intType(typ string) bool {
	switch typ {
	case "int16", "uint16", "enum16", "bitfield16", "count", "acc16",
		"int32", "uint32", "enum32", "bitfield32", "acc32",
		"int64", "uint64", "acc64", "bitfield64":
		return true
	}
	return false
}

// place sets a value at a Layout path ("W", "PFWInj.PF", "Crv.2.Pt.3.V",
// occurrences counted from 1) in a document of objects and lists.
func place(doc map[string]any, path string, v any) {
	parts := strings.Split(path, ".")
	cur := doc
	for i := 0; i < len(parts)-1; i++ {
		name := parts[i]
		if i+1 < len(parts)-1 {
			if k, err := strconv.Atoi(parts[i+1]); err == nil && k >= 1 {
				list, _ := cur[name].([]any)
				for len(list) < k {
					list = append(list, map[string]any{})
				}
				cur[name] = list
				cur = list[k-1].(map[string]any)
				i++
				continue
			}
		}
		next, ok := cur[name].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[name] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = v
}

// writeRequest is what client write and write-group were asked to do.
type writeRequest struct {
	model    int
	instance int
	name     string // the point as the request names it
	point    string // cB-Sigma's name or Layout path
	value    string
}

func parseWrite(f map[string]string, group bool) (writeRequest, error) {
	var w writeRequest
	for _, name := range []string{"model", "point", "value"} {
		if _, ok := f[name]; !ok {
			return w, fmt.Errorf("write needs --model, --point and --value")
		}
	}
	m, err := number(f["model"], 16)
	if err != nil || m > math.MaxUint16 {
		return w, fmt.Errorf("--model: %s", f["model"])
	}
	w.model = int(m) // at most 65535
	if v, ok := f["instance"]; ok {
		i, err := number(v, 16)
		if err != nil || i > math.MaxUint16 {
			return w, fmt.Errorf("--instance: %s", v)
		}
		w.instance = int(i) // at most 65535
	}
	w.value, w.name = f["value"], f["point"]
	if !group {
		if strings.ContainsAny(f["point"], ".[]") || f["point"] == "" {
			return w, fmt.Errorf("--point: %q is not a top-level point name (write-group takes a path)", f["point"])
		}
		w.point = f["point"]
		return w, nil
	}
	if w.point, err = layoutPath(f["point"]); err != nil {
		return w, fmt.Errorf("--point: %w", err)
	}
	return w, nil
}

// layoutPath turns the contract's path ("Crv[1].Pt[0].V", "PFWInj.PF",
// occurrences counted from 0) into cB-Sigma's ("Crv.2.Pt.1.V").
func layoutPath(text string) (string, error) {
	parts := strings.Split(text, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("%q names no group", text)
	}
	var out []string
	for i, part := range parts {
		name, index, indexed := strings.Cut(part, "[")
		if name == "" || strings.ContainsAny(name, "]") {
			return "", fmt.Errorf("%q: bad segment %q", text, part)
		}
		out = append(out, name)
		if !indexed {
			continue
		}
		digits, ok := strings.CutSuffix(index, "]")
		k, err := strconv.ParseUint(digits, 10, 16)
		if !ok || err != nil || i == len(parts)-1 {
			return "", fmt.Errorf("%q: bad index in %q", text, part)
		}
		out = append(out, strconv.FormatUint(k+1, 10))
	}
	return strings.Join(out, "."), nil
}

// instance is the n-th instance of a model in the map.
func instance(m *sunspec.Map, id, n int) (sunspec.Instance, bool) {
	for _, inst := range m.Models {
		if inst.ID == id {
			if n == 0 {
				return inst, true
			}
			n--
		}
	}
	return sunspec.Instance{}, false
}

// write writes one point in raw form with a sunspec.Write (Encode refuses a
// read-only point before anything is sent), then reads the model again and
// reports the raw value the device holds.
func write(r *mbtcp.Reader, m *sunspec.Map, w writeRequest) (any, error) {
	inst, ok := instance(m, w.model, w.instance)
	if !ok {
		return nil, failed("no instance %d of model %d", w.instance, w.model)
	}
	if inst.Def == nil {
		return nil, failed("model %d is not described", w.model)
	}
	regs, err := sunspec.ReadModel(r, inst)
	if err != nil {
		return nil, &failure{kindOf(err), err}
	}
	l, ok := sunspec.Locate(inst, regs, w.point)
	if !ok || l.Path != w.point && l.Name != w.point {
		return nil, failed("model %d has no point %s", w.model, w.point)
	}
	if !strings.Contains(w.point, ".") && strings.Contains(l.Path, ".") {
		return nil, failed("model %d has no top-level point %s", w.model, w.point)
	}
	wr := sunspec.Write{Model: inst, Point: l.Path, Regs: regs}
	switch t := l.Point.Type; {
	case t == "string" || t == "ipaddr" || t == "ipv6addr" || t == "eui48":
		wr.Value = w.value
	case strings.HasPrefix(t, "float"):
		f, err := strconv.ParseFloat(w.value, 64)
		if err != nil {
			return nil, failed("%s takes a number, not %q", w.point, w.value)
		}
		wr.Value = f
	default:
		n, big, err := rawInteger(w.value)
		if err != nil {
			return nil, failed("%s takes an integer: %v", w.point, err)
		}
		wr.Raw = n
		if big != 0 {
			wr.Value = big // Write encodes a uint64 Value into a 64-bit unsigned point
		}
	}
	addr, words, err := wr.Encode()
	if err != nil {
		return nil, failed("%v", err)
	}
	if err := r.WriteHolding(addr, words); err != nil {
		return nil, &failure{kindOf(err), fmt.Errorf("write %s of model %d: %w", w.point, w.model, err)}
	}
	after, err := sunspec.ReadModel(r, inst)
	if err != nil {
		return nil, &failure{kindOf(err), err}
	}
	decoded := map[string]sunspec.Value{}
	for _, v := range sunspec.Decode(inst, after) {
		decoded[v.Path] = v
	}
	l, ok = sunspec.Locate(inst, after, l.Path)
	if !ok {
		return nil, failed("model %d no longer has %s", w.model, w.point)
	}
	v, _ := pointValues(l, after, decoded)
	return map[string]any{"model": w.model, "point": w.name, "value": v}, nil
}

// rawInteger is a decimal or 0x-prefixed hexadecimal integer with an
// optional minus sign. A number above math.MaxInt64 comes back as big, for a
// 64-bit unsigned point.
func rawInteger(text string) (n int64, big uint64, err error) {
	digits, negative := strings.CutPrefix(text, "-")
	u, err := number(digits, 64)
	switch {
	case err != nil:
		return 0, 0, err
	case u <= math.MaxInt64 && negative:
		return -int64(u), 0, nil
	case u <= math.MaxInt64:
		return int64(u), 0, nil
	case negative:
		return 0, 0, fmt.Errorf("%s does not fit 64 bits", text)
	}
	return 0, u, nil
}
