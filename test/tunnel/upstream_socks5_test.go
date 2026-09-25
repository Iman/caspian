// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package tunnel

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// testSOCKS5 is a deliberately small SOCKS5 forwarder used only by the
// integration test below. It records every CONNECT request, then forwards the
// byte stream to the requested address. The test therefore observes the
// actual network hop made by Xray instead of trusting the generated JSON.
type testSOCKS5 struct {
	addr     string
	username string
	password string

	mu       sync.Mutex
	connects []string
	ln       net.Listener
}

func startTestSOCKS5(t *testing.T, username, password string) *testSOCKS5 {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for test SOCKS5 proxy: %v", err)
	}
	s := &testSOCKS5{
		addr:     ln.Addr().String(),
		username: username,
		password: password,
		ln:       ln,
	}

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handle(c)
		}
	}()

	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *testSOCKS5) handle(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(15 * time.Second))

	var head [2]byte
	if _, err := io.ReadFull(c, head[:]); err != nil || head[0] != 5 {
		return
	}
	methods := make([]byte, int(head[1]))
	if _, err := io.ReadFull(c, methods); err != nil {
		return
	}

	method := byte(0xff)
	if s.username == "" && s.password == "" {
		for _, m := range methods {
			if m == 0x00 {
				method = 0x00
				break
			}
		}
	} else {
		for _, m := range methods {
			if m == 0x02 {
				method = 0x02
				break
			}
		}
	}
	if _, err := c.Write([]byte{0x05, method}); err != nil || method == 0xff {
		return
	}

	if method == 0x02 {
		if err := s.authenticate(c); err != nil {
			_, _ = c.Write([]byte{0x01, 0x01})
			return
		}
	}

	var reqHead [4]byte
	if _, err := io.ReadFull(c, reqHead[:]); err != nil {
		return
	}
	if reqHead[0] != 5 || reqHead[1] != 1 || reqHead[2] != 0 {
		_, _ = c.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	host, err := readSOCKS5Host(c, reqHead[3])
	if err != nil {
		return
	}
	var portBytes [2]byte
	if _, err := io.ReadFull(c, portBytes[:]); err != nil {
		return
	}
	port := int(binary.BigEndian.Uint16(portBytes[:]))
	target := net.JoinHostPort(host, strconv.Itoa(port))

	s.mu.Lock()
	s.connects = append(s.connects, target)
	s.mu.Unlock()

	upstream, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		_, _ = c.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer upstream.Close()

	if _, err := c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(upstream, c)
		_ = upstream.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(c, upstream)
		_ = c.Close()
	}()
	wg.Wait()
}

func (s *testSOCKS5) authenticate(c net.Conn) error {
	var head [2]byte
	if _, err := io.ReadFull(c, head[:]); err != nil {
		return err
	}
	if head[0] != 1 {
		return fmt.Errorf("unexpected SOCKS5 auth version %d", head[0])
	}

	user := make([]byte, int(head[1]))
	if _, err := io.ReadFull(c, user); err != nil {
		return err
	}

	var plen [1]byte
	if _, err := io.ReadFull(c, plen[:]); err != nil {
		return err
	}
	pass := make([]byte, int(plen[0]))
	if _, err := io.ReadFull(c, pass); err != nil {
		return err
	}

	if string(user) != s.username || string(pass) != s.password {
		return fmt.Errorf("test SOCKS5 credentials did not match")
	}
	_, err := c.Write([]byte{0x01, 0x00})
	return err
}

func readSOCKS5Host(c net.Conn, atyp byte) (string, error) {
	switch atyp {
	case 0x01:
		var b [4]byte
		if _, err := io.ReadFull(c, b[:]); err != nil {
			return "", err
		}
		return net.IP(b[:]).String(), nil
	case 0x03:
		var l [1]byte
		if _, err := io.ReadFull(c, l[:]); err != nil {
			return "", err
		}
		b := make([]byte, int(l[0]))
		if _, err := io.ReadFull(c, b); err != nil {
			return "", err
		}
		return string(b), nil
	case 0x04:
		var b [16]byte
		if _, err := io.ReadFull(c, b[:]); err != nil {
			return "", err
		}
		return net.IP(b[:]).String(), nil
	default:
		return "", fmt.Errorf("unsupported SOCKS5 address type %d", atyp)
	}
}

func (s *testSOCKS5) targets() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.connects...)
}

func startClientThroughTestSOCKS5(t *testing.T, shareLink string, socksPort int, proxy *testSOCKS5) (*engine.Engine, error) {
	t.Helper()

	l, err := link.Parse(shareLink)
	if err != nil {
		return nil, fmt.Errorf("the share link did not parse: %w", err)
	}

	o := xcfg.Defaults()
	o.Link = l
	o.TUN.Disabled = true
	o.SOCKS.Listen = "127.0.0.1"
	o.SOCKS.Port = uint16(socksPort)
	o.LogLevel = xcfg.LogInfo
	o.Upstream = xcfg.UpstreamSOCKS5{
		Enabled:  true,
		Address:  "127.0.0.1",
		Port:     uint16(proxyPort(proxy)),
		Username: proxy.username,
		Password: proxy.password,
	}

	doc, err := xcfg.Build(o)
	if err != nil {
		return nil, fmt.Errorf("xcfg.Build refused the upstream configuration: %w", err)
	}

	e := engine.New()
	if err := e.Start(context.Background(), doc); err != nil {
		return nil, fmt.Errorf("the engine refused the upstream configuration: %w", err)
	}
	t.Cleanup(func() { _ = e.Stop() })

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c, derr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), time.Second)
		if derr == nil {
			_ = c.Close()
			return e, nil
		}
		time.Sleep(20 * time.Millisecond)
	}

	return nil, fmt.Errorf("the engine reports %s but nothing ever accepted on 127.0.0.1:%d",
		e.State().Phase, socksPort)
}

func proxyPort(proxy *testSOCKS5) int {
	_, portText, _ := net.SplitHostPort(proxy.addr)
	port, _ := strconv.Atoi(portText)
	return port
}

// TestUpstreamSOCKS5CarriesRealVLESSTrafficThroughAnActualProxy proves the
// complete chain:
// client request -> Caspian/Xray -> upstream SOCKS5 -> VLESS test server ->
// origin.
//
// The assertion that matters is not just that the HTTP request succeeds. The
// SOCKS5 server must have observed a CONNECT to the VLESS server's real
// loopback port. Without the upstream hop, that observation cannot happen.
func TestUpstreamSOCKS5CarriesRealVLESSTrafficThroughAnActualProxy(t *testing.T) {
	const (
		noAuthUser = ""
		noAuthPass = ""
		authUser   = "caspian-upstream-user"
		authPass   = "caspian-upstream-pass"
	)

	tests := []struct {
		name string
		user string
		pass string
	}{
		{name: "without authentication", user: noAuthUser, pass: noAuthPass},
		{name: "with authentication", user: authUser, pass: authPass},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := protocolCases()[0]
			token := newToken(t, "upstream-token")
			origin := startEndpoint(t, token)
			cert := makeServerCert(t)
			serverPort := freeLoopbackPort(t)
			clientSocksPort := freeLoopbackPort(t)
			upstream := startTestSOCKS5(t, tc.user, tc.pass)

			startXrayServer(t, serverConfig(p.inbound(serverPort, cert), origin.port))

			_, err := startClientThroughTestSOCKS5(
				t,
				p.shareLink(serverPort, p.secret, cert.pinHex),
				clientSocksPort,
				upstream,
			)
			if err != nil {
				t.Fatalf("client did not come up with upstream SOCKS5 enabled: %v", err)
			}

			path := "/" + token
			body, err := socksGet(
				fmt.Sprintf("127.0.0.1:%d", clientSocksPort),
				originHost,
				origin.port,
				path,
				carryTimeout,
			)
			if err != nil {
				t.Fatalf("request did not traverse the upstream SOCKS5 chain: %v", err)
			}
			if body != token {
				t.Fatalf("origin returned %q, want %q", body, token)
			}

			targets := upstream.targets()
			expected := fmt.Sprintf("127.0.0.1:%d", serverPort)
			if len(targets) == 0 {
				t.Fatal("upstream SOCKS5 proxy observed no CONNECT request")
			}
			found := false
			for _, target := range targets {
				if target == expected {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("upstream SOCKS5 targets=%v, want a CONNECT to %s", targets, expected)
			}

			if got := origin.requests(); len(got) != 1 || got[0].Host != fmt.Sprintf("%s:%d", originHost, origin.port) {
				t.Fatalf("origin saw unexpected request(s): %+v", origin.requests())
			}

			t.Logf("verified upstream SOCKS5 hop to %s; origin response crossed VLESS and matched the run token", expected)
		})
	}
}
