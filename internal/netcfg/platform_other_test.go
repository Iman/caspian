// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux

package netcfg

const (
	isLinux         = false
	hasSystemRunner = false
)
