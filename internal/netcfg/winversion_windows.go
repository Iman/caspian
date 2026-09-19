// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import "golang.org/x/sys/windows"

// windowsBuild is the build number of the running Windows, or zero if it
// could not be read.
//
// RtlGetVersion is used rather than GetVersionEx because GetVersionEx lies:
// since Windows 8.1 it reports 6.2 to any program without a compatibility
// manifest naming a later version, so a box on 19045 would read as 9200 and
// be refused. RtlGetVersion is not subject to the shim and returns what the
// kernel actually is.
func windowsBuild() uint32 {
	v := windows.RtlGetVersion()
	if v == nil {
		return 0
	}
	return v.BuildNumber
}

// windowsCapability is what this box can do.
func windowsCapability() WindowsCapability {
	return WindowsCapabilityFor(windowsBuild())
}
