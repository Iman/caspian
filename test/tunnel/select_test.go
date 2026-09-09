// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// ---------------------------------------------------------------------------
// A pasted text holding two entries, and which of them the tunnel uses
//
// internal/link.Select(raw, i) returns entry i of a pasted list with the same
// corrections Parse applies to the first, and internal/panel's bringUp hands
// its outbound to xcfg the way it always handed Parse's (internal/panel/
// handlers.go, bringUp). The carriage suite in carry_test.go proves that each
// protocol's link moves bytes; nothing there says that entry TWO of a
// two-line paste moves bytes to server two. That is what this file holds,
// with the same four controls carry() uses and one more: the server that was
// NOT chosen has an origin of its own, and that origin must serve nothing.
//
// What "server one saw no connection" means here, exactly. Each server's only
// outbound redirects to its own origin, so a connection that reached server
// one would have produced a request at origin one; origin one serving nothing
// is the observable. The server's own accept is not observed: xray-core owns
// the listening socket and this package does not stand in front of it.
// ---------------------------------------------------------------------------

// entryServer is one real xray-core server with an origin of its own behind
// it, and the share link that reaches it.
type entryServer struct {
	name   string
	port   int
	origin *endpoint
	link   string
}

// protocolNamed returns the row of the carriage table with that name. The
// rows are used rather than restated so that the inbound and the link here
// are the same pair carry_test.go proves.
func protocolNamed(t *testing.T, name string) protocolCase {
	t.Helper()
	for _, p := range protocolCases() {
		if p.name == name {
			return p
		}
	}
	t.Fatalf("no protocol row called %q", name)
	return protocolCase{}
}

// startEntryServer brings up one server of the given protocol in front of a
// fresh origin, and returns the link a user would paste for it.
func startEntryServer(t *testing.T, protocol string) entryServer {
	t.Helper()
	p := protocolNamed(t, protocol)
	origin := startEndpoint(t, newToken(t, "origin-"+protocol))
	cert := makeServerCert(t)
	port := freeLoopbackPort(t)
	startXrayServer(t, serverConfig(p.inbound(port, cert), origin.port))
	return entryServer{
		name:   protocol,
		port:   port,
		origin: origin,
		link:   p.shareLink(port, p.secret, cert.pinHex),
	}
}

// twoLinePaste is what a person pastes: both links, one per line.
func twoLinePaste(first, second entryServer) string {
	return first.link + "\n" + second.link
}

// startClientEntry is startClient with link.Select in place of link.Parse: the
// product path from pasted text to running engine, at the chosen entry.
// Everything after the parse is the same code, deliberately, so that the only
// thing this file adds to the carriage proof is WHICH entry.
//
// It returns the composed document as well as the engine, so a test can read
// which server the document names without trusting the origin alone, and the
// clamp flag Select reports, which is the product's own word for "the index
// was past the end and entry one was used".
func startClientEntry(t *testing.T, raw string, entry int, socksPort int) (*engine.Engine, []byte, bool, error) {
	t.Helper()
	l, clamped, err := link.Select(raw, entry)
	if err != nil {
		return nil, nil, false, fmt.Errorf("entry %d of the pasted text did not parse: %w", entry, err)
	}
	o := xcfg.Defaults()
	o.Link = l
	o.TUN.Disabled = true
	o.SOCKS.Listen = "127.0.0.1"
	o.SOCKS.Port = uint16(socksPort)
	o.LogLevel = xcfg.LogInfo

	doc, err := xcfg.Build(o)
	if err != nil {
		return nil, nil, false, fmt.Errorf("xcfg.Build refused entry %d: %w", entry, err)
	}

	e := engine.New()
	if err := e.Start(context.Background(), doc); err != nil {
		return nil, nil, false, fmt.Errorf("the engine refused to start with the composed document: %w", err)
	}
	t.Cleanup(func() { _ = e.Stop() })

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c, derr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), time.Second)
		if derr == nil {
			_ = c.Close()
			return e, doc, clamped, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil, nil, false, fmt.Errorf("the engine reports %s but nothing ever accepted on 127.0.0.1:%d",
		e.State().Phase, socksPort)
}

// carryEntry starts the client on entry `entry` of raw and requires one
// request through it to reach `want` and nothing to reach `other`. wantClamp
// is whether Select is expected to report that the index was past the end.
//
// Like carry, it returns an error rather than failing, so that it can be run
// in the failing direction by TestTheEntryProofCanFail.
func carryEntry(t *testing.T, raw string, entry int, wantClamp bool, want, other entryServer) error {
	t.Helper()

	sentinel := startEndpoint(t, newToken(t, "bypass-sentinel"))
	socksPort := freeLoopbackPort(t)
	path := "/" + newToken(t, "entry-path")
	decoyAuthority := fmt.Sprintf("%s:%d", originHost, sentinel.port)

	// The same control carry runs: the client's view of the origin must be
	// unreachable without the tunnel.
	if body, err := directGet(originHost, sentinel.port, path); err == nil && (body == want.origin.body || body == other.origin.body) {
		return fmt.Errorf("an origin answered a direct request to %s, so this test would pass with no tunnel at all",
			decoyAuthority)
	}
	sentinelBefore := len(sentinel.requests())
	wantBefore := len(want.origin.requests())
	otherBefore := len(other.origin.requests())

	e, doc, clamped, err := startClientEntry(t, raw, entry, socksPort)
	if err != nil {
		return fmt.Errorf("the client did not come up: %w", err)
	}
	var problems []error
	if clamped != wantClamp {
		problems = append(problems, fmt.Errorf(
			"link.Select reported clamped=%t for entry %d, want %t", clamped, entry, wantClamp))
	}
	if err := documentNamesServer(doc, want.port, other.port); err != nil {
		problems = append(problems, err)
	}

	body, err := socksGet(fmt.Sprintf("127.0.0.1:%d", socksPort), originHost, sentinel.port, path, carryTimeout)
	if err != nil {
		problems = append(problems, fmt.Errorf("no traffic reached an origin through the tunnel: %w", err))
		return errors.Join(problems...)
	}
	switch body {
	case want.origin.body:
	case other.origin.body:
		problems = append(problems, fmt.Errorf(
			"the answer came from the %s server's origin, not the chosen %s server's: the tunnel used the wrong entry",
			other.name, want.name))
	case sentinel.body:
		problems = append(problems, errors.New(
			"the answer came from the bypass sentinel, not an origin: the request never entered the tunnel"))
	default:
		problems = append(problems, fmt.Errorf("the tunnel returned %q, which is no origin's token", body))
	}
	if err := checkOriginSawTheTunnelledRequest(want.origin, decoyAuthority, path); err != nil {
		problems = append(problems, fmt.Errorf("the chosen %s server's origin: %w", want.name, err))
	}
	if n := len(other.origin.requests()) - otherBefore; n != 0 {
		problems = append(problems, fmt.Errorf(
			"the %s server's origin served %d request(s), so the server that was NOT chosen carried traffic: %+v",
			other.name, n, other.origin.requests()))
	}
	if n := len(want.origin.requests()) - wantBefore; n != 1 {
		problems = append(problems, fmt.Errorf("the chosen origin served %d request(s), want exactly 1", n))
	}
	if hits := sentinel.requests(); len(hits) > sentinelBefore {
		problems = append(problems, fmt.Errorf(
			"the tunnelled request reached the bypass sentinel %d time(s)", len(hits)-sentinelBefore))
	}
	if err := checkTheProxyOutboundCarriedIt(e, decoyAuthority); err != nil {
		problems = append(problems, err)
	}
	if st := e.State(); st.Phase != engine.PhaseRunning {
		problems = append(problems, fmt.Errorf("the engine finished the run in phase %s (%q), not running",
			st.Phase, st.Reason))
	}
	return errors.Join(problems...)
}

// documentNamesServer reads the composed document for the server port it
// dials. The document is what the appliance would hand the privileged side,
// so it is the earliest place the choice is visible, before any byte moves.
//
// internal/xcfg indents its document with two spaces (internal/xcfg/build.go,
// Build calls json.Indent), so the port appears as `"port": N`. The wanted
// port is required to appear in that form; if the layout ever changes this
// fails on the WANTED port with a message saying so, rather than passing
// because the unwanted one was not found either.
func documentNamesServer(doc []byte, wantPort, otherPort int) error {
	text := string(doc)
	wanted := fmt.Sprintf(`"port": %d`, wantPort)
	unwanted := fmt.Sprintf(`"port": %d`, otherPort)
	if !strings.Contains(text, wanted) {
		if strings.Contains(text, fmt.Sprintf(`"port":%d`, wantPort)) {
			return fmt.Errorf("the document's layout changed (compact port field); update documentNamesServer")
		}
		return fmt.Errorf("the composed document does not dial port %d, the chosen server", wantPort)
	}
	if strings.Contains(text, unwanted) {
		return fmt.Errorf("the composed document dials port %d, the server that was not chosen", otherPort)
	}
	return nil
}

// ---------------------------------------------------------------------------
// The tests
// ---------------------------------------------------------------------------

// TestTheChosenEntryOfATwoLinePasteIsTheServerTheTunnelUses is the claim: a
// two-line paste, entry two chosen, and the request comes out at server two
// while server one's origin serves nothing.
func TestTheChosenEntryOfATwoLinePasteIsTheServerTheTunnelUses(t *testing.T) {
	first := startEntryServer(t, "vless")
	second := startEntryServer(t, "shadowsocks")
	raw := twoLinePaste(first, second)

	start := time.Now()
	if err := carryEntry(t, raw, 1, false, second, first); err != nil {
		t.Fatalf("entry two of the paste did not carry traffic to the second server:\n%v", err)
	}
	t.Logf("entry two of a two-line paste carried real traffic to the second server, and the first saw none, in %s",
		time.Since(start).Round(time.Millisecond))
}

// TestAChoicePastTheEndFallsBackToEntryOne is the unhappy path: Select(raw, 5)
// on a two-line paste reports the clamp and the request reaches server ONE.
// That fallback is the product's choice (a stale index must not strand the
// box: internal/link/list.go, Select), and this pins which server it lands on.
func TestAChoicePastTheEndFallsBackToEntryOne(t *testing.T) {
	first := startEntryServer(t, "vless")
	second := startEntryServer(t, "shadowsocks")
	raw := twoLinePaste(first, second)

	if err := carryEntry(t, raw, 5, true, first, second); err != nil {
		t.Fatalf("an index past the end did not fall back to entry one:\n%v", err)
	}
	t.Log("Select(raw, 5) reported the clamp and the request reached the first server")
}

// TestTheEntryProofCanFail runs carryEntry in the failing direction, for the
// same reason carry_test.go runs carry under defects: a proof that has only
// ever passed is a proof nobody has seen work.
//
// The two defects are the two ways this could be wrong: the choice is
// ignored (entry one is used whatever was chosen), and the clamp is not
// reported (a stale index is used silently).
func TestTheEntryProofCanFail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped under -short: this starts the table twice more")
	}
	t.Run("the choice is ignored and entry one is used", func(t *testing.T) {
		first := startEntryServer(t, "vless")
		second := startEntryServer(t, "shadowsocks")
		raw := twoLinePaste(first, second)
		// Entry zero is what a client that ignored the choice would start on.
		// The proof still wants the second server, and must say no.
		err := carryEntry(t, raw, 0, false, second, first)
		if err == nil {
			t.Fatal("the proof accepted traffic that reached the first server as proof the second was used")
		}
		if !strings.Contains(err.Error(), "wrong entry") && !strings.Contains(err.Error(), "NOT chosen") {
			t.Fatalf("the proof complained, but not about the wrong server: %v", err)
		}
		t.Logf("caught, as required: %v", err)
	})
	t.Run("the clamp is not reported", func(t *testing.T) {
		first := startEntryServer(t, "vless")
		second := startEntryServer(t, "shadowsocks")
		raw := twoLinePaste(first, second)
		// Entry one is in the list, so Select reports no clamp. A proof that
		// expects a clamp here has to notice its absence.
		err := carryEntry(t, raw, 0, true, first, second)
		if err == nil {
			t.Fatal("the proof accepted an in-range choice as a clamped one")
		}
		if !strings.Contains(err.Error(), "clamped=false") {
			t.Fatalf("the proof complained, but not about the clamp: %v", err)
		}
		t.Logf("caught, as required: %v", err)
	})
}
