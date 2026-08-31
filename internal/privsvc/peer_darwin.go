// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build darwin

package privsvc

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// peerCredential reads LOCAL_PEERCRED from the connected socket.
//
// Darwin is not a target for this appliance (docs/2026-08-29-design.md section
// 2 lists macOS as a later phase). It is implemented anyway for one reason that
// is not convenience: the authorisation path is the security of the whole
// split, and it is worth being able to exercise the REAL syscall on the machine
// this is developed on rather than only the policy above it.
//
// The struct has no pid field on this platform, which is why Peer.PID is
// documented as advisory and used only in a log line.
func peerCredential(c *net.UnixConn) (Peer, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return Peer{}, fmt.Errorf("privsvc: reading the peer's credentials: %w", err)
	}
	var (
		cred    *unix.Xucred
		credErr error
	)
	ctrlErr := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	})
	if ctrlErr != nil {
		return Peer{}, fmt.Errorf("privsvc: reading the peer's credentials: %w", ctrlErr)
	}
	if credErr != nil {
		return Peer{}, fmt.Errorf("privsvc: reading the peer's credentials: %w", credErr)
	}
	return Peer{UID: cred.Uid}, nil
}
