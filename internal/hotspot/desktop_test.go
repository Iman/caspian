// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package hotspot

import (
	"context"
	"net/netip"
	"testing"
	"time"
)

// Shared by the desktop access point drivers' tests: a plan shaped like the
// ones internal/privsvc renders, and a System whose Sleep fails.

func macPlan(t *testing.T) Plan {
	t.Helper()
	ap := APConfig{Interface: "en0", Uplink: "en7", SSID: "Caspian-Wifi", Passphrase: "example-password",
		CountryCode: "GB", Channel: 6, Band: Band2GHz, ControlDir: "/run/hostapd"}
	dns := DNSConfig{Interface: "en0", Subnet: netip.MustParsePrefix("10.83.51.0/24"),
		Gateway: netip.MustParseAddr("10.83.51.1"), RangeStart: netip.MustParseAddr("10.83.51.50"),
		RangeEnd: netip.MustParseAddr("10.83.51.200"), LeaseTime: 12 * time.Hour,
		LeaseFile: "/var/lib/caspian/dnsmasq.leases",
		Upstream:  netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 5354), CacheSize: 150}
	rc := RadioConstraint{SupportsAP: true, MaxAPs: 1, AllowedChannels: []int{1, 6, 11}}
	p, err := NewPlan(ap, dns, rc)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return p
}

// sleepless is a System whose Sleep fails, for the waits that must surface it.
type sleepless struct {
	*Recorder
	err error
}

func (s sleepless) Sleep(context.Context, time.Duration) error { return s.err }
