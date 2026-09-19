// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// The loopback SOCKS port fallback, added 2026-09-12 after GitHub issue #2: a
// Windows 11 box where another proxy client held 127.0.0.1:10808 and the engine
// could not bind. These tests hold real loopback ports rather than faking the
// probe, because the probe IS the claim: that the operating system was asked.

// socksInboundPort reads the SOCKS inbound's port out of a recorded engine
// document, the way the engine itself would.
func socksInboundPort(t *testing.T, doc []byte) uint16 {
	t.Helper()
	var d struct {
		Inbounds []struct {
			Tag  string `json:"tag"`
			Port uint16 `json:"port"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(doc, &d); err != nil {
		t.Fatalf("the engine document is not JSON: %v", err)
	}
	for _, in := range d.Inbounds {
		if in.Tag == xcfg.TagSOCKSIn {
			return in.Port
		}
	}
	t.Fatalf("no inbound tagged %q in the engine document", xcfg.TagSOCKSIn)
	return 0
}

func lastDocument(t *testing.T, w *world) []byte {
	t.Helper()
	docs := w.eng.documents()
	if len(docs) == 0 {
		t.Fatal("the engine was never handed a document")
	}
	return docs[len(docs)-1]
}

// TestTheLocalProxyMovesWhenAnotherProgramHoldsThePreferredPort is the report
// itself: the preferred port is taken by something that is not Caspian. The
// run has to bind elsewhere, tell the engine, tell the panel, and say so in
// the log with both numbers, and Stop has to forget the port.
func TestTheLocalProxyMovesWhenAnotherProgramHoldsThePreferredPort(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not take a loopback port to block it: %v", err)
	}
	defer blocker.Close()
	held := uint16(blocker.Addr().(*net.TCPAddr).Port)

	w := newWorld(t, func(w *world) { w.cfg.SocksPort = held })
	ctx := context.Background()
	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("Start with the preferred port held: %v", err)
	}

	chosen := socksInboundPort(t, lastDocument(t, w))
	if chosen == held {
		t.Fatalf("the engine document still names the held port %d", held)
	}
	if chosen == 0 {
		t.Fatal("the engine document names port 0, which the engine would refuse")
	}

	st, err := w.svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if want := fmt.Sprintf("127.0.0.1:%d", chosen); st.LocalProxy != want {
		t.Errorf("Status.LocalProxy = %q, want %q, the port the engine document names", st.LocalProxy, want)
	}
	logs := w.logs.String()
	if !strings.Contains(logs, "using a free one") ||
		!strings.Contains(logs, fmt.Sprintf("preferred=%d", held)) ||
		!strings.Contains(logs, fmt.Sprintf("chosen=%d", chosen)) {
		t.Errorf("the log does not record the move with both ports:\n%s", logs)
	}
	// The note goes to the diagnostics ring, which the advanced view reads, so
	// a person at the box can find the port the proxy moved to.
	if shown := advancedView(t, w); !strings.Contains(shown, fmt.Sprintf("local proxy listening at 127.0.0.1:%d", chosen)) {
		t.Errorf("the advanced view does not name the port the proxy moved to:\n%s", shown)
	}

	if err := w.svc.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	st, err = w.svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status after Stop: %v", err)
	}
	if st.LocalProxy != "" {
		t.Errorf("Status.LocalProxy = %q after Stop, want empty: a stale address is one somebody types in", st.LocalProxy)
	}
}

// TestTheLocalProxyStaysOnThePreferredPortWhenItIsFree is the normal day:
// nothing holds 10808's stand-in, so nothing moves, docs/LAYOUT.md stays true,
// and the log says nothing about a move.
func TestTheLocalProxyStaysOnThePreferredPortWhenItIsFree(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not find a free loopback port: %v", err)
	}
	preferred := uint16(probe.Addr().(*net.TCPAddr).Port)
	_ = probe.Close()

	w := newWorld(t, func(w *world) { w.cfg.SocksPort = preferred })
	ctx := context.Background()
	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := socksInboundPort(t, lastDocument(t, w)); got != preferred {
		t.Errorf("the engine document names port %d, want the free preferred port %d", got, preferred)
	}
	st, err := w.svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if want := fmt.Sprintf("127.0.0.1:%d", preferred); st.LocalProxy != want {
		t.Errorf("Status.LocalProxy = %q, want %q", st.LocalProxy, want)
	}
	if strings.Contains(w.logs.String(), "using a free one") {
		t.Error("the log reports a move that did not happen")
	}
}

// TestTheMacSystemProxyFollowsTheChosenPort: the networksetup steps that point
// every macOS network service at the inbound must carry the port the engine
// bound, not the preferred one. A Mac pointed at 10808 while the engine
// listens elsewhere is a Mac whose proxy setting reaches nothing.
func TestTheMacSystemProxyFollowsTheChosenPort(t *testing.T) {
	cfg := Config{Backend: netcfg.BackendFor(netcfg.PlatformDarwin), DNSPort: 53, LocalDNSPort: 5354, SocksPort: 10808, PanelPort: 8088}
	s := &Service{cfg: cfg}
	const chosen uint16 = 20808
	opts, err := s.netOptionsFor(startRequest(t), chosen)
	if err != nil {
		t.Fatalf("netOptionsFor: %v", err)
	}
	if !opts.SystemSOCKS.Enabled || opts.SystemSOCKS.Port != chosen {
		t.Fatalf("Darwin system SOCKS options = %+v, want port %d", opts.SystemSOCKS, chosen)
	}
	// And with the preferred port itself, nothing changes.
	opts, err = s.netOptionsFor(startRequest(t), cfg.SocksPort)
	if err != nil || opts.SystemSOCKS.Port != cfg.SocksPort {
		t.Fatalf("with the preferred port: %+v, %v", opts.SystemSOCKS, err)
	}
}
