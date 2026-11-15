// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/otfabric/go-modbus"
	sunspec "github.com/otfabric/go-sunspec"
	"github.com/otfabric/go-sunspec/registry"
)

func TestShift(t *testing.T) {
	for _, c := range []struct {
		in   string
		exp  int
		want string
	}{
		{"500", 1, "5000"}, {"-30", 2, "-3000"}, {"6001", -2, "60.01"}, {"5", -3, "0.005"},
		{"-985", -1, "-98.5"}, {"0", -2, "0.00"}, {"1.5", -1, "1.5"},
	} {
		if got := shift(json.Number(c.in), c.exp); string(got) != c.want {
			t.Errorf("shift(%s, %d) = %s, want %s", c.in, c.exp, got, c.want)
		}
	}
}

func TestNumber(t *testing.T) {
	for in, want := range map[string]uint64{"1502": 1502, "0x5DE": 1502, "0X10": 16} {
		if got, err := number(in, 16); err != nil || got != want {
			t.Errorf("number(%s) = %d, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "-1", "65536", "1e3", "0x"} {
		if _, err := number(in, 16); err == nil {
			t.Errorf("number(%q) has no error", in)
		}
	}
}

// The raw values of a fixture are what go-sunspec serves: scaled points pass
// through engineering values and back, with scale factors of either sign, in
// a repeating group too.
func TestFixtureRawValuesAreServed(t *testing.T) {
	const doc = `{"name": "probe", "unitId": 3, "baseAddress": 40000, "models": [
	 {"ID": 1, "Mn": "OT Fabric", "Md": "Probe", "SN": "SN-1", "DA": 3},
	 {"ID": 103, "A": 123, "A_SF": -1, "W": 500, "W_SF": 1, "Hz": 6001, "Hz_SF": -2, "WH": 1234567, "WH_SF": 0, "VAr": -30, "VAr_SF": 1, "St": 4},
	 {"ID": 160, "DCA_SF": -2, "DCV_SF": -1, "DCW_SF": 0, "N": 2,
	  "module": [{"ID": 1, "DCA": 1011, "DCV": 4005}, {"ID": 2, "DCA": 999, "DCV": 3990}]}
	]}`
	dir := filepath.Join(t.TempDir(), "probe")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	fx, err := loadFixture(path)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := fx.server()
	if err != nil {
		t.Fatal(err)
	}
	regs, err := registerMap(srv)
	if err != nil {
		t.Fatal(err)
	}
	if regs[0] != 0x5375 || regs[1] != 0x6E53 || regs[len(regs)-2] != 0xFFFF || regs[len(regs)-1] != 0 {
		t.Fatalf("no marker or end model: %v", regs)
	}
	for i, m := range fx.Models {
		dm, err := srv.Model(i)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := groupDoc(dm.Group, nil)
		for name, want := range m {
			if name == "module" {
				continue
			}
			if got := fmt.Sprint(raw[name]); got != fmt.Sprint(want) {
				t.Errorf("model %v %s: served %s, fixture %v", m["ID"], name, got, want)
			}
		}
	}
	dm, _ := srv.Model(2)
	raw, _ := groupDoc(dm.Group, registry.ByID(160).Group)
	modules, _ := raw["module"].([]any)
	if len(modules) != 2 || fmt.Sprint(modules[1].(map[string]any)["DCA"]) != "999" || fmt.Sprint(modules[0].(map[string]any)["DCV"]) != "4005" {
		t.Errorf("module group: %v", raw["module"])
	}
}

func TestFixtureErrors(t *testing.T) {
	for name, doc := range map[string]string{
		"broken":  `{ not json`,
		"nomodel": `{"name": "nomodel", "unitId": 1, "baseAddress": 0, "models": []}`,
		"unit":    `{"name": "unit", "unitId": 0, "baseAddress": 0, "models": [{"ID": 1}]}`,
		"other":   `{"name": "elsewhere", "unitId": 1, "baseAddress": 0, "models": [{"ID": 1}]}`,
	} {
		dir := filepath.Join(t.TempDir(), name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "fixture.json")
		if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadFixture(path); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{
		{"no-such-command"}, {"client"}, {"client", "scan"}, {"client", "write", "--host", "x"},
		{"client", "scan", "--host", "x", "--nope", "1"}, {"server", "--nope", "1"}, {"print-fixture", "a", "b"},
	} {
		if code := run(args); code != exitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, exitUsage)
		}
	}
}

// A 64-bit accumulator with scale factor 0 is reported exactly, not through
// a float64.
func TestScaledKeepsExactIntegers(t *testing.T) {
	big := float64(uint64(18446744073709551))
	sf := int16(0)
	raw, scaled := groupDoc(&sunspec.DecodedGroup{Points: []sunspec.DecodedPoint{
		{Name: "TotWhInj", Type: "acc64", RawValue: uint64(18446744073709551), ScaledValue: &big, SFRawValue: &sf, Implemented: true},
		{Name: "W", Type: "int16", RawValue: int16(-30), ScaledValue: ptr(-300.0), Implemented: true},
	}}, nil)
	if scaled["TotWhInj"] != uint64(18446744073709551) || raw["TotWhInj"] != uint64(18446744073709551) {
		t.Errorf("TotWhInj: raw %v, scaled %v", raw["TotWhInj"], scaled["TotWhInj"])
	}
	if scaled["W"] != -300.0 {
		t.Errorf("W: scaled %v", scaled["W"])
	}
}

func ptr(f float64) *float64 { return &f }

func TestRawValue(t *testing.T) {
	for in, want := range map[string]any{"750": json.Number("750"), "-95": json.Number("-95"), "0x10": json.Number("16"), "abc": "abc", "1.5": "1.5", "": ""} {
		if got := rawValue(in); got != want {
			t.Errorf("rawValue(%q) = %#v, want %#v", in, got, want)
		}
	}
	for _, args := range [][]string{
		{"--model", "123", "--point", "W"}, {"--model", "x", "--point", "W", "--value", "1"},
		{"--model", "123", "--point", "W", "--value", "1", "--instance", "-1"},
	} {
		f, err := flags(args, "model", "instance", "point", "value")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parseWrite(f); err == nil {
			t.Errorf("parseWrite(%v) has no error", args)
		}
	}
}

func TestReadFixtureLimits(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big.json")
	if err := os.WriteFile(big, make([]byte, maxFixture+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{big, filepath.Join(dir, "sub"), filepath.Join(dir, "missing.json")} {
		if _, err := readFixture(path); err == nil {
			t.Errorf("readFixture(%s) has no error", path)
		}
	}
	for _, v := range []int64{-1, 65536} {
		if _, err := u16(v); err == nil {
			t.Errorf("u16(%d) has no error", v)
		}
	}
	if _, err := u8(256); err == nil {
		t.Error("u8(256) has no error")
	}
}

// A group the schema says occurs once is an object in the scan document, as
// the fixture writes it; repeating groups, nested ones too, are lists.
func TestGroupDocShapes(t *testing.T) {
	const doc = `{"name": "shapes", "unitId": 1, "baseAddress": 40000, "models": [
	 {"ID": 1, "Mn": "OT Fabric", "Md": "Shapes", "SN": "SN-1"},
	 {"ID": 704, "PF_SF": -3, "PFWInj": {"PF": 950, "Ext": 1}},
	 {"ID": 705, "NPt": 4, "NCrv": 2, "V_SF": -1, "DeptRef_SF": 0, "RspTms_SF": 0, "Crv": [
	  {"ActPt": 4, "Pt": [{"V": 920}, {"V": 980}, {"V": 1020}, {"V": 1080}]},
	  {"ActPt": 3, "Pt": [{"V": 900}, {"V": 1000}, {"V": 1100}, {}]}]}
	]}`
	dir := filepath.Join(t.TempDir(), "shapes")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	fx, err := loadFixture(path)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := fx.server()
	if err != nil {
		t.Fatal(err)
	}
	ctl, _ := srv.Model(1)
	raw, _ := groupDoc(ctl.Group, registry.ByID(704).Group)
	if pf, ok := raw["PFWInj"].(map[string]any); !ok || fmt.Sprint(pf["PF"]) != "950" {
		t.Errorf("PFWInj: %#v", raw["PFWInj"])
	}
	vv, _ := srv.Model(2)
	raw, _ = groupDoc(vv.Group, registry.ByID(705).Group)
	curves, ok := raw["Crv"].([]any)
	if !ok || len(curves) != 2 {
		t.Fatalf("Crv: %#v", raw["Crv"])
	}
	points, ok := curves[1].(map[string]any)["Pt"].([]any)
	if !ok || len(points) != 4 || fmt.Sprint(points[2].(map[string]any)["V"]) != "1100" {
		t.Errorf("Crv[1].Pt: %#v", curves[1])
	}
}

// serve serves the fixture document doc as the server command does, on a
// loopback port, until the test ends, and returns the port.
func serve(t *testing.T, name, doc string) int {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	fx, err := loadFixture(path)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := fx.server()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	ms, err := modbus.NewServer(&modbus.ServerConfig{URL: "tcp://127.0.0.1:" + strconv.Itoa(port), MaxClients: 4, Logger: modbus.NopLogger()}, device{srv})
	if err != nil {
		t.Fatal(err)
	}
	if err := ms.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ms.Stop() })
	return port
}

// TestWriteRaw: client write writes the raw value of a scaled point and reads
// it back, whether the device reports the point's scale factor as a usable
// value or not (not implemented, 0, or another exponent), and also while the
// point holds its not-implemented value (OutPFSet is not in the fixture), as
// the contract's raw write and both reference clients do. A raw value the
// point's type cannot hold, or its not-implemented value, is refused (exit 1).
func TestWriteRaw(t *testing.T) {
	for _, c := range []struct {
		name, points, point, value string
		ok                         bool
	}{
		{"scale factor -1", `, "WMaxLimPct_SF": -1`, "WMaxLimPct", "750", true},
		{"scale factor 0", `, "WMaxLimPct_SF": 0`, "WMaxLimPct", "750", true},
		{"scale factor not implemented", ``, "WMaxLimPct", "750", true},
		{"scale factor not implemented, 0", ``, "WMaxLimPct", "0", true},
		{"scale factor not implemented, 1", ``, "WMaxLimPct", "1", true},
		{"scale factor not implemented, largest", ``, "WMaxLimPct", "65534", true},
		{"scale factor not implemented, not-implemented value", ``, "WMaxLimPct", "65535", false},
		{"scale factor not implemented, too large", ``, "WMaxLimPct", "65536", false},
		{"scale factor not implemented, negative", ``, "WMaxLimPct", "-1", false},
		{"signed, scale factor -3", `, "OutPFSet_SF": -3`, "OutPFSet", "-850", true},
		{"signed, scale factor not implemented", ``, "OutPFSet", "-850", true},
		{"signed, scale factor not implemented, hexadecimal", ``, "OutPFSet", "0x7FFF", true},
		{"signed, scale factor not implemented, smallest", ``, "OutPFSet", "-32767", true},
		{"signed, scale factor not implemented, not-implemented value", ``, "OutPFSet", "-32768", false},
		{"signed, scale factor not implemented, too large", ``, "OutPFSet", "32768", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			doc := `{"name": "write", "unitId": 1, "baseAddress": 40000, "models": [
			 {"ID": 1, "Mn": "OT Fabric", "Md": "Probe", "SN": "SN-1"},
			 {"ID": 123, "Conn": 1, "WMaxLimPct": 500, "WMaxLim_Ena": 1` + c.points + `}]}`
			port := serve(t, "write", doc)
			args := []string{"client", "write", "--host", "127.0.0.1", "--port", strconv.Itoa(port),
				"--model", "123", "--point", c.point, "--value", c.value}
			if code := run(args); !c.ok {
				if code != exitFailed {
					t.Fatalf("client write: exit %d, want %d", code, exitFailed)
				}
				return
			} else if code != exitOK {
				t.Fatalf("client write: exit %d", code)
			}
			dev, s, kind, err := connect("127.0.0.1", port, 1, 5*time.Second)
			if err != nil {
				t.Fatalf("connect: %s %v", kind, err)
			}
			defer s.close()
			w, err := parseWrite(map[string]string{"model": "123", "point": c.point, "value": c.value})
			if err != nil {
				t.Fatal(err)
			}
			result, kind, err := write(s.ctx, dev, w)
			if err != nil {
				t.Fatalf("write: %s %v", kind, err)
			}
			want := fmt.Sprint(w.value)
			if got := fmt.Sprint(result.(map[string]any)["value"]); got != want {
				t.Errorf("read back %s, want %s", got, want)
			}
		})
	}
}
