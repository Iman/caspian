// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package netcfg

import "testing"

func TestClassifyLeavesSuccessfulWindowsCallSuccessful(t *testing.T) {
	if err := classify("successful call", nil); err != nil {
		t.Fatalf("classify(nil) = %v, want nil", err)
	}
}
