// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"

	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
)

// platformPrivileged on Windows: the IP Helper API, the Windows Filtering
// Platform and Wintun through the in-process runner, and Mobile Hotspot
// through the tethering helper that sits beside this binary.
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

// tetheringHelperName is the C# helper, tools/caspian-tethering, published
// beside caspian.exe. It is looked for next to this executable and nowhere
// else: the program directory is admin-only writable, and a helper found on
// PATH would be a helper somebody else chose.
const tetheringHelperName = "caspian-tethering.exe"

// GetUserDefaultLocaleName is not in golang.org/x/sys/windows; kernel32 has it.
var procGetUserDefaultLocaleName = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")

func newPlatformPrivileged() (platformPrivileged, error) {
	exe, err := os.Executable()
	if err != nil {
		return platformPrivileged{}, err
	}
	helper := filepath.Join(filepath.Dir(exe), tetheringHelperName)
	sys := hotspot.NewExecSystem()
	return platformPrivileged{
		runner:  netcfg.NewSystemRunner(),
		backend: netcfg.SystemBackend(),
		system:  sys,
		ap:      hotspot.NewMobileHotspot(sys, hotspot.MobileHotspotPaths{Helper: helper}),
		country: windowsCountry(),
	}, nil
}

// windowsCountry reads the region from the user's default locale name, for
// example "en-GB" gives "GB". Windows keeps the Wi-Fi regulatory domain in
// the driver, so the region is the honest fallback for the country the plan
// needs; the panel's advanced settings can override it.
func windowsCountry() string {
	buf := make([]uint16, 85)
	n, _, err := procGetUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		_ = err
		return ""
	}
	name := windows.UTF16ToString(buf[:n])
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return ""
	}
	cc := strings.ToUpper(parts[len(parts)-1])
	if !netcfg.IsUpperAlpha2(cc) {
		return ""
	}
	return cc
}

func platformPrograms() []checkedProgram {
	return []checkedProgram{
		{tetheringHelperName, "Mobile Hotspot, through the WinRT tethering API this helper calls"},
		{"wintun.dll", "the tunnel adapter driver, loaded by the engine from beside the binary"},
	}
}

// platformProgramSearchPath is the program directory only: both files above
// are looked for beside the binary and nowhere else.
func platformProgramSearchPath() []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	return []string{filepath.Dir(exe)}
}
