// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/cB-Sigma/sunspec-sim/sim"
)

// runServer serves a fixture with sunspec-sim's device: functions 3, 6 and
// 16, exception 2 outside the map and on registers that are not writable,
// exception 11 for another unit, exception 1 for other functions.
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
	im, err := fx.image()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixture error:", err)
		return exitFixture
	}
	n := int(im.Next()) - int(im.Base)

	address := net.JoinHostPort(bind, strconv.FormatUint(port, 10))
	dev, err := sim.StartOptions(im, sim.Options{Listen: address, Units: []uint8{fx.UnitID}})
	if err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		return exitFailed
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	if err := os.WriteFile(readyFile, []byte("ready\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "ready file:", err)
		_ = dev.Close()
		return exitFailed
	}
	fmt.Fprintf(os.Stderr, "serving %d registers at %d, unit %d, on %s\n", n, fx.BaseAddress, fx.UnitID, dev.Addr())
	select {
	case <-stop:
	case <-dev.Done():
		fmt.Fprintln(os.Stderr, "server: stopped")
	}
	_ = os.Remove(readyFile)
	if err := dev.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "stop:", err)
	}
	return exitOK
}
