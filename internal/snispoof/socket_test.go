// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestOwnedDialRegistersBeforeConnectionAndPreservesHalfClose(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var registered atomic.Bool
	serverErr := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(5 * time.Second))
		if !registered.Load() {
			serverErr <- errors.New("connection arrived before registration")
			return
		}
		b, err := io.ReadAll(c)
		if err == nil {
			_, err = c.Write(b)
		}
		serverErr <- err
	}()
	c, err := dialOwned(context.Background(), netip.MustParseAddr("127.0.0.1"), ln.Addr().(*net.TCPAddr).AddrPort(), func(addr netip.AddrPort) error {
		if !addr.Addr().IsLoopback() || addr.Port() == 0 {
			return errors.New("socket not reserved")
		}
		registered.Store(true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	want := bytes.Repeat([]byte("exact-stream\x00"), 20000)
	if _, err = c.Write(want); err != nil {
		t.Fatal(err)
	}
	if err = c.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(c)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("relay truncated data after half-close")
	}
	if err = <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestOwnedDialFailureAndReadDeadline(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	remote := ln.Addr().(*net.TCPAddr).AddrPort()
	if c, err := dialOwned(context.Background(), netip.MustParseAddr("127.0.0.1"), remote, func(netip.AddrPort) error { return ErrConfirmation }); err == nil {
		c.Close()
		t.Fatal("registration failure ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if c, err := dialOwned(ctx, netip.MustParseAddr("127.0.0.1"), remote, func(netip.AddrPort) error { return nil }); err == nil {
		c.Close()
		t.Fatal("cancellation ignored")
	}
	c, err := dialOwned(context.Background(), netip.MustParseAddr("127.0.0.1"), remote, func(netip.AddrPort) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	if _, err = c.Read(make([]byte, 1)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("deadline: %v", err)
	}
	c.SetReadDeadline(time.Time{})
	done := make(chan error, 1)
	go func() { _, err := c.Read(make([]byte, 1)); done <- err }()
	c.Close()
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("close did not interrupt read")
		}
	case <-time.After(time.Second):
		t.Fatal("read hung after close")
	}
}
