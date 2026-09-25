// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"encoding/json"
	"testing"
)

// TestTheEngineBlocksTheTunnelSubnetThePlanChose holds the start path to
// handing the engine the subnet the network plan gave the tunnel adapter.
//
// xcfg.loopGuardRule can only block the tunnel's own subnet if it is told
// which one it is, and the plan chooses it at run time from a pool. Without
// it, a broadcast to that subnet goes direct, leaves through the tunnel
// adapter and comes back in as a new flow, which on Windows grew caspian.exe
// to 1.9 GB (GitHub issue 7, v0.2.12-rc.2).
func TestTheEngineBlocksTheTunnelSubnetThePlanChose(t *testing.T) {
	w := newWorld(t)
	got := startAndReadDocument(t, w, startRequest(t))

	w.svc.mu.RLock()
	subnet := w.svc.plan.TunSubnet
	w.svc.mu.RUnlock()
	if !subnet.IsValid() {
		t.Fatal("the plan chose no tunnel subnet; this test would pass vacuously")
	}

	var d struct {
		Routing struct {
			Rules []struct {
				RuleTag     string   `json:"ruleTag"`
				IP          []string `json:"ip"`
				OutboundTag string   `json:"outboundTag"`
			} `json:"rules"`
		} `json:"routing"`
	}
	if err := json.Unmarshal(got.raw, &d); err != nil {
		t.Fatalf("decoding the engine document: %v", err)
	}
	for _, r := range d.Routing.Rules {
		if r.RuleTag != "loop-guard-block" {
			continue
		}
		if r.OutboundTag != "block" {
			t.Errorf("the loop guard sends traffic to %q, want block", r.OutboundTag)
		}
		if len(r.IP) == 0 || r.IP[0] != subnet.Masked().String() {
			t.Errorf("the loop guard carries %v; want it to start with the plan's tunnel subnet %s", r.IP, subnet.Masked())
		}
		return
	}
	t.Fatal("the engine document has no loop-guard-block rule")
}
