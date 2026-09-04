// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows && (amd64 || arm64)

package netcfg

const (
	isLinux         = false
	hasSystemRunner = true
)
