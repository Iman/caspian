// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build !linux && !darwin && !windows

package main

import (
	"errors"
	"runtime"

	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
)

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

func newPlatformPrivileged() (platformPrivileged, error) {
	return platformPrivileged{}, errors.New("this build has no network backend or access point driver for " +
		runtime.GOOS + ", so the privileged service cannot run here")
}

func platformPrograms() []checkedProgram { return nil }

func platformProgramSearchPath() []string { return nil }
