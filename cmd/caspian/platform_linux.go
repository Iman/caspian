// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build linux

package main

import (
	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
)

// platformPrivileged is the Linux appliance: iproute2, iw, nftables and sysctl
// through the runner, hostapd and dnsmasq under the Supervisor.
type platformPrivileged struct {
	runner  netcfg.Runner
	backend netcfg.Backend
	system  hotspot.System
	ap      hotspot.AccessPoint

	// tunName overrides the tunnel device name where the platform demands one
	// (macOS insists on utunN); empty means netcfg's default.
	tunName string
	// country is the regulatory domain to fall back on when neither the
	// request nor the platform reports one.
	country string // nil means the Supervisor over system and hotspotPaths
}

func newPlatformPrivileged() (platformPrivileged, error) {
	sys, err := hotspot.NewSystemRunner()
	if err != nil {
		return platformPrivileged{}, err
	}
	return platformPrivileged{
		runner:  netcfg.NewSystemRunner(),
		backend: netcfg.SystemBackend(),
		system:  sys,
	}, nil
}

// platformPrograms are the programs "caspian check" looks for on this
// platform, with what each is needed for.
func platformPrograms() []checkedProgram {
	return []checkedProgram{
		{netcfg.BinIP, "addresses, routes and rules"},
		{netcfg.BinIw, "which interfaces are wireless, and what the radio can do"},
		{netcfg.BinNft, "the fail-closed firewall"},
		{netcfg.BinSysctl, "forwarding and reverse-path filtering"},
		{"hostapd", "the access point"},
		{"dnsmasq", "DHCP and DNS for joined devices"},
		{"hostapd_cli", "asking hostapd whether the network is actually being broadcast"},
		{"rfkill", "switching the radio back on when it is soft blocked"},
		{"pgrep", "finding a leftover hostapd or dnsmasq from a previous run"},
	}
}

func platformProgramSearchPath() []string {
	return []string{"/sbin", "/usr/sbin", "/bin", "/usr/bin", "/usr/local/sbin", "/usr/local/bin"}
}
