// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"caspianbyoc.org/caspian/internal/state"
)

// resetPanelPassword replaces only the panel verifier. It deliberately reads
// from stdin so callers can use a silent terminal read; the password never
// appears in argv, process listings, logs, or the state file.
func resetPanelPassword(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(errOut, "caspian: reset-panel-password takes no options")
		return exitUsage
	}
	if stateDir == layout.StateDir {
		if conn, err := net.DialTimeout("tcp", "127.0.0.1:8088", time.Second); err == nil {
			conn.Close()
			fmt.Fprintln(errOut, "caspian: stop the panel service before resetting its password; use Reset Password in the macOS app")
			return exitError
		}
	}
	reader := bufio.NewScanner(io.LimitReader(in, 8193))
	reader.Buffer(make([]byte, 1024), 4096)
	if !reader.Scan() {
		fmt.Fprintln(errOut, "caspian: could not read the new panel password")
		return exitError
	}
	first := strings.TrimSuffix(reader.Text(), "\r")
	if !reader.Scan() {
		fmt.Fprintln(errOut, "caspian: repeat the password on a second line")
		return exitError
	}
	second := strings.TrimSuffix(reader.Text(), "\r")
	if len([]rune(first)) < 8 || first != second {
		fmt.Fprintln(errOut, "caspian: use at least 8 characters and repeat the same password")
		return exitError
	}
	store, err := state.Load(stateDir)
	if err != nil {
		fmt.Fprintf(errOut, "caspian: cannot load state: %v\n", err)
		return exitError
	}
	if store.FirstRun() {
		fmt.Fprintln(errOut, "caspian: no existing installation; use first-run setup")
		return exitError
	}
	if err := store.SetPanelPassword(first); err != nil {
		fmt.Fprintf(errOut, "caspian: cannot save panel password: %v\n", err)
		return exitError
	}
	fmt.Fprintln(out, "panel password updated")
	return exitOK
}
