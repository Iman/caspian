// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build linux

package privsvc

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// peerCredential reads SO_PEERCRED from the connected socket.
//
// The kernel fills the credentials in at connect time from the process that
// called connect, and they cannot be forged by the caller: this is not a value
// the peer sends, it is one the kernel records. That is what makes it usable as
// an authorisation check where a value in the message would not be.
func peerCredential(c *net.UnixConn) (Peer, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return Peer{}, fmt.Errorf("privsvc: reading the peer's credentials: %w", err)
	}
	var (
		cred    *unix.Ucred
		credErr error
	)
	ctrlErr := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if ctrlErr != nil {
		return Peer{}, fmt.Errorf("privsvc: reading the peer's credentials: %w", ctrlErr)
	}
	if credErr != nil {
		return Peer{}, fmt.Errorf("privsvc: reading the peer's credentials: %w", credErr)
	}
	return Peer{UID: cred.Uid, PID: int(cred.Pid)}, nil
}
