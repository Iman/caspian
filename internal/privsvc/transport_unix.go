// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build unix

package privsvc

import (
	"context"
	"fmt"
	"net"
	"time"
)

// The transport on Linux and macOS is a unix socket with the kernel's word on
// who is at the other end. This file is that transport; transport_windows.go
// is the named pipe that takes its place there. The Listener and the Client
// see only net.Listener, net.Conn and Peer.

// listenEndpoint opens the endpoint the service is served on.
func listenEndpoint(cfg ListenConfig) (net.Listener, error) {
	if err := prepareSocketPath(cfg.Path); err != nil {
		return nil, err
	}
	addr := &net.UnixAddr{Name: cfg.Path, Net: "unix"}
	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("privsvc: could not open the socket at %s: %w", cfg.Path, err)
	}
	// SetUnlinkOnClose is on by default for a listener that created the file;
	// it is stated so that a later reader does not have to know the default.
	ln.SetUnlinkOnClose(true)
	if err := secureSocket(cfg.Path, cfg.Group); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// peerOf reads the connected account from the kernel.
func peerOf(conn net.Conn) (Peer, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return Peer{}, ErrPeerUnknown
	}
	return peerCredential(uc)
}

// dialEndpoint connects the panel to the service.
func dialEndpoint(ctx context.Context, path string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	return d.DialContext(ctx, "unix", path)
}

// EndpointPresent reports whether something is listening at the endpoint,
// for "caspian check". It connects and hangs up; nothing is sent.
func EndpointPresent(path string) bool {
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
