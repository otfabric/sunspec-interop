// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"math"
	"net"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/cB-Sigma/sunspec"
	"github.com/cB-Sigma/sunspec-sim/sim"
	"github.com/cB-Sigma/sunspec/mbtcp"
)

func TestLayoutPath(t *testing.T) {
	for in, want := range map[string]string{
		"PFWInj.PF":      "PFWInj.PF",
		"Crv[1].Pt[0].V": "Crv.2.Pt.1.V",
		"Crv[0].ActPt":   "Crv.1.ActPt",
	} {
		if got, err := layoutPath(in); err != nil || got != want {
			t.Errorf("layoutPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"W", "Crv[x].V", "Crv[1", "Crv.V[0]", ".V", "Crv[].V", "Crv[70000].V"} {
		if got, err := layoutPath(bad); err == nil {
			t.Errorf("layoutPath(%q) = %q, want an error", bad, got)
		}
	}
}

func TestRawInteger(t *testing.T) {
	for _, c := range []struct {
		in  string
		n   int64
		big uint64
	}{
		{"0", 0, 0}, {"-500", -500, 0}, {"0x10", 16, 0},
		{"9223372036854775807", math.MaxInt64, 0},
		{"-9223372036854775807", -math.MaxInt64, 0},
		{"18446744073709551615", 0, math.MaxUint64},
	} {
		n, big, err := rawInteger(c.in)
		if err != nil || n != c.n || big != c.big {
			t.Errorf("rawInteger(%q) = %d, %d, %v", c.in, n, big, err)
		}
	}
	for _, bad := range []string{"", "x", "-18446744073709551615", "1.5", "--1"} {
		if _, _, err := rawInteger(bad); err == nil {
			t.Errorf("rawInteger(%q): no error", bad)
		}
	}
}

func TestPlace(t *testing.T) {
	doc := map[string]any{}
	place(doc, "W", 1)
	place(doc, "PFWInj.PF", 2)
	place(doc, "Crv.2.Pt.1.V", 3)
	want := map[string]any{
		"W":      1,
		"PFWInj": map[string]any{"PF": 2},
		"Crv":    []any{map[string]any{}, map[string]any{"Pt": []any{map[string]any{"V": 3}}}},
	}
	if !reflect.DeepEqual(doc, want) {
		t.Errorf("place: %v", doc)
	}
}

func TestFlagsAndWriteRequests(t *testing.T) {
	if _, err := flags([]string{"--host"}, "host"); err == nil {
		t.Error("a flag without a value")
	}
	if _, err := flags([]string{"--host", "a", "--host", "b"}, "host"); err == nil {
		t.Error("a flag given twice")
	}
	if _, err := flags([]string{"--nope", "1"}, "host"); err == nil {
		t.Error("an unknown flag")
	}
	f := map[string]string{"model": "705", "point": "Crv[1].Pt[0].V", "value": "900", "instance": "0"}
	w, err := parseWrite(f, true)
	if err != nil || w.point != "Crv.2.Pt.1.V" || w.name != "Crv[1].Pt[0].V" || w.model != 705 {
		t.Errorf("parseWrite: %+v, %v", w, err)
	}
	if _, err := parseWrite(f, false); err == nil {
		t.Error("write takes no path")
	}
	if _, err := parseWrite(map[string]string{"model": "1"}, false); err == nil {
		t.Error("write without --point and --value")
	}
	if code := run([]string{"client", "scan", "--host", "x", "--nope", "1"}); code != exitUsage {
		t.Errorf("unknown flag: exit %d", code)
	}
	if code := run([]string{"client", "frobnicate"}); code != exitUsage {
		t.Errorf("unknown operation: exit %d", code)
	}
}

func fixtureFile(name string) string {
	return filepath.Join("..", "..", "fixtures", name, "fixture.json")
}

// serve starts sunspec-sim on a fixture and returns a reader of its unit.
func serve(t *testing.T, name string) (*fixture, *mbtcp.Reader) {
	t.Helper()
	fx, err := loadFixture(fixtureFile(name))
	if err != nil {
		t.Fatal(err)
	}
	im, err := fx.image()
	if err != nil {
		t.Fatal(err)
	}
	dev, err := sim.StartOptions(im, sim.Options{Listen: "127.0.0.1:0", Units: []uint8{fx.UnitID}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dev.Close() })
	r, err := mbtcp.Dial("tcp://"+dev.Addr(), fx.UnitID, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return fx, r
}

// equal compares a fixture value with a document value: numbers by value.
func equal(want, got any) bool {
	switch w := want.(type) {
	case json.Number:
		switch g := got.(type) {
		case int64:
			return string(w) == strconv.FormatInt(g, 10)
		case uint64:
			return string(w) == strconv.FormatUint(g, 10)
		case int:
			return string(w) == strconv.Itoa(g)
		case float64:
			f, err := w.Float64()
			return err == nil && math.Abs(f-g) <= math.Abs(f)*1e-6
		}
		return false
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range w {
			if !equal(v, g[k]) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !equal(w[i], g[i]) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(want, got)
}

func TestScanEveryFixture(t *testing.T) {
	for _, name := range []string{"baseline", "edge", "altbase", "der"} {
		t.Run(name, func(t *testing.T) {
			fx, r := serve(t, name)
			m, err := sunspec.FindAt(r, baseAddresses...)
			if err != nil {
				t.Fatal(err)
			}
			doc := scan(r, m).(map[string]any)
			models := doc["models"].([]map[string]any)
			if doc["baseAddress"] != fx.BaseAddress || len(models) != len(fx.Models) {
				t.Fatalf("base %v, %d models", doc["baseAddress"], len(models))
			}
			for i, want := range fx.Models {
				if _, bad := models[i]["error"]; bad {
					t.Errorf("model %v: %v", models[i]["id"], models[i]["error"])
				}
				for k, v := range want {
					if got := models[i]["values"].(map[string]any)[k]; !equal(v, got) {
						t.Errorf("model %v %s: fixture %v, read %v (%T)", models[i]["id"], k, v, got, got)
					}
				}
			}
		})
	}
}

func TestWriteDER(t *testing.T) {
	_, r := serve(t, "der")
	m, err := sunspec.FindAt(r, baseAddresses...)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		model int
		point string
		group bool
		value string
		want  any
	}{
		{702, "WMax", false, "900", int64(900)},
		{704, "WSetPct", false, "-500", int64(-500)},
		{704, "PFWInj.PF", true, "970", int64(970)},
		{705, "Crv[1].Pt[2].V", true, "1090", int64(1090)},
		{705, "Crv[1].ActPt", true, "4", int64(4)},
	} {
		w, err := parseWrite(map[string]string{"model": strconv.Itoa(c.model), "point": c.point, "value": c.value}, c.group)
		if err != nil {
			t.Fatal(err)
		}
		res, err := write(r, m, w)
		if err != nil {
			t.Errorf("%d %s: %v", c.model, c.point, err)
			continue
		}
		if got := res.(map[string]any); got["value"] != c.want || got["point"] != c.point {
			t.Errorf("%d %s: %v", c.model, c.point, got)
		}
	}
	for _, c := range []struct {
		model int
		point string
		group bool
	}{
		{705, "AdptCrvRslt", false},    // read-only
		{705, "Crv[0].ReadOnly", true}, // read-only, in a group
		{705, "Nope", false},
		{705, "Crv[9].ActPt", true},
		{705, "ActPt", false}, // a group's point is no top-level point
		{999, "W", false},
	} {
		w, err := parseWrite(map[string]string{"model": strconv.Itoa(c.model), "point": c.point, "value": "1"}, c.group)
		if err != nil {
			t.Fatal(err)
		}
		_, err = write(r, m, w)
		if f, ok := err.(*failure); !ok || f.kind != "failed" {
			t.Errorf("%d %s: %v, want a failure of kind failed", c.model, c.point, err)
		}
	}
}

func TestOtherUnitAndNoDevice(t *testing.T) {
	_, r := serve(t, "baseline")
	other := r.ForUnit(9)
	if _, err := sunspec.FindAt(other, baseAddresses...); err == nil {
		t.Error("another unit answered")
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().(*net.TCPAddr)
	_ = l.Close()
	if _, err := operate("scan", "127.0.0.1", addr.Port, 1, 500*time.Millisecond, writeRequest{}); err == nil || err.(*failure).kind != "connect" {
		t.Errorf("no device: %v", err)
	}
}

func TestBadFixtures(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadFixture(filepath.Join(dir, "missing", "fixture.json")); err == nil {
		t.Error("a missing fixture")
	}
	fx := &fixture{BaseAddress: 40000, Models: []map[string]any{{"ID": json.Number("65000")}}}
	if _, err := fx.image(); err == nil {
		t.Error("a model sunspec-sim does not describe")
	}
	fx = &fixture{BaseAddress: 40000, Models: []map[string]any{{"ID": json.Number("1"), "Mn": json.Number("70000")}}}
	if _, err := fx.image(); err == nil {
		t.Error("a number for a string point")
	}
}

func TestNative(t *testing.T) {
	got := native(map[string]any{"a": json.Number("-1"), "b": json.Number("18446744073709551615"), "c": json.Number("1.5"),
		"d": []any{json.Number("2")}})
	want := map[string]any{"a": int64(-1), "b": uint64(math.MaxUint64), "c": 1.5, "d": []any{int64(2)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("native: %v", got)
	}
}

func TestCapabilities(t *testing.T) {
	c := capabilities()
	up := c["upstream"].(map[string]any)
	if c["adapter"] != "cbsigma" || up["independentEngine"] != true || c["contractVersion"] != "1.2" {
		t.Errorf("capabilities: %v", c)
	}
}
