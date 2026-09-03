// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build unix

package main

import (
	"context"
	"os/signal"
	"syscall"
)

// serviceContext returns the context a role runs under, cancelled when the
// service manager or a person asks it to stop. systemd and launchd both send
// SIGTERM; a terminal sends SIGINT. The role name is unused here and is what
// Windows registers with its service manager.
func serviceContext(string) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}
