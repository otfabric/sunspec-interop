// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	sunspec "github.com/otfabric/go-sunspec"
)

// writeRequest is what client write was asked to do.
type writeRequest struct {
	model    uint16
	instance int
	point    string
	value    any // json.Number for a number, string otherwise
}

func parseWrite(f map[string]string) (writeRequest, error) {
	var w writeRequest
	for _, name := range []string{"model", "point", "value"} {
		if _, ok := f[name]; !ok {
			return w, fmt.Errorf("write needs --model, --point and --value")
		}
	}
	m, err := number(f["model"], 16)
	if err == nil && m <= math.MaxUint16 {
		w.model = uint16(m)
	} else {
		return w, fmt.Errorf("--model: %s", f["model"])
	}
	w.point = f["point"]
	if v, ok := f["instance"]; ok {
		i, err := number(v, 16)
		if err != nil || i > math.MaxUint16 {
			return w, fmt.Errorf("--instance: %s", v)
		}
		w.instance = int(i) // at most 65535
	}
	w.value = rawValue(f["value"])
	return w, nil
}

// rawValue is a number, decimal or hexadecimal with a 0x prefix, with an
// optional minus sign, or else the text itself.
func rawValue(text string) any {
	sign, digits := "", text
	if s, ok := strings.CutPrefix(text, "-"); ok {
		sign, digits = "-", s
	}
	if u, err := number(digits, 64); err == nil {
		return json.Number(sign + strconv.FormatUint(u, 10))
	}
	return text
}

// write writes one point in raw form with go-sunspec's Device.WritePoint and
// reports the raw value the device holds afterwards. go-sunspec takes the
// engineering value of a scaled point, so a raw number becomes raw * 10^sf
// with the scale factor the device reports, as for the fixtures.
func write(ctx context.Context, dev *sunspec.Device, w writeRequest) (any, string, error) {
	insts := dev.ModelsByID(w.model)
	if w.instance >= len(insts) {
		return nil, "failed", fmt.Errorf("no instance %d of model %d", w.instance, w.model)
	}
	inst := insts[w.instance]
	value := w.value
	if n, ok := value.(json.Number); ok && inst.Schema != nil && inst.Schema.Group != nil {
		p, err := dev.ReadPoint(ctx, inst, w.point)
		if err != nil {
			return nil, kindOf(err), err
		}
		if sf, ok := exponent(inst, p); ok && sf != 0 && sf >= -10 && sf <= 10 {
			value = shift(n, sf)
		}
	}
	p, err := dev.WritePoint(ctx, inst, w.point, value)
	if err != nil {
		if errors.Is(err, sunspec.ErrPointNotWritable) || errors.Is(err, sunspec.ErrPointNotFound) || errors.Is(err, sunspec.ErrEncode) {
			return nil, "failed", err
		}
		return nil, kindOf(err), err
	}
	var v any
	if p.Implemented {
		v = finite(p.RawValue)
	}
	return map[string]any{"model": w.model, "point": w.point, "value": v}, "", nil
}

// exponent is the scale factor go-sunspec applied to a top-level point: the
// raw value of its sunssf point, or the schema's literal exponent.
func exponent(inst sunspec.ModelInstance, p *sunspec.DecodedPoint) (int, bool) {
	if p.SFRawValue != nil {
		return int(*p.SFRawValue), true
	}
	for _, pm := range inst.Schema.Group.Points {
		if pm.Name == p.Name && pm.SFIsLiteral {
			return pm.SFLiteral, true
		}
	}
	return 0, false
}
