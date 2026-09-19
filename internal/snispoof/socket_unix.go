// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//go:build !windows

package snispoof

import (
	"context"
	"net"
	"net/netip"
	"syscall"
	"time"
)

func reserveSocket(c syscall.RawConn, local netip.Addr) (netip.AddrPort, error) {
	var addr netip.AddrPort
	var bindErr error
	err := c.Control(func(fd uintptr) {
		bindErr = syscall.Bind(int(fd), &syscall.SockaddrInet4{Addr: local.As4()})
		if bindErr != nil {
			return
		}
		sa, e := syscall.Getsockname(int(fd))
		bindErr = e
		if a, ok := sa.(*syscall.SockaddrInet4); ok {
			addr = netip.AddrPortFrom(netip.AddrFrom4(a.Addr), uint16(a.Port))
		} else if e == nil {
			bindErr = ErrUnavailable
		}
	})
	if err != nil {
		return addr, err
	}
	return addr, bindErr
}

func dialOwned(ctx context.Context, local netip.Addr, remote netip.AddrPort, register func(netip.AddrPort) error) (net.Conn, error) {
	d := net.Dialer{Timeout: 5 * time.Second, Control: func(_, _ string, c syscall.RawConn) error {
		addr, err := reserveSocket(c, local)
		if err != nil {
			return err
		}
		return register(addr)
	}}
	return d.DialContext(ctx, "tcp4", remote.String())
}
