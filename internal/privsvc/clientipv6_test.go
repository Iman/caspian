// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"testing"

	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/state"
)

// Anything but the blocking policy is refused, by name, and never defaulted.
//
// The empty case is the one that matters most and is the least obvious. A
// caller that forgot the field and one that meant "block" are indistinguishable
// if empty is treated as the safe value, so the field would stop being a
// setting and become a comment. It is refused for the same reason DNSMode and
// OnTunnelDown are.
//
// The "tunnel" case is the value somebody will reach for when they want the
// feature. It is refused because this build cannot deliver it: internal/netcfg
// installs no IPv6 address on the hotspot or the tunnel device, no IPv6 route
// into the tunnel table and no IPv6 policy rule, so permitting the traffic
// would produce a hotspot whose IPv6 goes nowhere rather than a working one.
// netcfg.TestIPv6Forward_InstallsNoIPv6AddressingOrRouting is the record of
// which work has to land before this refusal can be lifted.
func TestStartRefusesEveryClientIPv6PolicyButTheBlockingOne(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{"empty, which is a caller that forgot the field", ""},
		{"the value somebody wanting the feature would set", "tunnel"},
		{"a value no build has ever had", "allow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			req := startRequest(t)
			req.Network.ClientIPv6 = tc.value

			err := w.svc.Start(t.Context(), req)
			if err == nil {
				t.Fatalf("Start accepted client IPv6 policy %q", tc.value)
			}
			if got := panel.FaultOf(err); got != panel.FaultIPv6Unsupported {
				t.Errorf("fault = %q, want %q. A generic fault here sends the user to look at "+
					"their config instead of at the setting they changed.", got, panel.FaultIPv6Unsupported)
			}
		})
	}
}

// The supported value starts, and the plan it produces blocks client IPv6.
//
// This is the "off means off" half. The setting exists, it crosses the socket,
// and what comes out of it is netcfg.IPv6Block, which is what the goldens are
// generated from. Nothing about the appliance's behaviour moved.
func TestTheBlockingPolicyStartsAndProducesTheBlockingNetcfgOption(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)
	if req.Network.ClientIPv6 != state.ClientIPv6Block {
		t.Fatalf("the shared test request does not carry state.ClientIPv6Block")
	}
	if err := w.svc.Start(t.Context(), req); err != nil {
		t.Fatalf("Start refused the supported policy: %v", err)
	}

	o, err := w.svc.netOptionsFor(req)
	if err != nil {
		t.Fatalf("netOptionsFor: %v", err)
	}
	if o.IPv6 != netcfg.IPv6Block {
		t.Errorf("netOptionsFor gave IPv6 = %v, want %v", o.IPv6, netcfg.IPv6Block)
	}
}

// A change of client IPv6 policy is a different configuration and has to force
// a restart.
//
// The fingerprint is what decides whether an incoming request is "the same one
// we are already running". internal/netcfg's steps are not idempotent, so a
// policy change that hashed the same would be accepted, skipped, and reported
// as applied while the machine kept the old firewall. That is the shape of the
// measured cut-was-skipped incident recorded above netcfg.RunnerKey.
func TestTheRequestFingerprintCoversTheClientIPv6Policy(t *testing.T) {
	a := startRequest(t)
	b := startRequest(t)
	b.Network.ClientIPv6 = "tunnel"

	if requestFingerprint(a) == requestFingerprint(b) {
		t.Error("two requests differing only in the client IPv6 policy have the same fingerprint, " +
			"so a change of policy would be skipped as 'already running'")
	}
}
