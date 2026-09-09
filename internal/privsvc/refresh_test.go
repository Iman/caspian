// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/panel"
)

// The tests in this file were written BEFORE Service.Refresh, the refresh
// wire shapes and Config.RefreshRootCAs existed, on 2026-09-09. Their first
// run was a compile failure naming them; that is the "red" in the report.
//
// What they hold, in the words of the design (proposal-subscription-fetch.md,
// "Routing the request so never direct is enforced, not assumed"): the
// request is refused before any socket opens unless the engine is running;
// the host name is handed to the engine's SOCKS inbound UNRESOLVED and the
// host resolver is never asked; plain http is refused at the address and at
// every redirect; the body is capped; and no error carries the address or
// the name inside it.

// fakeSubscriptionHost is inside the name set of the certificate
// net/http/httptest serves (127.0.0.1, ::1, example.com, *.example.com), so
// the client's ordinary certificate check passes against the pool the test
// supplies. It is a documentation name and resolves to nothing, which is the
// point: if anything resolved it, the test would have to fail.
const fakeSubscriptionHost = "sub.example.com"

// fakeSubscriptionURL carries a token in the query, which is the shape
// providers use and the reason the address is a credential.
const fakeSubscriptionURL = "https://" + fakeSubscriptionHost + "/api/sub?token=fake-refresh-token-not-real"

// socksRelay is a minimal SOCKS5 server standing in for the engine's
// loopback inbound. It records what the client asked to reach, in the shape
// the client asked for it, and relays every connection to one fixed target,
// because the name it is given resolves to nothing.
type socksRelay struct {
	ln     net.Listener
	target string

	mu       sync.Mutex
	connects int
	domains  []string
	literals []string
}

func newSocksRelay(t *testing.T, target string) *socksRelay {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	r := &socksRelay{ln: ln, target: target}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go r.serve(c)
		}
	}()
	return r
}

func (r *socksRelay) port() uint16 { return uint16(r.ln.Addr().(*net.TCPAddr).Port) }

func (r *socksRelay) serve(c net.Conn) {
	defer c.Close()
	r.mu.Lock()
	r.connects++
	r.mu.Unlock()

	// Greeting: version, method count, methods. Answer: no authentication.
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(c, hdr); err != nil || hdr[0] != 5 {
		return
	}
	if _, err := io.ReadFull(c, make([]byte, int(hdr[1]))); err != nil {
		return
	}
	if _, err := c.Write([]byte{5, 0}); err != nil {
		return
	}

	// Request: version, command, reserved, address type, address, port.
	req := make([]byte, 4)
	if _, err := io.ReadFull(c, req); err != nil || req[0] != 5 || req[1] != 1 {
		return
	}
	var dest string
	switch req[3] {
	case 1:
		b := make([]byte, 4)
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		dest = net.IP(b).String()
		r.mu.Lock()
		r.literals = append(r.literals, dest)
		r.mu.Unlock()
	case 3:
		n := make([]byte, 1)
		if _, err := io.ReadFull(c, n); err != nil {
			return
		}
		b := make([]byte, int(n[0]))
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		dest = string(b)
		r.mu.Lock()
		r.domains = append(r.domains, dest)
		r.mu.Unlock()
	case 4:
		b := make([]byte, 16)
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		dest = net.IP(b).String()
		r.mu.Lock()
		r.literals = append(r.literals, dest)
		r.mu.Unlock()
	default:
		return
	}
	port := make([]byte, 2)
	if _, err := io.ReadFull(c, port); err != nil {
		return
	}

	up, err := net.Dial("tcp", r.target)
	if err != nil {
		_, _ = c.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer up.Close()
	if _, err := c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(up, c); done <- struct{}{} }()
	go func() { _, _ = io.Copy(c, up); done <- struct{}{} }()
	<-done
}

func (r *socksRelay) seen() (connects int, domains, literals []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.connects, append([]string(nil), r.domains...), append([]string(nil), r.literals...)
}

// providerFixture is a TLS provider behind a SOCKS relay, with the world's
// service pointed at the relay and trusting the provider's certificate.
type providerFixture struct {
	w      *world
	relay  *socksRelay
	server *httptest.Server

	mu       sync.Mutex
	requests []*http.Request
}

func newProvider(t *testing.T, handler http.HandlerFunc) *providerFixture {
	t.Helper()
	f := &providerFixture{}
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.Clone(context.Background()))
		f.mu.Unlock()
		handler(w, r)
	}))
	t.Cleanup(f.server.Close)
	f.relay = newSocksRelay(t, f.server.Listener.Addr().String())

	pool := x509.NewCertPool()
	pool.AddCert(f.server.Certificate())
	f.w = newWorld(t, func(w *world) {
		w.cfg.SocksPort = f.relay.port()
		w.cfg.RefreshRootCAs = pool
	})
	f.w.eng.setRunning(true)
	return f
}

func (f *providerFixture) received() []*http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*http.Request(nil), f.requests...)
}

// setRunning flips the fake engine's phase without a start, which is what a
// refresh keys on.
func (e *recordingEngine) setRunning(v bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running = v
}

// forbidLocalLookups swaps the process resolver for one that fails the test
// on any query, for the duration of the test.
//
// The SOCKS dialer is meant to hand the NAME to the proxy, so nothing in a
// refresh should ever ask this machine's resolver anything. A refresh that
// resolved locally would put the provider's name on the ISP's resolver, which
// is the disclosure the whole routing argument exists to prevent.
func forbidLocalLookups(t *testing.T) {
	t.Helper()
	original := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			t.Errorf("a refresh asked the host resolver something (%s %s); the name must go to the proxy unresolved", network, address)
			return nil, errors.New("local lookups are forbidden in this test")
		},
	}
	t.Cleanup(func() { net.DefaultResolver = original })
}

// assertNoAddressIn fails if the text carries the address or any part of it
// that would identify the provider or the account.
func assertNoAddressIn(t *testing.T, what, text string) {
	t.Helper()
	for _, part := range []string{fakeSubscriptionURL, fakeSubscriptionHost, "fake-refresh-token-not-real"} {
		if strings.Contains(text, part) {
			t.Errorf("%s carries part of the subscription address (%q)", what, part)
		}
	}
}

// ---------------------------------------------------------------------------
// The gates before any socket opens
// ---------------------------------------------------------------------------

// TestRefreshIsRefusedUnlessTheTunnelIsRunning: the request leaves through
// the engine's loopback inbound and nothing else, so with no engine there is
// nothing to leave through, and the refusal happens before a dial. The relay
// counts connections to prove it.
func TestRefreshIsRefusedUnlessTheTunnelIsRunning(t *testing.T) {
	forbidLocalLookups(t)
	relay := newSocksRelay(t, "127.0.0.1:1")
	w := newWorld(t, func(w *world) { w.cfg.SocksPort = relay.port() })

	for _, phase := range []string{"stopped", "failed"} {
		w.eng.setRunning(false)
		_, err := w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
		if err == nil {
			t.Fatalf("engine %s: the refresh was not refused", phase)
		}
		if got := faultOf(err); got != panel.FaultNotRunning {
			t.Errorf("engine %s: fault %q, want %q", phase, got, panel.FaultNotRunning)
		}
		if !strings.Contains(err.Error(), "the tunnel is not running") {
			t.Errorf("engine %s: the error does not name the state: %v", phase, err)
		}
		assertNoAddressIn(t, "the error", err.Error())
	}
	if n, _, _ := relay.seen(); n != 0 {
		t.Fatalf("%d connections reached the proxy for a refresh that was refused", n)
	}
	assertNoAddressIn(t, "the log", w.logs.String())
}

// TestRefreshRefusesABadAddressBeforeDialling is the defence in depth behind
// internal/state's own check. Each refused shape opens no connection, and an
// IP literal is the one that matters most: the engine sends a private literal
// out of the direct outbound, so accepting it would be the one way a refresh
// could leave the box in the clear.
func TestRefreshRefusesABadAddressBeforeDialling(t *testing.T) {
	forbidLocalLookups(t)
	relay := newSocksRelay(t, "127.0.0.1:1")
	w := newWorld(t, func(w *world) { w.cfg.SocksPort = relay.port() })
	w.eng.setRunning(true)

	for _, bad := range []string{
		"",
		"http://" + fakeSubscriptionHost + "/sub",
		"https:///sub",
		"https://192.0.2.10/sub",
		"https://[2001:db8::1]:8443/sub",
		"https://10.0.0.1/sub",
		"https://user:pass@" + fakeSubscriptionHost + "/sub",
		"not an address",
	} {
		_, err := w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: bad})
		if err == nil {
			t.Errorf("%q was accepted", bad)
			continue
		}
		if got := faultOf(err); got != panel.FaultRefreshBadAddress {
			t.Errorf("%q: fault %q, want %q", bad, got, panel.FaultRefreshBadAddress)
		}
		if bad != "" && strings.Contains(err.Error(), bad) {
			t.Errorf("the error quotes the address: %v", err)
		}
	}
	if n, _, _ := relay.seen(); n != 0 {
		t.Fatalf("%d connections reached the proxy for addresses that were refused", n)
	}
}

// ---------------------------------------------------------------------------
// The happy path, and what it proves
// ---------------------------------------------------------------------------

// TestRefreshHandsTheNameToTheProxyAndResolvesNothing is acceptance item 2 of
// the design: the request is observed on the loopback SOCKS listener with the
// HOST NAME in the SOCKS request, and none on the host resolver.
func TestRefreshHandsTheNameToTheProxyAndResolvesNothing(t *testing.T) {
	forbidLocalLookups(t)
	const body = "vless://11111111-2222-4333-8444-555555555555@server.example.com:443?type=tcp&security=none#one"
	f := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1; download=2; total=3; expire=4")
		w.Header().Set("Profile-Title", "base64:UHJvdmlkZXI=")
		w.Header().Set("Content-Disposition", `attachment; filename="sub.txt"`)
		w.Header().Set("Profile-Update-Interval", "24")
		w.Header().Set("Set-Cookie", "session=should-not-come-back")
		w.Header().Set("X-Provider-Secret", "should-not-come-back-either")
		w.Header().Set("Server", "nginx")
		_, _ = io.WriteString(w, body)
	})

	reply, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
	if err != nil {
		t.Fatalf("Refresh: %v\nlog:\n%s", err, f.w.logs.String())
	}
	if reply.Status != http.StatusOK {
		t.Errorf("status %d, want 200", reply.Status)
	}
	if string(reply.Body) != body {
		t.Errorf("body %q, want the provider's body", reply.Body)
	}

	// The proxy saw the name, not an address.
	n, domains, literals := f.relay.seen()
	if n != 1 {
		t.Errorf("%d connections reached the proxy, want 1", n)
	}
	if len(literals) != 0 {
		t.Errorf("the proxy was handed an IP literal %v; the name must be passed through unresolved", literals)
	}
	if len(domains) != 1 || domains[0] != fakeSubscriptionHost {
		t.Errorf("the proxy was handed %v, want [%s]", domains, fakeSubscriptionHost)
	}

	// The allow-listed headers come back, keyed in lower case, and nothing
	// else does.
	want := map[string]string{
		"subscription-userinfo":   "upload=1; download=2; total=3; expire=4",
		"profile-title":           "base64:UHJvdmlkZXI=",
		"content-disposition":     `attachment; filename="sub.txt"`,
		"profile-update-interval": "24",
	}
	for k, v := range want {
		if reply.Headers[k] != v {
			t.Errorf("header %q = %q, want %q", k, reply.Headers[k], v)
		}
	}
	for k := range reply.Headers {
		if _, ok := want[k]; !ok {
			t.Errorf("header %q came back and is not on the allow list", k)
		}
	}

	// What the provider saw: a fixed user agent, no cookie, the path and the
	// query intact.
	got := f.received()
	if len(got) != 1 {
		t.Fatalf("the provider received %d requests, want 1", len(got))
	}
	if ua := got[0].Header.Get("User-Agent"); ua != "Caspian/"+panel.Version {
		t.Errorf("User-Agent %q, want %q", ua, "Caspian/"+panel.Version)
	}
	if c := got[0].Header.Get("Cookie"); c != "" {
		t.Errorf("a cookie was sent: %q", c)
	}
	if got[0].URL.Path != "/api/sub" || got[0].URL.RawQuery != "token=fake-refresh-token-not-real" {
		t.Errorf("the provider was asked for %s?%s", got[0].URL.Path, got[0].URL.RawQuery)
	}
	if got[0].Host != fakeSubscriptionHost {
		t.Errorf("Host header %q, want %q", got[0].Host, fakeSubscriptionHost)
	}

	// A second press is a second connection: nothing is kept alive between
	// presses.
	if _, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL}); err != nil {
		t.Fatalf("second Refresh: %v", err)
	}
	if n, _, _ := f.relay.seen(); n != 2 {
		t.Errorf("%d connections after two presses, want 2: a connection was reused across presses", n)
	}

	assertNoAddressIn(t, "the log", f.w.logs.String())
}

// TestRefreshCapsEachHeaderValue: a provider cannot spend the frame with one
// header. Over the cap the header is dropped, not truncated, because a
// truncated title is a wrong title and a dropped one is no title.
func TestRefreshCapsEachHeaderValue(t *testing.T) {
	forbidLocalLookups(t)
	f := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Profile-Title", strings.Repeat("t", maxRefreshHeaderBytes+1))
		w.Header().Set("Subscription-Userinfo", strings.Repeat("u", maxRefreshHeaderBytes))
		_, _ = io.WriteString(w, "body")
	})
	reply, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if _, ok := reply.Headers["profile-title"]; ok {
		t.Error("a header over the cap came back")
	}
	if len(reply.Headers["subscription-userinfo"]) != maxRefreshHeaderBytes {
		t.Error("a header exactly at the cap was dropped or cut")
	}
}

// ---------------------------------------------------------------------------
// The refusals after the socket opens
// ---------------------------------------------------------------------------

// TestRefreshRefusesARedirectToPlainHTTP is acceptance item 6, second half:
// the redirect is refused BEFORE it is followed, so the plain address is
// never connected to. The relay counts connections to prove it.
func TestRefreshRefusesARedirectToPlainHTTP(t *testing.T) {
	forbidLocalLookups(t)
	f := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+fakeSubscriptionHost+"/plain", http.StatusFound)
	})
	_, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
	if err == nil {
		t.Fatal("a redirect to plain http was followed")
	}
	if got := faultOf(err); got != panel.FaultRefreshNotHTTPS {
		t.Errorf("fault %q, want %q", got, panel.FaultRefreshNotHTTPS)
	}
	assertNoAddressIn(t, "the error", err.Error())
	if n, _, _ := f.relay.seen(); n != 1 {
		t.Errorf("%d connections reached the proxy, want 1: the plain address must not be connected to", n)
	}
}

// TestRefreshRefusesARedirectToAnIPLiteral: the same check the address gets
// on the way in is applied to every hop, because a redirect to a private
// literal is the direct-outbound shape by another route.
func TestRefreshRefusesARedirectToAnIPLiteral(t *testing.T) {
	forbidLocalLookups(t)
	f := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://10.0.0.1/sub", http.StatusFound)
	})
	_, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
	if err == nil {
		t.Fatal("a redirect to an IP literal was followed")
	}
	if got := faultOf(err); got != panel.FaultRefreshBadAddress {
		t.Errorf("fault %q, want %q", got, panel.FaultRefreshBadAddress)
	}
	if n, _, _ := f.relay.seen(); n != 1 {
		t.Errorf("%d connections reached the proxy, want 1", n)
	}
}

// TestRefreshFollowsAtMostThreeRedirects: three https hops are followed and
// a fourth is refused as no answer, so a provider that loops cannot hold the
// root process for the whole timeout.
func TestRefreshFollowsAtMostThreeRedirects(t *testing.T) {
	forbidLocalLookups(t)
	hops := func(n int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var i int
			fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/hop"), "%d", &i)
			if r.URL.Path == "/api/sub" {
				i = 0
			}
			if i < n {
				http.Redirect(w, r, fmt.Sprintf("https://%s/hop%d", fakeSubscriptionHost, i+1), http.StatusFound)
				return
			}
			_, _ = io.WriteString(w, "arrived")
		}
	}

	t.Run("three hops arrive", func(t *testing.T) {
		f := newProvider(t, hops(3))
		reply, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
		if err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		if string(reply.Body) != "arrived" {
			t.Errorf("body %q after three redirects", reply.Body)
		}
	})

	t.Run("four hops are refused", func(t *testing.T) {
		f := newProvider(t, hops(4))
		_, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
		if err == nil {
			t.Fatal("four redirects were followed")
		}
		if got := faultOf(err); got != panel.FaultRefreshNoAnswer {
			t.Errorf("fault %q, want %q", got, panel.FaultRefreshNoAnswer)
		}
		assertNoAddressIn(t, "the error", err.Error())
	})
}

// TestRefreshRefusesABodyOverTheCap: the cap is the same one a pasted config
// is held to, so a body over it could not have been pasted either. Exactly
// the cap is accepted, one byte more is refused, and the refusal names the
// size rule rather than the provider.
func TestRefreshRefusesABodyOverTheCap(t *testing.T) {
	forbidLocalLookups(t)
	serve := func(n int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(bytes.Repeat([]byte("a"), n))
		}
	}

	t.Run("exactly the cap", func(t *testing.T) {
		f := newProvider(t, serve(maxRefreshBodyBytes))
		reply, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
		if err != nil {
			t.Fatalf("a body of exactly %d bytes was refused: %v", maxRefreshBodyBytes, err)
		}
		if len(reply.Body) != maxRefreshBodyBytes {
			t.Errorf("%d bytes came back, want %d", len(reply.Body), maxRefreshBodyBytes)
		}
	})

	t.Run("one byte over", func(t *testing.T) {
		f := newProvider(t, serve(maxRefreshBodyBytes+1))
		reply, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
		if err == nil {
			t.Fatalf("a body of %d bytes was accepted (%d came back)", maxRefreshBodyBytes+1, len(reply.Body))
		}
		if got := faultOf(err); got != panel.FaultRefreshTooLarge {
			t.Errorf("fault %q, want %q", got, panel.FaultRefreshTooLarge)
		}
		if len(reply.Body) != 0 {
			t.Error("a refused body was returned anyway")
		}
	})
}

// TestRefreshReportsAProviderErrorStatusWithoutABody: a status outside 2xx is
// reported as the number it is, and the provider's error page does not cross
// the socket, because nothing about it is a configuration.
func TestRefreshReportsAProviderErrorStatusWithoutABody(t *testing.T) {
	forbidLocalLookups(t)
	for _, code := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusServiceUnavailable} {
		f := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			_, _ = io.WriteString(w, "<html>an error page with the account name in it</html>")
		})
		reply, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
		if err != nil {
			t.Fatalf("status %d was reported as an error: %v", code, err)
		}
		if reply.Status != code {
			t.Errorf("status %d, want %d", reply.Status, code)
		}
		if len(reply.Body) != 0 {
			t.Errorf("status %d: the error page came back (%d bytes)", code, len(reply.Body))
		}
	}
}

// TestRefreshReportsNoAnswerWhenNothingListens: a tunnel that is up on paper
// and dead in practice, or a provider that is down, is "no answer", and the
// error says so without naming the address.
func TestRefreshReportsNoAnswerWhenNothingListens(t *testing.T) {
	forbidLocalLookups(t)

	t.Run("no proxy inbound", func(t *testing.T) {
		// A port nothing listens on: take one and release it.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := uint16(ln.Addr().(*net.TCPAddr).Port)
		ln.Close()
		w := newWorld(t, func(w *world) { w.cfg.SocksPort = port })
		w.eng.setRunning(true)

		_, err = w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
		if err == nil {
			t.Fatal("a refresh with no proxy to go through succeeded")
		}
		if got := faultOf(err); got != panel.FaultRefreshNoAnswer {
			t.Errorf("fault %q, want %q", got, panel.FaultRefreshNoAnswer)
		}
		assertNoAddressIn(t, "the error", err.Error())
		assertNoAddressIn(t, "the log", w.logs.String())
	})

	t.Run("the provider is down", func(t *testing.T) {
		f := newProvider(t, func(w http.ResponseWriter, r *http.Request) {})
		f.server.Close()
		_, err := f.w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
		if err == nil {
			t.Fatal("a refresh from a provider that is down succeeded")
		}
		if got := faultOf(err); got != panel.FaultRefreshNoAnswer {
			t.Errorf("fault %q, want %q", got, panel.FaultRefreshNoAnswer)
		}
		assertNoAddressIn(t, "the error", err.Error())
	})

	t.Run("the certificate is not trusted", func(t *testing.T) {
		// The provider's certificate is self-signed and the pool is left at
		// the system roots, which is the production setting. A provider with
		// a certificate this box cannot verify is a provider that did not
		// answer, and nothing is read from it.
		f := newProvider(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "body") })
		w := newWorld(t, func(w *world) { w.cfg.SocksPort = f.relay.port() })
		w.eng.setRunning(true)
		_, err := w.svc.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
		if err == nil {
			t.Fatal("an untrusted certificate was accepted")
		}
		if got := faultOf(err); got != panel.FaultRefreshNoAnswer {
			t.Errorf("fault %q, want %q", got, panel.FaultRefreshNoAnswer)
		}
		if len(f.received()) != 0 {
			t.Error("the provider received a request over a connection whose certificate was not verified")
		}
	})

	t.Run("the caller gave up", func(t *testing.T) {
		f := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		})
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_, err := f.w.svc.Refresh(ctx, panel.RefreshRequest{URL: fakeSubscriptionURL})
		if err == nil {
			t.Fatal("a refresh whose caller gave up succeeded")
		}
		if got := faultOf(err); got != panel.FaultRefreshNoAnswer {
			t.Errorf("fault %q, want %q", got, panel.FaultRefreshNoAnswer)
		}
	})
}

// ---------------------------------------------------------------------------
// The wire
// ---------------------------------------------------------------------------

// TestRefreshCrossesTheSocket drives the action through a real socket: the
// argument goes one way, the body and the allow-listed headers come back,
// and a fault comes back as its own word.
func TestRefreshCrossesTheSocket(t *testing.T) {
	forbidLocalLookups(t)
	const body = "vless://11111111-2222-4333-8444-555555555555@server.example.com:443?type=tcp&security=none#one"
	f := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1; download=2; total=3; expire=4")
		_, _ = io.WriteString(w, body)
	})
	path := serving(t, f.w, ListenConfig{ServiceAccount: currentAccount(t)})
	c := NewClient(path)

	reply, err := c.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
	if err != nil {
		t.Fatalf("Refresh over the socket: %v", err)
	}
	if string(reply.Body) != body || reply.Status != 200 {
		t.Errorf("reply %v did not survive the socket", reply)
	}
	if reply.Headers["subscription-userinfo"] != "upload=1; download=2; total=3; expire=4" {
		t.Errorf("headers %v did not survive the socket", reply.Headers)
	}

	f.w.eng.setRunning(false)
	_, err = c.Refresh(context.Background(), panel.RefreshRequest{URL: fakeSubscriptionURL})
	if got := panel.FaultOf(err); got != panel.FaultNotRunning {
		t.Errorf("the fault crossed the socket as %q, want %q", got, panel.FaultNotRunning)
	}

	// Nothing on the privileged side wrote the address anywhere.
	assertNoAddressIn(t, "the log", f.w.logs.String())
}

// TestTheWireRefusesARefreshWithoutItsArgumentAndAnArgumentOnAnotherAction
// extends TestNothingBeyondTheClosedVocabularyIsAccepted by the two shapes the
// new action adds. Neither reaches the service.
func TestTheWireRefusesARefreshWithoutItsArgumentAndAnArgumentOnAnotherAction(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		want    Refusal
	}{
		{"a refresh with no argument", mustJSON(t, map[string]any{"v": protocolVersion, "action": "refresh"}), RefusalMissingArg},
		{"a status carrying a refresh argument", mustJSON(t, map[string]any{
			"v": protocolVersion, "action": "status", "refresh": map[string]any{"URL": fakeSubscriptionURL},
		}), RefusalUnexpectedArg},
		{"a refresh carrying a start argument", mustJSON(t, map[string]any{
			"v": protocolVersion, "action": "refresh", "refresh": map[string]any{"URL": fakeSubscriptionURL},
			"start": map[string]any{},
		}), RefusalUnexpectedArg},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			relay := newSocksRelay(t, "127.0.0.1:1")
			w := newWorld(t, func(w *world) { w.cfg.SocksPort = relay.port() })
			w.eng.setRunning(true)
			path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})
			resp := sendRaw(t, path, tc.payload)
			if resp.Refusal != tc.want {
				t.Fatalf("refusal was %q, want %q (fault %q)", resp.Refusal, tc.want, resp.Fault)
			}
			if n, _, _ := relay.seen(); n != 0 {
				t.Fatalf("%d connections reached the proxy for a message that was refused", n)
			}
		})
	}
}

// TestALargestRefreshReplyFitsInOneFrame pins the arithmetic the frame bound
// rests on. A body of exactly the cap, base64-encoded by encoding/json, plus
// four headers each at their cap, has to fit under maxFrameBytes, or the
// server's own writeFrame refuses the reply and the panel sees a service that
// hung up rather than a configuration.
func TestALargestRefreshReplyFitsInOneFrame(t *testing.T) {
	reply := panel.RefreshReply{
		Status:  200,
		Body:    bytes.Repeat([]byte{0xff}, maxRefreshBodyBytes),
		Headers: map[string]string{},
	}
	for _, k := range refreshHeaderAllowList {
		reply.Headers[k] = strings.Repeat(`"`, maxRefreshHeaderBytes) // the worst case for JSON escaping
	}
	b, err := json.Marshal(wireResponse{Refresh: &reply})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > maxFrameBytes {
		t.Fatalf("the largest reply encodes to %d bytes, over the frame bound of %d", len(b), maxFrameBytes)
	}
	var buf bytes.Buffer
	if err := writeFrame(&buf, b); err != nil {
		t.Fatalf("writeFrame refused the largest reply: %v", err)
	}
	if got := binary.BigEndian.Uint32(buf.Bytes()[:4]); int(got) != len(b) {
		t.Fatalf("frame length %d, want %d", got, len(b))
	}
}
