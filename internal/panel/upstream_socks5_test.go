// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import "testing"

func TestValidateUpstreamSOCKS5(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		address string
		port    uint16
		user    string
		pass    string
		bad     bool
	}{
		{"disabled", false, "", 0, "", "", false},
		{"noauth", true, "127.0.0.1", 1080, "", "", false},
		{"auth", true, "proxy.example", 443, "u", "p", false},
		{"missing address", true, "", 1080, "", "", true},
		{"missing port", true, "127.0.0.1", 0, "", "", true},
		{"whitespace", true, " 127.0.0.1", 1080, "", "", true},
		{"username only", true, "127.0.0.1", 1080, "u", "", true},
		{"password only", true, "127.0.0.1", 1080, "", "p", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateUpstreamSOCKS5(tc.enabled, tc.address, tc.port, tc.user, tc.pass)
			if got.Empty() == tc.bad {
				t.Fatalf("unexpected validation result bad=%t problem=%+v", tc.bad, got)
			}
		})
	}
}
