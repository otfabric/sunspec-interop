// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/otfabric/go-modbus"
	sunspec "github.com/otfabric/go-sunspec"
)

// device is the go-sunspec Server as the contract wants it served. go-sunspec
// answers function 4 with exception 2 (the other register type); the
// contract's device keeps its map in holding registers and answers it with
// exception 1, as pysunspec2's does. Everything else is go-sunspec's.
type device struct {
	*sunspec.Server
}

func (device) HandleInputRegisters(context.Context, *modbus.InputRegistersRequest) ([]uint16, error) {
	return nil, modbus.ErrIllegalFunction
}

func runServer(args []string) int {
	f, err := flags(args, "fixture", "bind-address", "port", "ready-file")
	if err != nil {
		return usage(err.Error())
	}
	path, bind, readyFile := fixturePath("baseline"), "0.0.0.0", "/run/sunspec-interop/ready"
	if v, ok := f["fixture"]; ok {
		path = v
	}
	if v, ok := f["bind-address"]; ok {
		bind = v
	}
	if v, ok := f["ready-file"]; ok {
		readyFile = v
	}
	port := uint64(1502)
	if v, ok := f["port"]; ok {
		if port, err = number(v, 16); err != nil {
			return usage("--port: " + v)
		}
	}

	fx, err := loadFixture(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixture error:", err)
		return exitFixture
	}
	srv, err := fx.server()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixture error:", err)
		return exitFixture
	}

	address := net.JoinHostPort(bind, strconv.FormatUint(port, 10))
	ms, err := modbus.NewServer(&modbus.ServerConfig{URL: "tcp://" + address, MaxClients: 64, Logger: modbus.NopLogger()}, device{srv})
	if err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		return exitFailed
	}
	if err := ms.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		return exitFailed
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	if err := os.WriteFile(readyFile, []byte("ready\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "ready file:", err)
		_ = ms.Stop()
		return exitFailed
	}
	n := 4
	for _, h := range srv.Models() {
		n += 2 + int(h.Length)
	}
	fmt.Fprintf(os.Stderr, "serving %d registers at %d, unit %d, on %s\n", n, fx.BaseAddress, fx.UnitID, address)
	<-stop
	_ = os.Remove(readyFile)
	if err := ms.Stop(); err != nil && !errors.Is(err, net.ErrClosed) {
		fmt.Fprintln(os.Stderr, "stop:", err)
	}
	return exitOK
}
