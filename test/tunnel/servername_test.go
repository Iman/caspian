// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package tunnel

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// serverName is the name the share links in this file give the proxy server.
//
// It is under ".invalid" for the reason originHost is: RFC 6761 section 6.4
// reserves it so that no resolver answers it. That is what makes this file a
// model of GitHub issue 7 rather than a test of this machine's DNS. On Windows
// the whole host is routed into the tunnel and its DNS is pointed there
// (internal/netcfg/winnet.go, windowsPostEngineSteps), so once the engine is
// up the system resolver cannot answer the server's name without the tunnel
// that needs the answer. A name no resolver answers is the same situation
// reproduced on loopback, without a Windows machine and without root.
const serverName = "proxy.caspian.invalid"

// nameCase is one protocol from protocolCases whose share link names the server
// by serverName instead of by 127.0.0.1.
type nameCase struct {
	protocolCase
}

// nameCases picks the rows whose link spells the server as "@127.0.0.1:" and
// can therefore be renamed by substitution. vmess carries its address inside a
// base64 JSON blob and is left out rather than re-encoded here; it reaches the
// same dialer as every other row, so it adds no path this list does not have.
//
// The list is required to include one TCP row with no stream settings, one
// with a transport that writes the server name into an HTTP header, one with
// TLS, and one over UDP. Those are the four ways the destination reaches
// internet.DialSystem differently, and TestNameCasesCoverEveryDialShape holds
// the list to it.
func nameCases() []nameCase {
	var out []nameCase
	for _, p := range protocolCases() {
		if !strings.Contains(p.shareLink(1, p.secret, "00"), "@127.0.0.1:") {
			continue
		}
		out = append(out, nameCase{p})
	}
	return out
}

func (c nameCase) namedLink(port int, secret, pin string) string {
	return strings.Replace(c.shareLink(port, secret, pin), "@127.0.0.1:", "@"+serverName+":", 1)
}

// TestNameCasesCoverEveryDialShape keeps the table above honest.
func TestNameCasesCoverEveryDialShape(t *testing.T) {
	want := map[string]bool{
		"vless":                false, // raw TCP, no stream settings
		"vless over websocket": false, // Host header derived from the name
		"trojan":               false, // TLS, server name filled from the address
		"hysteria2":            false, // UDP
	}
	for _, c := range nameCases() {
		if _, ok := want[c.name]; ok {
			want[c.name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("the %q row is not in nameCases, so the dial shape it stands for is not exercised", name)
		}
	}
}

// startNamedClient is startClient with the one extra input this file is about:
// the addresses the privileged side resolved and pinned before the engine
// started. Everything else is the product path, unchanged.
func startNamedClient(t *testing.T, shareLink string, socksPort int, pinned []netip.Addr) (*engine.Engine, error) {
	t.Helper()
	l, err := link.Parse(shareLink)
	if err != nil {
		return nil, fmt.Errorf("the share link did not parse: %w", err)
	}
	if l.Address != serverName {
		return nil, fmt.Errorf("the link names its server %q, not %q; the substitution did not apply", l.Address, serverName)
	}
	o := xcfg.Defaults()
	o.Link = l
	o.TUN.Disabled = true
	o.SOCKS.Listen = "127.0.0.1"
	o.SOCKS.Port = uint16(socksPort)
	o.LogLevel = xcfg.LogInfo
	o.PinnedServer = pinned

	doc, err := xcfg.Build(o)
	if err != nil {
		return nil, fmt.Errorf("xcfg.Build refused the parsed link: %w", err)
	}
	e := engine.New()
	if err := e.Start(context.Background(), doc); err != nil {
		return nil, fmt.Errorf("the engine refused to start with the composed document: %w", err)
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
	return nil, fmt.Errorf("nothing ever accepted on 127.0.0.1:%d", socksPort)
}

// namedCarry runs one request through a client whose link names its server by
// a name the system resolver cannot answer.
func namedCarry(t *testing.T, c nameCase, pinned []netip.Addr, timeout time.Duration) (*engine.Engine, error) {
	t.Helper()
	token := newToken(t, "named-token")
	origin := startEndpoint(t, token)
	sentinel := startEndpoint(t, newToken(t, "bypass-sentinel"))
	cert := makeServerCert(t)
	serverPort := freeLoopbackPort(t)
	socksPort := freeLoopbackPort(t)
	path := "/" + token

	startXrayServer(t, serverConfig(c.inbound(serverPort, cert), origin.port))
	e, err := startNamedClient(t, c.namedLink(serverPort, c.secret, cert.pinHex), socksPort, pinned)
	if err != nil {
		t.Fatalf("setting up the client: %v", err)
	}
	body, err := socksGet(fmt.Sprintf("127.0.0.1:%d", socksPort), originHost, sentinel.port, path, timeout)
	if err != nil {
		return e, err
	}
	if body != token {
		return e, fmt.Errorf("the tunnel returned %q, which is not this run's origin token", body)
	}
	return e, checkOriginSawTheTunnelledRequest(origin, fmt.Sprintf("%s:%d", originHost, sentinel.port), path)
}

// TestTheServerNameIsNeverLookedUpOnceTheAddressesArePinned is the regression
// guard for GitHub issue 7.
//
// The client is given the addresses the privileged side resolved before the
// tunnel existed, and the system resolver is unable to answer the server's
// name, which is the Windows state after start. Traffic must still be carried,
// which it can only be if the engine never asks the system resolver for the
// server's name.
func TestTheServerNameIsNeverLookedUpOnceTheAddressesArePinned(t *testing.T) {
	if addrs, err := net.DefaultResolver.LookupNetIP(context.Background(), "ip", serverName); err == nil {
		t.Skipf("%s resolved on this machine to %d address(es), so the resolver cannot be shown to be unused here",
			serverName, len(addrs))
	}
	pinned := []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	for _, c := range nameCases() {
		t.Run(c.name, func(t *testing.T) {
			if _, err := namedCarry(t, c, pinned, carryTimeout); err != nil {
				t.Fatalf("a %s link naming its server by a name the system cannot resolve carried nothing "+
					"although the resolved address was pinned: %v", c.name, err)
			}
		})
	}
}

// TestWithoutPinnedAddressesTheEngineAsksTheSystemResolver is the reproduction
// of the mechanism behind issue 7, kept as a test so the premise of the fix is
// measured on every run rather than asserted once.
//
// With no pinned addresses the document is the one this appliance produced
// before the addresses were fed in: the outbound names the server by its domain
// and nothing tells the engine anything else. The engine then resolves the name at every dial through
// the system resolver (xray-core transport/internet/system_dialer.go, the
// net.Dialer.DialContext at the end of DefaultSystemDialer.Dial, reached from
// internet.DialSystem because no sockopt domainStrategy is set). With a
// resolver that cannot answer, no request is carried.
//
// If this test ever starts passing traffic, the engine has found another way to
// resolve the server name and the reasoning in internal/xcfg/servername.go has
// to be re-checked against it.
func TestWithoutPinnedAddressesTheEngineAsksTheSystemResolver(t *testing.T) {
	if addrs, err := net.DefaultResolver.LookupNetIP(context.Background(), "ip", serverName); err == nil {
		t.Skipf("%s resolved on this machine to %d address(es), so this reproduction cannot hold here",
			serverName, len(addrs))
	}
	for _, c := range nameCases() {
		t.Run(c.name, func(t *testing.T) {
			e, err := namedCarry(t, c, nil, blockedTimeout)
			if err == nil {
				t.Fatalf("a %s link naming its server by an unresolvable name carried traffic with no pinned "+
					"address, so the engine did not depend on the system resolver", c.name)
			}
			// The failure has to be the lookup and not something else that
			// happens to stop traffic, or this test reproduces nothing. The
			// text is Go's own resolver error, which the engine wraps and
			// logs at info level: "lookup <name>: no such host".
			//
			// Polled rather than read once. The vless outbound retries its
			// dial through common/retry and logs only when every attempt has
			// failed, which was measured to land after blockedTimeout had
			// already ended the request on one run in nine.
			want := "lookup " + serverName
			deadline := time.Now().Add(carryTimeout)
			for time.Now().Before(deadline) {
				for _, l := range e.Logs() {
					if strings.Contains(l.Text, want) {
						return
					}
				}
				time.Sleep(50 * time.Millisecond)
			}
			t.Fatalf("no traffic was carried, but no engine log line contains %q, so the failure was not the "+
				"system resolver being asked for the server name", want)
		})
	}
}
