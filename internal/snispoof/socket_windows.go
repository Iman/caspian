// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var wsaPoll = windows.NewLazySystemDLL("ws2_32.dll").NewProc("WSAPoll")
var winsockOnce sync.Once
var winsockErr error

// A nonblocking Winsock connection allows binding and registration before
// connect. Go's Windows Dialer otherwise binds again after its Control hook.
// Polling is bounded so Close and changed deadlines promptly interrupt I/O.
type ownedConn struct {
	mu                          sync.RWMutex
	fd                          windows.Handle
	closed                      bool
	readMu, writeMu, deadlineMu sync.Mutex
	readDeadline, writeDeadline time.Time
	local, remote               netip.AddrPort
}

func dialOwned(ctx context.Context, local netip.Addr, remote netip.AddrPort, register func(netip.AddrPort) error) (net.Conn, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	winsockOnce.Do(func() { var data windows.WSAData; winsockErr = windows.WSAStartup(0x202, &data) })
	if winsockErr != nil {
		return nil, ErrUnavailable
	}
	fd, err := windows.WSASocket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_NO_HANDLE_INHERIT)
	if err != nil {
		return nil, err
	}
	c := &ownedConn{fd: fd, remote: remote}
	ok := false
	defer func() {
		if !ok {
			c.Close()
		}
	}()
	if err = windows.Bind(fd, &windows.SockaddrInet4{Addr: local.As4()}); err != nil {
		return nil, err
	}
	sa, err := windows.Getsockname(fd)
	if err != nil {
		return nil, err
	}
	bound, okAddr := sa.(*windows.SockaddrInet4)
	if !okAddr {
		return nil, ErrUnavailable
	}
	c.local = netip.AddrPortFrom(netip.AddrFrom4(bound.Addr), uint16(bound.Port))
	if err = register(c.local); err != nil {
		return nil, err
	}
	var nonblocking uint32 = 1
	var returned uint32
	if err = windows.WSAIoctl(fd, 0x8004667e, (*byte)(unsafe.Pointer(&nonblocking)), 4, nil, 0, &returned, nil, 0); err != nil {
		return nil, err
	}
	if err = windows.SetsockoptInt(fd, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1); err != nil {
		return nil, err
	}
	err = windows.Connect(fd, &windows.SockaddrInet4{Addr: remote.Addr().As4(), Port: int(remote.Port())})
	if err != nil && !errors.Is(err, windows.WSAEWOULDBLOCK) {
		return nil, err
	}
	if err != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		for {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			ready, err := c.poll(0x10)
			if err != nil {
				return nil, err
			}
			if !ready {
				continue
			}
			code, err := windows.GetsockoptInt(fd, windows.SOL_SOCKET, 0x1007) // SO_ERROR
			if err != nil {
				return nil, err
			}
			if code != 0 {
				return nil, ErrUnavailable
			}
			break
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	ok = true
	return c, nil
}

// Caller either owns the socket during connect or holds mu for established I/O.
func (c *ownedConn) poll(events int16) (bool, error) {
	p := struct {
		fd              windows.Handle
		events, revents int16
	}{fd: c.fd, events: events}
	r, _, _ := wsaPoll.Call(uintptr(unsafe.Pointer(&p)), 1, 20)
	if int32(r) < 0 {
		return false, ErrUnavailable
	}
	if p.revents&4 != 0 {
		return false, net.ErrClosed
	}
	return r > 0, nil
}

func (c *ownedConn) expired(read bool) bool {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	d := c.writeDeadline
	if read {
		d = c.readDeadline
	}
	return !d.IsZero() && !time.Now().Before(d)
}

func (c *ownedConn) Read(b []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	if len(b) == 0 {
		return 0, nil
	}
	for {
		if c.expired(true) {
			return 0, os.ErrDeadlineExceeded
		}
		c.mu.RLock()
		if c.closed {
			c.mu.RUnlock()
			return 0, net.ErrClosed
		}
		buf := windows.WSABuf{Len: uint32(len(b)), Buf: &b[0]}
		var n, flags uint32
		err := windows.WSARecv(c.fd, &buf, 1, &n, &flags, nil, nil)
		if errors.Is(err, windows.WSAEWOULDBLOCK) {
			_, err = c.poll(0x100)
			c.mu.RUnlock()
			if err != nil {
				return 0, err
			}
			continue
		}
		c.mu.RUnlock()
		if err != nil {
			return int(n), err
		}
		if n == 0 {
			return 0, io.EOF
		}
		return int(n), nil
	}
}

func (c *ownedConn) Write(b []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	written := 0
	for written < len(b) {
		if c.expired(false) {
			return written, os.ErrDeadlineExceeded
		}
		c.mu.RLock()
		if c.closed {
			c.mu.RUnlock()
			return written, net.ErrClosed
		}
		buf := windows.WSABuf{Len: uint32(len(b) - written), Buf: &b[written]}
		var n uint32
		err := windows.WSASend(c.fd, &buf, 1, &n, 0, nil, nil)
		if errors.Is(err, windows.WSAEWOULDBLOCK) {
			_, err = c.poll(0x10)
			c.mu.RUnlock()
			if err != nil {
				return written, err
			}
			continue
		}
		c.mu.RUnlock()
		written += int(n)
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrShortWrite
		}
	}
	return written, nil
}

func (c *ownedConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return windows.Closesocket(c.fd)
}
func (c *ownedConn) CloseWrite() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return net.ErrClosed
	}
	return windows.Shutdown(c.fd, windows.SHUT_WR)
}
func (c *ownedConn) LocalAddr() net.Addr  { return net.TCPAddrFromAddrPort(c.local) }
func (c *ownedConn) RemoteAddr() net.Addr { return net.TCPAddrFromAddrPort(c.remote) }
func (c *ownedConn) SetDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	c.readDeadline = t
	c.writeDeadline = t
	return nil
}
func (c *ownedConn) SetReadDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	c.readDeadline = t
	return nil
}
func (c *ownedConn) SetWriteDeadline(t time.Time) error {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	c.writeDeadline = t
	return nil
}
