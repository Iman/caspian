// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package netcfg

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClassifyLeavesSuccessfulWindowsCallSuccessful(t *testing.T) {
	if err := classify("successful call", nil); err != nil {
		t.Fatalf("classify(nil) = %v, want nil", err)
	}
}

func TestWindowsRunnerPreservesOldWindowsRefusal(t *testing.T) {
	w := &windowsRunner{capability: func() WindowsCapability { return WindowsCapabilityFor(18363) }}
	for _, verb := range []string{"set", "clear"} {
		res, err := w.Run(context.Background(), Command{Path: BinIPHelper, Args: []string{"dns", verb, "xray0"}})
		if !errors.Is(err, ErrWindowsTooOld) {
			t.Fatalf("%s: error = %v", verb, err)
		}
		if res.ExitCode != 1 || !strings.Contains(res.Stderr, "18363") {
			t.Fatalf("result = %+v", res)
		}
	}
}

func TestWindowsRunnerChecksTheRunningBuild(t *testing.T) {
	w := NewSystemRunner().(*windowsRunner)
	if got, want := w.CheckSupport(), windowsCapability().CheckSupport(); (got == nil) != (want == nil) {
		t.Fatalf("support = %v, want %v", got, want)
	}
}
