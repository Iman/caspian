// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package tunnel

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
)

func occupyPort(t *testing.T, port int) {
	t.Helper()
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
}

func TestLoopbackStartupRetriesClientPortCollision(t *testing.T) {
	for _, kind := range []string{"share link", "selected entry", "pinned server name"} {
		t.Run(kind, func(t *testing.T) {
			server := startEntryServer(t, "vless")
			attempts, occupiedPort := 0, 0
			port, err := startOnLoopbackPort(t, func(port int) error {
				attempts++
				if attempts == 1 {
					occupiedPort = port
					occupyPort(t, port)
				}
				switch kind {
				case "selected entry":
					_, _, _, err := startClientEntry(t, server.link, 0, port)
					return err
				case "pinned server name":
					raw := strings.Replace(server.link, "127.0.0.1", serverName, 1)
					_, err := startNamedClient(t, raw, port, []netip.Addr{netip.MustParseAddr("127.0.0.1")})
					return err
				default:
					_, err := startClient(t, server.link, port)
					return err
				}
			})
			if err != nil {
				t.Fatalf("client did not recover from the occupied port: %v", err)
			}
			if attempts < 2 || attempts > 5 || port == occupiedPort {
				t.Fatalf("startup made %d attempts, port %d, occupied %d", attempts, port, occupiedPort)
			}
			body, err := socksGet(fmt.Sprintf("127.0.0.1:%d", port), originHost, 80, "/collision", carryTimeout)
			if err != nil || body != server.origin.body {
				t.Fatalf("retried client did not carry the origin's response: body %q, error %v", body, err)
			}
		})
	}
}

func TestLoopbackStartupRetriesServerPortCollision(t *testing.T) {
	p := protocolNamed(t, "vless")
	origin := startEndpoint(t, newToken(t, "collision-origin"))
	cert := makeServerCert(t)
	attempts, occupiedPort := 0, 0
	serverPort := startXrayServer(t, func(port int) string {
		attempts++
		if attempts == 1 {
			occupiedPort = port
			occupyPort(t, port)
		}
		return serverConfig(p.inbound(port, cert), origin.port)
	})
	if attempts < 2 || attempts > 5 || serverPort == occupiedPort {
		t.Fatalf("startup made %d attempts, port %d, occupied %d", attempts, serverPort, occupiedPort)
	}
	port, err := startOnLoopbackPort(t, func(port int) error {
		_, err := startClient(t, p.shareLink(serverPort, p.secret, cert.pinHex), port)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := socksGet(fmt.Sprintf("127.0.0.1:%d", port), originHost, 80, "/collision", carryTimeout)
	if err != nil || body != origin.body {
		t.Fatalf("retried server did not carry the origin's response: body %q, error %v", body, err)
	}
}

func TestLoopbackStartupStopsOnOtherErrors(t *testing.T) {
	for _, message := range []string{"bad configuration", "failed to listen TCP: bind: permission denied"} {
		t.Run(message, func(t *testing.T) {
			want := errors.New(message)
			attempts := 0
			_, err := startOnLoopbackPort(t, func(int) error {
				attempts++
				return want
			})
			if !errors.Is(err, want) || attempts != 1 {
				t.Fatalf("got %v after %d attempts, want original error after one attempt", err, attempts)
			}
		})
	}
}

func TestLoopbackStartupBoundsPortCollisions(t *testing.T) {
	server := startEntryServer(t, "vless")
	attempts := 0
	_, err := startOnLoopbackPort(t, func(port int) error {
		attempts++
		occupyPort(t, port)
		_, _, _, err := startClientEntry(t, server.link, 0, port)
		return err
	})
	if err == nil || attempts != 5 || !strings.Contains(err.Error(), "bind:") {
		t.Fatalf("got %v after %d attempts, want a bind error after five attempts", err, attempts)
	}
}

func TestLoopbackStartupRecognizesWindowsPortCollision(t *testing.T) {
	attempts := 0
	_, err := startOnLoopbackPort(t, func(int) error {
		attempts++
		if attempts == 1 {
			return errors.New("listen tcp 127.0.0.1:1080: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted.")
		}
		return nil
	})
	if err != nil || attempts != 2 {
		t.Fatalf("got %v after %d attempts, want success after two attempts", err, attempts)
	}
}
