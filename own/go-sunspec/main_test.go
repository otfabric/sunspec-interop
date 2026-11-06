// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	sunspec "github.com/otfabric/go-sunspec"
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
		raw, _ := groupDoc(dm.Group)
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
	raw, _ := groupDoc(dm.Group)
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
	}})
	if scaled["TotWhInj"] != uint64(18446744073709551) || raw["TotWhInj"] != uint64(18446744073709551) {
		t.Errorf("TotWhInj: raw %v, scaled %v", raw["TotWhInj"], scaled["TotWhInj"])
	}
	if scaled["W"] != -300.0 {
		t.Errorf("W: scaled %v", scaled["W"])
	}
}

func ptr(f float64) *float64 { return &f }
