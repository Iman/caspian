// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux && !(windows && (amd64 || arm64))

package netcfg

const (
	isLinux         = false
	hasSystemRunner = false
)
