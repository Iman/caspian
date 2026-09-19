// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import (
	"bytes"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

// This opt-in test opens WinDivert with a loopback endpoint filter only.
func TestNativeWindowsSpoofingPreservesTheRealStream(t *testing.T) {
	if os.Getenv("CASPIAN_SNI_NATIVE_TEST") != "1" {
		t.Skip("requires the signed WinDivert package and administrator access")
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	serverResult := make(chan []byte, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			serverResult <- nil
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(8 * time.Second))
		b, _ := io.ReadAll(c)
		c.Write(b)
		serverResult <- b
	}()
	f, err := Start(ln.Addr().(*net.TCPAddr).AddrPort(), "", "cover.example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := net.Dial("tcp4", f.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(8 * time.Second))
	want := bytes.Repeat([]byte("real-stream-only\x00"), 100)
	c.Write(want)
	c.(*net.TCPConn).CloseWrite()
	got, err := io.ReadAll(c)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("real stream failed: got %d of %d bytes", len(got), len(want))
	}
	if got = <-serverResult; !bytes.Equal(got, want) {
		t.Fatal("fake data reached the application stream")
	}
}

func TestNativeWindowsSpoofingPreservesTLSIdentity(t *testing.T) {
	if os.Getenv("CASPIAN_SNI_NATIVE_TEST") != "1" {
		t.Skip("requires WinDivert and administrator access")
	}
	testTLSCombinations(t, "cover.example.invalid")
}
