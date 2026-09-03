// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import "context"

// darwinBackend is macOS.
//
// This file is untagged on purpose: the backend is a value, chosen by
// Options.Platform, so that its command generation is compiled and tested on
// every development machine. Only exec_darwin.go, the runner that actually
// executes on a Mac, carries a build tag.
//
// The mechanisms, each measured or verified on 2026-09-03 and recorded in the
// port research under local/port-research (not committed):
//
//   - The access point is Apple's Internet Sharing on the built-in radio. No
//     USB Wi-Fi adapter has an AP-capable driver on Apple Silicon, DriverKit
//     has no Wi-Fi family, and hostapd has no Darwin backend. So the plan's
//     hotspot interface is the Wi-Fi hardware port and the uplink must be a
//     different service (Ethernet, USB Ethernet, iPhone USB).
//   - Routes come from route(8), addresses from ifconfig(8), knobs from
//     sysctl(8), and the fail-closed firewall is one pf anchor loaded with
//     "pfctl -a caspian -f -" as a single transaction, the same all-or-nothing
//     property "nft -f -" gives on Linux.
//   - The engine's utun assigns its own address (xray-core tun_darwin.go),
//     so there is no tunnel-address step here.
type darwinBackend struct{}

func init() { registerBackend(darwinBackend{}) }

func (darwinBackend) Platform() Platform { return PlatformDarwin }

// The binaries the macOS runner may execute. Nothing here is Homebrew: every
// one ships with macOS at a fixed path, which is what lets exec_darwin.go
// ignore PATH the way exec_linux.go does.
const (
	BinRoute        = "route"
	BinIfconfig     = "ifconfig"
	BinPfctl        = "pfctl"
	BinNetworksetup = "networksetup"
)

var darwinAllowedBinaries = map[string]bool{
	BinRoute:        true,
	BinIfconfig:     true,
	BinPfctl:        true,
	BinSysctl:       true,
	BinNetworksetup: true,
}

func (darwinBackend) AllowedBinaries() map[string]bool { return darwinAllowedBinaries }

// darwinSysctlKnobs are the knobs a macOS plan changes. Both are global, as
// on Linux, so detection can read them before any interface exists.
func darwinSysctlKnobs() []string {
	return []string{"net.inet.ip.forwarding", "net.inet6.ip6.forwarding"}
}

func (darwinBackend) BaseSysctlKnobs() []string  { return darwinSysctlKnobs() }
func (darwinBackend) SysctlKnobs(*Plan) []string { return darwinSysctlKnobs() }

// The remaining methods are filled in by the darwin port, one measured step at
// a time. Until then each one refuses, so that a macOS build that reaches
// this far reports what is missing rather than applying nothing and reporting
// success.

func (d darwinBackend) Detect(ctx context.Context, r Runner, knobs []string) (Facts, error) {
	return darwinDetect(ctx, r, knobs)
}

func (d darwinBackend) PreEngineSteps(p *Plan, current map[string]string) []Step {
	return p.darwinPreEngineSteps(current)
}

func (d darwinBackend) PostEngineSteps(p *Plan, current map[string]string) []Step {
	return p.darwinPostEngineSteps(current)
}

func (d darwinBackend) CutStep(p *Plan) Step     { return p.darwinCutStep() }
func (d darwinBackend) RestoreStep(p *Plan) Step { return p.darwinRestoreStep() }

func (d darwinBackend) AssertHotspotInterfaceReleased(ctx context.Context, r Runner, p *Plan) error {
	return darwinAssertHotspotInterfaceReleased(ctx, r, p)
}

func (d darwinBackend) AssertHotspotIsAccessPoint(ctx context.Context, r Runner, p *Plan, ssid string) error {
	return darwinAssertHotspotIsAccessPoint(ctx, r, p, ssid)
}

// RegulatoryDomain is not readable from a shipped macOS tool: the airport
// utility that printed it was removed in macOS 14.4, and wdutil needs a
// terminal with Location permission. Internet Sharing picks the domain itself
// from the Wi-Fi hardware, so the country only matters for the rendered
// hostapd text that macOS never uses. The service falls back to its
// configured country; see cmd/caspian.
func (darwinBackend) RegulatoryDomain(context.Context, Runner) (string, bool) { return "", false }
