// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package privsvc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/ipc/namedpipe"
)

// The transport on Windows is a named pipe.
//
// Go supports AF_UNIX sockets on Windows, but they carry no peer credentials:
// no SO_PEERCRED, no LOCAL_PEERCRED, no ancillary data at all. A helper that
// runs with the operating system's full privileges and answers anybody it
// cannot identify is not a privilege boundary, so the unix socket is not
// used here. A named pipe gives two things instead: a security descriptor
// that decides who may open it at all, and the client's identity through
// impersonation once it has.
//
// The pipe package is WireGuard's (MIT), already in this module's graph
// through the engine.

// pipeSecurity is the descriptor the pipe is created with: full access for
// SYSTEM and Administrators, read and write for the panel's account, nothing
// for anyone else. The panel account's SID is resolved at listen time.
func pipeSecurity(serviceAccount string) (*windows.SECURITY_DESCRIPTOR, error) {
	sddl := "D:P(A;;GA;;;SY)(A;;GA;;;BA)"
	if serviceAccount != "" {
		sid, _, _, err := windows.LookupSID("", serviceAccount)
		if err == nil {
			sddl += "(A;;GRGW;;;" + sid.String() + ")"
		}
	}
	return windows.SecurityDescriptorFromString(sddl)
}

func listenEndpoint(cfg ListenConfig) (net.Listener, error) {
	sd, err := pipeSecurity(cfg.ServiceAccount)
	if err != nil {
		return nil, fmt.Errorf("privsvc: building the pipe's security descriptor: %w", err)
	}
	ln, err := (&namedpipe.ListenConfig{SecurityDescriptor: sd}).Listen(cfg.Path)
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_PIPE_BUSY) {
			return nil, fmt.Errorf("%w: %s", ErrAlreadyRunning, cfg.Path)
		}
		return nil, fmt.Errorf("privsvc: could not open the pipe at %s: %w", cfg.Path, err)
	}
	return ln, nil
}

var (
	modadvapi32                    = windows.NewLazySystemDLL("advapi32.dll")
	procImpersonateNamedPipeClient = modadvapi32.NewProc("ImpersonateNamedPipeClient")
)

// peerOf establishes who opened the pipe: impersonate the client on this
// thread, read the thread token's user, revert. The SID string is the Peer's
// identity on Windows; UID is meaningless here and left zero.
func peerOf(conn net.Conn) (Peer, error) {
	h, ok := conn.(interface{ Handle() windows.Handle })
	if !ok {
		return Peer{}, ErrPeerUnknown
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, _, e := procImpersonateNamedPipeClient.Call(uintptr(h.Handle()))
	if r == 0 {
		return Peer{}, fmt.Errorf("privsvc: reading the peer's credentials: %w", e)
	}
	defer windows.RevertToSelf()
	var tok windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &tok); err != nil {
		return Peer{}, fmt.Errorf("privsvc: reading the peer's credentials: %w", err)
	}
	defer tok.Close()
	user, err := tok.GetTokenUser()
	if err != nil {
		return Peer{}, fmt.Errorf("privsvc: reading the peer's credentials: %w", err)
	}
	p := Peer{SID: user.User.Sid.String()}
	// Administrators: the token says so through membership, not through the
	// user SID, so it is recorded on the peer for Permits.
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err == nil {
		if member, err := tok.IsMember(admins); err == nil && member {
			p.Admin = true
		}
	}
	if s := strings.ToUpper(p.SID); s == "S-1-5-18" {
		p.Admin = true // LocalSystem
	}
	_ = unsafe.Pointer(nil)
	return p, nil
}

func dialEndpoint(ctx context.Context, path string, timeout time.Duration) (net.Conn, error) {
	dctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return namedpipe.DialContext(dctx, path)
}

// EndpointPresent reports whether something is listening at the endpoint,
// for "caspian check". It connects and hangs up; nothing is sent.
func EndpointPresent(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := namedpipe.DialContext(ctx, path)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
