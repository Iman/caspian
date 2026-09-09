// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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
		t.Skip("requires the signed WinDivert package and administrator access")
	}
	observedName := make(chan string, 1)
	want := bytes.Repeat([]byte("verified TLS payload\x00"), 4096)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedName <- r.TLS.ServerName
		w.Write(want)
	}))
	defer server.Close()
	f, err := Start(server.Listener.Addr().(*net.TCPAddr).AddrPort(), "", "cover.example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	for _, tc := range []struct {
		name  string
		valid bool
	}{
		{"example.com", true},
		{"wrong.example.invalid", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: tc.name, MinVersion: tls.VersionTLS12},
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "tcp4", f.Addr().String())
				},
			}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
			response, err := client.Get(server.URL)
			if !tc.valid {
				if err == nil {
					response.Body.Close()
					t.Fatal("spoofing bypassed real certificate name verification")
				}
				var nameError x509.HostnameError
				if !errors.As(err, &nameError) {
					t.Fatalf("expected certificate name rejection, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			got, err := io.ReadAll(response.Body)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("TLS payload changed: bytes=%d error=%v", len(got), err)
			}
			if got := <-observedName; got != tc.name {
				t.Fatalf("real TLS name changed: got %q, want %q", got, tc.name)
			}
		})
	}
}
