// SPDX-License-Identifier: MIT

// Command sunspec-interop is the sunspec-interop-otfabric image: go-sunspec
// behind the container contract (docs/CONTAINER_CONTRACT.md).
//
// It is OT Fabric's own stack, not an independent implementation: results
// against it are never independent evidence for go-sunspec (README.md).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	adapterName = "otfabric"
	fixtureRoot = "/fixtures"
	contract    = "1.1"
	schema      = "1.0"
)

// Set with -ldflags by the Dockerfile.
var (
	adapterVersion   = "dev"
	upstreamVersion  = "dev"
	upstreamRevision = "dev"
)

const (
	exitOK      = 0
	exitFailed  = 1
	exitUsage   = 2
	exitConnect = 3
	exitFixture = 4
)

// baseAddresses are probed in the contract's order.
var baseAddresses = []uint16{40000, 0, 50000}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return runServer(nil)
	}
	switch args[0] {
	case "server":
		return runServer(args[1:])
	case "client":
		return runClient(args[1:])
	case "print-capabilities":
		if len(args) > 1 {
			return usage("print-capabilities takes no arguments")
		}
		return emit(capabilities())
	case "print-fixture":
		return printFixture(args[1:])
	case "print-registers":
		return printRegisters(args[1:])
	case "version":
		if len(args) > 1 {
			return usage("version takes no arguments")
		}
		fmt.Printf("sunspec-interop %s (go-sunspec %s)\n", adapterVersion, upstreamVersion)
		return exitOK
	}
	return usage("unknown command " + args[0])
}

func usage(message string) int {
	fmt.Fprintln(os.Stderr, "usage error:", message)
	return exitUsage
}

func emit(v any) int {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailed
	}
	fmt.Println(string(b))
	return exitOK
}

// number parses a decimal number, or a hexadecimal one with a 0x prefix.
func number(text string, bits int) (uint64, error) {
	if h, ok := strings.CutPrefix(strings.ToLower(text), "0x"); ok {
		return strconv.ParseUint(h, 16, bits)
	}
	return strconv.ParseUint(text, 10, bits)
}

// fixtureName takes the optional NAME of print-fixture and print-registers.
func fixtureName(args []string) (string, int) {
	switch {
	case len(args) == 0:
		return "baseline", exitOK
	case len(args) > 1:
		return "", usage("at most one fixture name")
	case strings.HasPrefix(args[0], "-"):
		return "", usage("unknown flag " + args[0])
	}
	return args[0], exitOK
}

func fixturePath(name string) string { return fixtureRoot + "/" + name + "/fixture.json" }

func printFixture(args []string) int {
	name, code := fixtureName(args)
	if code != exitOK {
		return code
	}
	fx, err := loadFixture(fixturePath(name))
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixture error:", err)
		return exitFixture
	}
	return emit(fx.doc)
}

func printRegisters(args []string) int {
	name, code := fixtureName(args)
	if code != exitOK {
		return code
	}
	fx, err := loadFixture(fixturePath(name))
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixture error:", err)
		return exitFixture
	}
	srv, err := fx.server()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixture error:", err)
		return exitFixture
	}
	regs, err := registerMap(srv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailed
	}
	return emit(map[string]any{
		"schemaVersion": schema, "adapter": adapterName, "fixture": fx.Name, "unitId": fx.UnitID,
		"baseAddress": fx.BaseAddress, "registers": regs,
	})
}

func fixtureNames() []string {
	entries, err := os.ReadDir(fixtureRoot)
	if err != nil {
		return []string{}
	}
	names := []string{}
	for _, e := range entries {
		if st, err := os.Stat(fixturePath(e.Name())); err == nil && st.Mode().IsRegular() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// capabilities is the document of docs/CAPABILITIES.md. The client offers
// only what go-sunspec's public API offers: it reads and does not write.
func capabilities() map[string]any {
	return map[string]any{
		"schemaVersion":   schema,
		"adapter":         adapterName,
		"adapterVersion":  adapterVersion,
		"contractVersion": contract,
		"upstream": map[string]any{
			"name":              "go-sunspec",
			"version":           upstreamVersion,
			"revision":          upstreamRevision,
			"artifact":          "go:github.com/otfabric/go-sunspec@" + upstreamVersion,
			"independentEngine": false,
		},
		"roles":            []string{"server", "client"},
		"clientOperations": []string{"scan"},
		"serverFunctions":  []int{3, 6, 16},
		"baseAddresses":    baseAddresses,
		"fixtures":         fixtureNames(),
	}
}

// errUsage marks a usage error of the command line.
var errUsage = errors.New("usage error")

// flags parses "--name value" pairs, all of them known in advance.
func flags(args []string, known ...string) (map[string]string, error) {
	out := map[string]string{}
	for i := 0; i < len(args); i++ {
		name, ok := strings.CutPrefix(args[i], "--")
		if !ok || !contains(known, name) {
			return nil, fmt.Errorf("%w: unknown argument %s", errUsage, args[i])
		}
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("%w: --%s given twice", errUsage, name)
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("%w: --%s needs a value", errUsage, name)
		}
		out[name] = args[i+1]
		i++
	}
	return out, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
