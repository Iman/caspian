// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build darwin

package main

import (
	"os/exec"
	"strings"

	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
)

// platformPrivileged on macOS: route, ifconfig, pfctl, sysctl and
// networksetup through the runner, and Apple's Internet Sharing as the access
// point. The access point driver arrives with the macOS port; until it does,
// the privileged role refuses to start here rather than starting without one.
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
	country string
}

// darwinTunName is the tunnel device. xray-core's darwin TUN refuses any name
// that is not utunN and creates unit N; the kernel numbers the ones it makes
// for VPN clients from 0 upwards, so a high unit avoids the ones in use.
const darwinTunName = "utun100"

func newPlatformPrivileged() (platformPrivileged, error) {
	sys := hotspot.NewExecSystem()
	return platformPrivileged{
		runner:  netcfg.NewSystemRunner(),
		backend: netcfg.SystemBackend(),
		system:  sys,
		ap:      hotspot.NewInternetSharing(sys, hotspot.DefaultInternetSharingPaths()),
		tunName: darwinTunName,
		country: darwinCountry(),
	}, nil
}

// darwinCountry reads the region the Mac is set to. macOS keeps its Wi-Fi
// regulatory domain to itself, so the system region is the honest fallback
// for the country the hotspot plan needs; the panel's advanced settings can
// override it, and an empty string here leaves that to the person.
func darwinCountry() string {
	out, err := exec.Command("/usr/bin/defaults", "read", "/Library/Preferences/.GlobalPreferences", "Country").Output()
	if err != nil {
		return ""
	}
	cc := strings.TrimSpace(string(out))
	if !netcfg.IsUpperAlpha2(cc) {
		return ""
	}
	return cc
}

func platformPrograms() []checkedProgram {
	return []checkedProgram{
		{netcfg.BinRoute, "the pinned route to the server"},
		{netcfg.BinIfconfig, "interfaces and the addresses on them"},
		{netcfg.BinPfctl, "the fail-closed firewall and the steering of client traffic into the tunnel"},
		{netcfg.BinSysctl, "forwarding"},
		{netcfg.BinNetworksetup, "which interface is the Wi-Fi radio, and whether it is joined to a network"},
		{"InternetSharing", "Apple's access point, DHCP and NAT for joined devices"},
		{"bootpd", "Apple's DHCP server, started by Internet Sharing"},
	}
}

func platformProgramSearchPath() []string {
	return []string{"/sbin", "/usr/sbin", "/bin", "/usr/bin", "/usr/libexec"}
}
