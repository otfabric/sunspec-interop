// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/otfabric/go-modbus"
	sunspec "github.com/otfabric/go-sunspec"
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

func runClient(args []string) int {
	if len(args) == 0 {
		return usage("client needs an operation")
	}
	op := args[0]
	switch op {
	case "scan":
	case "write":
		// go-sunspec's client does not write (its LIMITATIONS.md), and the
		// adapter does not write registers behind its back.
		return usage("write: go-sunspec's client does not write; clientOperations is [\"scan\"]")
	default:
		return usage("unknown operation " + op)
	}
	f, err := flags(args[1:], "host", "port", "unit-id", "timeout")
	if err != nil {
		return usage(err.Error())
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
		if timeout, err = strconv.ParseFloat(v, 64); err != nil || !(timeout > 0) || math.IsInf(timeout, 0) {
			return usage("--timeout: " + v)
		}
	}

	doc := clientDoc{SchemaVersion: schema, Adapter: adapterName, Operation: op, Host: host, Port: int(port), UnitID: int(unit)}
	result, kind, err := scan(host, int(port), uint8(unit), time.Duration(timeout*float64(time.Second)))
	code := exitOK
	if err != nil {
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

// scan finds the device and reads every model: what go-sunspec's Discover
// and ReadModels return, in the contract's document.
func scan(host string, port int, unit uint8, timeout time.Duration) (any, string, error) {
	client, err := modbus.New(modbus.Config{
		URL: "tcp://" + net.JoinHostPort(host, strconv.Itoa(port)), Timeout: timeout, DialTimeout: timeout, Logger: modbus.NopLogger(),
	})
	if err != nil {
		return nil, "connect", err
	}
	if err := client.Open(); err != nil {
		return nil, "connect", err
	}
	defer func() { _ = client.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 4*timeout+30*time.Second)
	defer cancel()
	dev, err := sunspec.Discover(ctx, client, &sunspec.DiscoverOptions{UnitID: unit, BaseAddresses: baseAddresses})
	if err != nil {
		return nil, kindOf(err), err
	}
	results, _ := dev.ReadModels(ctx, dev.Discovery.Models)
	models := make([]map[string]any, 0, len(results))
	for _, r := range results {
		h := r.Instance.Header
		entry := map[string]any{"id": h.ID, "name": r.Instance.Name, "address": h.StartAddress, "length": h.Length}
		if r.Model != nil {
			if r.Model.Name != "" {
				entry["name"] = r.Model.Name
			}
			if r.Model.Group != nil {
				entry["values"], entry["scaled"] = groupDoc(r.Model.Group)
			}
		}
		if _, ok := entry["values"]; !ok {
			entry["values"], entry["scaled"] = map[string]any{}, map[string]any{}
		}
		if r.Err != nil {
			entry["error"] = r.Err.Error()
		} else if r.Model != nil && len(r.Model.Warnings) > 0 {
			entry["error"] = fmt.Sprint(r.Model.Warnings)
		}
		models = append(models, entry)
	}
	return map[string]any{"baseAddress": dev.Discovery.BaseAddress, "models": models}, "", nil
}

// kindOf tells a device that was not reached from one that refused.
func kindOf(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, sunspec.ErrNotSunSpec), errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, modbus.ErrRequestTimedOut), errors.As(err, &ne), errors.Is(err, os.ErrDeadlineExceeded):
		return "connect"
	}
	return "failed"
}

// groupDoc gives the raw and the scaled values of a group instance by point
// name, nested instances as lists; a point not implemented is null.
func groupDoc(g *sunspec.DecodedGroup) (map[string]any, map[string]any) {
	raw, scaled := map[string]any{}, map[string]any{}
	for _, p := range g.Points {
		if !p.Implemented {
			raw[p.Name], scaled[p.Name] = nil, nil
			continue
		}
		raw[p.Name] = finite(p.RawValue)
		scaled[p.Name] = raw[p.Name]
		// go-sunspec gives a scaled value as a float64. A scale factor of 0
		// leaves the raw integer, which a float64 cannot always hold (a
		// 64-bit accumulator): then the exact raw value is the scaled one.
		if p.ScaledValue != nil && !sameAsInteger(p.RawValue, *p.ScaledValue) {
			scaled[p.Name] = finite(*p.ScaledValue)
		}
	}
	for _, c := range g.Groups {
		r, s := groupDoc(c)
		rl, _ := raw[c.Name].([]any)
		sl, _ := scaled[c.Name].([]any)
		raw[c.Name], scaled[c.Name] = append(rl, r), append(sl, s)
	}
	return raw, scaled
}

// sameAsInteger is true when raw is an integer whose float64 is f: the scale
// factor did nothing.
func sameAsInteger(raw any, f float64) bool {
	switch v := raw.(type) {
	case int8, int16, int32, int64, int, uint8, uint16, uint32, uint64, uint:
		return float64(reflect.ValueOf(v).Convert(reflect.TypeFor[float64]()).Float()) == f
	}
	return false
}

func finite(v any) any {
	switch x := v.(type) {
	case float32:
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return nil
		}
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
	}
	return v
}
