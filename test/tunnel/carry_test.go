// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package tunnel

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// ---------------------------------------------------------------------------
// The two deadlines
// ---------------------------------------------------------------------------

// carryTimeout bounds a request that is expected to succeed, and blockedTimeout
// bounds one that is expected to be stopped.
//
// They are different numbers because they answer different questions, and the
// second one is the one that needs justifying.
//
// A run that is expected to succeed wants a deadline generous enough that a
// loaded machine or the race detector cannot turn a working tunnel into a
// failure, so carryTimeout is far above anything this suite has been observed
// to need. Measured 2026-08-30 on this machine, without the race detector, the
// seven passing rows completed in 44, 5, 5, 4, 6, 7 and 5 milliseconds.
//
// A run that is expected to be BLOCKED is asserting a negative, and a negative
// has to be bounded by something. blockedTimeout is that bound: three seconds
// of nothing arriving, on loopback, where the same request completes in single
// milliseconds when it is not broken. It is a deliberate cap and not an
// incidental one, because two of these protocols do not fail fast by design:
// measured on the same day, a vmess and a shadowsocks server handed the wrong
// credential hold the connection open and answer nothing rather than closing,
// which is anti-probing behaviour and which made the mutation run take 59
// seconds before this cap existed.
const (
	carryTimeout   = 20 * time.Second
	blockedTimeout = 3 * time.Second
)

// ---------------------------------------------------------------------------
// One attempt
// ---------------------------------------------------------------------------

// attempt is everything about a run that a defect is allowed to change.
//
// Every field describes the CLIENT. The server is built from the real
// credential, on the real port, with the real certificate, always, so an
// attempt with a defect in it is a client that disagrees with a correct server:
// which is what a user with a typo in their pasted link has.
type attempt struct {
	clientSecret string
	clientPort   int
	clientPin    string
	startTunnel  bool

	// timeout bounds the request. Zero means carryTimeout.
	timeout time.Duration
}

// defect is a named change to an attempt, plus the requirement that the
// carriage proof notices it. TestEveryCarriageProofCanFail applies each one.
type defect struct {
	name  string
	apply func(t *testing.T, p protocolCase, a *attempt)
}

// defectsFor lists the faults each protocol must be shown to catch.
//
// A protocol with no defect named is a protocol whose proof nobody has watched
// fail, and TestEveryProtocolNamesADefect refuses to let one exist. Same rule,
// and the same reasoning, as test/bdd's BreaksWhen.
func defectsFor(p protocolCase) []defect {
	list := []defect{
		{
			name: "the pasted link carries the wrong credential",
			apply: func(_ *testing.T, p protocolCase, a *attempt) {
				a.clientSecret = p.wrongSecret
			},
		},
		{
			name: "the pasted link names a port nothing is listening on",
			apply: func(t *testing.T, _ protocolCase, a *attempt) {
				a.clientPort = freeLoopbackPort(t)
			},
		},
		{
			name: "the tunnel is never started",
			apply: func(_ *testing.T, _ protocolCase, a *attempt) {
				a.startTunnel = false
			},
		},
	}
	if p.usesTLS {
		list = append(list, defect{
			name: "the pasted link pins a different certificate",
			apply: func(t *testing.T, _ protocolCase, a *attempt) {
				a.clientPin = makeServerCert(t).pinHex
			},
		})
	}
	return list
}

// ---------------------------------------------------------------------------
// The proof
// ---------------------------------------------------------------------------

// carry runs one protocol end to end and reports every proof that did not
// hold.
//
// It returns an error rather than calling t.Fatal so that the same code can be
// used twice: once expecting nil, and once, under a defect, expecting a
// failure. A proof that is only ever run in the passing direction is a proof
// nobody has seen work.
//
// t is still used, for setup that no defect can reach: ports, certificates and
// the origin. A failure there is a broken harness, not a broken product, and it
// should stop the test rather than be reported as a product failure.
func carry(t *testing.T, p protocolCase, a attempt) error {
	t.Helper()

	token := newToken(t, "tunnel-token")
	origin := startEndpoint(t, token)
	sentinel := startEndpoint(t, newToken(t, "bypass-sentinel"))
	cert := makeServerCert(t)
	serverPort := freeLoopbackPort(t)
	socksPort := freeLoopbackPort(t)
	path := "/" + token

	// The origin's port enters the test here and nowhere else. Everything
	// below this line works with the DECOY authority, which is the sentinel's
	// port under a name that does not resolve.
	startXrayServer(t, serverConfig(p.inbound(serverPort, cert), origin.port))

	if a.clientSecret == "" {
		a.clientSecret = p.secret
	}
	if a.clientPort == 0 {
		a.clientPort = serverPort
	}
	if a.clientPin == "" {
		a.clientPin = cert.pinHex
	}
	if a.timeout == 0 {
		a.timeout = carryTimeout
	}
	shareLink := p.shareLink(a.clientPort, a.clientSecret, a.clientPin)

	decoyAuthority := fmt.Sprintf("%s:%d", originHost, sentinel.port)

	// Control, executed before the tunnel is used: the client's view of the
	// origin must be unreachable without it. If this ever returns the token,
	// nothing below proves anything.
	controlBody, controlErr := directGet(originHost, sentinel.port, path)
	if controlErr == nil && controlBody == token {
		return fmt.Errorf(
			"the origin answered a direct request to %s, so this test would pass with no tunnel at all",
			decoyAuthority)
	}
	if controlErr == nil {
		t.Logf("note: %s resolved on this machine, so the control request reached the bypass sentinel. "+
			"The name-does-not-resolve control is not holding here. The other two still are: the client is "+
			"never given the origin's port, and the origin checks the authority the request was addressed "+
			"to.", originHost)
	}
	// Counted AFTER the control, so this test's own control request cannot be
	// reported below as a bypass by the tunnelled one.
	sentinelBeforeTunnel := len(sentinel.requests())

	var e *engine.Engine
	if a.startTunnel {
		var err error
		e, err = startClient(t, shareLink, socksPort)
		if err != nil {
			return fmt.Errorf("the client did not come up: %w", err)
		}
	}

	body, err := socksGet(fmt.Sprintf("127.0.0.1:%d", socksPort), originHost, sentinel.port, path, a.timeout)
	if err != nil {
		return fmt.Errorf("no traffic reached the origin through the %s tunnel: %w", p.name, err)
	}

	var problems []error
	if body != token {
		if body == sentinel.body {
			problems = append(problems, fmt.Errorf(
				"the answer came from the bypass sentinel, not the origin: the request never entered the tunnel"))
		} else {
			problems = append(problems, fmt.Errorf(
				"the tunnel returned %q, which is not this run's origin token", body))
		}
	}
	if err := checkOriginSawTheTunnelledRequest(origin, decoyAuthority, path); err != nil {
		problems = append(problems, err)
	}
	if hits := sentinel.requests(); len(hits) > sentinelBeforeTunnel {
		problems = append(problems, fmt.Errorf(
			"the tunnelled request reached the bypass sentinel on 127.0.0.1:%d %d time(s), so %s resolved to "+
				"a loopback address and the decoy endpoint answered instead of the origin",
			sentinel.port, len(hits)-sentinelBeforeTunnel, originHost))
	}
	if e != nil {
		if err := checkTheProxyOutboundCarriedIt(e, decoyAuthority); err != nil {
			problems = append(problems, err)
		}
		if st := e.State(); st.Phase != engine.PhaseRunning {
			problems = append(problems, fmt.Errorf(
				"the engine finished the run in phase %s (%q), not running", st.Phase, st.Reason))
		}
	}
	return errors.Join(problems...)
}

// checkOriginSawTheTunnelledRequest is the "it arrived from the tunnel" proof.
//
// The origin listens on a port the client was never given. A request that
// reached it carries the authority the CLIENT addressed, because the server's
// freedom "redirect" rewrites the TCP destination and leaves the payload alone.
// So a request whose Host is the decoy authority was redirected by the server;
// a request whose Host is the origin's own address arrived some other way and
// is rejected here.
//
// TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel runs a real direct
// request through this function and requires it to complain, so the rejection
// is a measured behaviour and not a claim about one.
func checkOriginSawTheTunnelledRequest(origin *endpoint, wantHost, wantPath string) error {
	seen := origin.requests()
	if len(seen) == 0 {
		return fmt.Errorf("the origin on 127.0.0.1:%d served no request at all", origin.port)
	}
	if len(seen) != 1 {
		return fmt.Errorf("the origin served %d requests, want exactly 1: %+v", len(seen), seen)
	}
	got := seen[0]
	if got.Host != wantHost {
		return fmt.Errorf(
			"the origin saw a request addressed to %q, want %q. Only the server's redirect turns the second "+
				"into the first, so a request carrying anything else did not come through the tunnel",
			got.Host, wantHost)
	}
	if got.Path != wantPath {
		return fmt.Errorf("the origin saw path %q, want %q", got.Path, wantPath)
	}
	return nil
}

// checkTheProxyOutboundCarriedIt reads the engine's own routing decision out of
// the log ring.
//
// This is the second, independent source. The first is the origin, which says
// where the bytes came out; this says which outbound the client chose, from
// inside the client, and it is the one that would catch the private-address
// rule swallowing the destination: internal/xcfg emits a rule sending every
// private CIDR, 127.0.0.0/8 included, to the direct outbound
// (internal/xcfg/private.go, PrivateRanges). The destination here is a NAME and
// the document's domainStrategy is AsIs, so the router never resolves it and
// the IP rule cannot match; that is the reason this suite addresses the origin
// by name and not by address, and this check is what holds it.
func checkTheProxyOutboundCarriedIt(e *engine.Engine, authority string) error {
	var mentions []string
	for _, entry := range e.Logs() {
		if strings.Contains(entry.Text, authority) {
			mentions = append(mentions, entry.Text)
		}
	}
	if len(mentions) == 0 {
		return fmt.Errorf(
			"the engine log never mentions %s, so the routing decision could not be read. Either the log level "+
				"is below info or xray-core changed what it logs; check the pinned version in go.mod",
			authority)
	}
	var decisions []string
	for _, line := range mentions {
		if strings.Contains(line, "taking detour") {
			decisions = append(decisions, line)
		}
	}
	if len(decisions) == 0 {
		return fmt.Errorf(
			"the engine log mentions %s but records no routing decision for it. app/dispatcher logs "+
				"\"taking detour\" at info; if that wording changed in xray-core this check needs updating, "+
				"and until then nothing here proves which outbound was used:\n  %s",
			authority, strings.Join(mentions, "\n  "))
	}
	wantProxy := "[" + xcfg.TagProxy + "]"
	wantNotDirect := "[" + xcfg.TagDirect + "]"
	for _, line := range decisions {
		if strings.Contains(line, wantNotDirect) {
			return fmt.Errorf("the client routed %s to the %s outbound, not through the tunnel: %s",
				authority, xcfg.TagDirect, line)
		}
		if !strings.Contains(line, wantProxy) {
			return fmt.Errorf("the client's routing decision for %s does not name the %s outbound: %s",
				authority, xcfg.TagProxy, line)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// The tests
// ---------------------------------------------------------------------------

// TestEachProtocolCarriesRealTrafficThroughARealServer is the suite's claim.
//
// Every row runs the appliance's own path from a pasted share link to a running
// engine, drives an HTTP request through it to a server the client cannot reach
// any other way, and checks the answer, where the request was addressed, and
// which outbound the client chose.
func TestEachProtocolCarriesRealTrafficThroughARealServer(t *testing.T) {
	for _, p := range protocolCases() {
		t.Run(p.name, func(t *testing.T) {
			start := time.Now()
			if err := carry(t, p, attempt{startTunnel: true}); err != nil {
				t.Fatalf("%s did not carry traffic:\n%v", p.name, err)
			}
			t.Logf("%s carried real traffic through a real xray-core server in %s",
				p.name, time.Since(start).Round(time.Millisecond))
		})
	}
}

// TestEveryCarriageProofCanFail is the evidence that the test above is worth
// running.
//
// For every protocol it breaks one real thing in the pasted link, or does not
// start the tunnel at all, and requires the proof to go red. A suite where
// every test passes against a deliberately broken client is worthless, and the
// only way to know which kind this is, is to watch it fail on every run rather
// than once, by hand, on the day it was written.
func TestEveryCarriageProofCanFail(t *testing.T) {
	if testing.Short() {
		// Roughly one extra run per defect per protocol, several of which wait
		// out a dial that cannot complete. The smoke subset keeps the passing
		// direction and leaves this to the gate.
		t.Skip("skipped under -short: this runs the whole table once per defect")
	}
	var table []string
	table = append(table, "protocol | defect injected | result")

	for _, p := range protocolCases() {
		for _, d := range defectsFor(p) {
			t.Run(p.name+"/"+d.name, func(t *testing.T) {
				a := attempt{startTunnel: true, timeout: blockedTimeout}
				d.apply(t, p, &a)

				err := carry(t, p, a)
				if err == nil {
					table = append(table, fmt.Sprintf("%s | %s | STILL GREEN", p.name, d.name))
					t.Errorf("the carriage proof passed with %q injected, so it does not test what it says",
						d.name)
					return
				}
				table = append(table, fmt.Sprintf("%s | %s | caught", p.name, d.name))
				t.Logf("caught, as required: %v", err)
			})
		}
	}
	for _, line := range table {
		t.Log(line)
	}
}

// TestEveryProtocolNamesADefect refuses a row nobody has watched fail.
func TestEveryProtocolNamesADefect(t *testing.T) {
	cases := protocolCases()
	if len(cases) == 0 {
		t.Fatal("the protocol table is empty, so every test over it would pass vacuously")
	}
	for _, p := range cases {
		if len(defectsFor(p)) == 0 {
			t.Errorf("%s names no defect, so nobody has seen its carriage proof fail", p.name)
		}
		if p.secret == p.wrongSecret {
			t.Errorf("%s uses the same value for its credential and its wrong credential, "+
				"so the wrong-credential defect changes nothing", p.name)
		}
	}
}

// TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel is the bypass control
// itself, under test.
//
// checkOriginSawTheTunnelledRequest decides whether an arriving request came
// through the tunnel. This runs a request that certainly did not, straight to
// the origin's own address with no proxy, and requires that function to reject
// it. Without this, "the origin saw it arrive from the tunnel" would be a claim
// about a check nobody had exercised in the failing direction.
func TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel(t *testing.T) {
	token := newToken(t, "tunnel-token")
	origin := startEndpoint(t, token)
	sentinel := startEndpoint(t, newToken(t, "bypass-sentinel"))
	decoyAuthority := fmt.Sprintf("%s:%d", originHost, sentinel.port)
	path := "/" + token

	// A direct request to the origin's real address. It is exactly what a test
	// with no bypass control would accept as a pass: the token comes back.
	body, err := directGet("127.0.0.1", origin.port, path)
	if err != nil {
		t.Fatalf("the direct control request did not complete: %v", err)
	}
	if body != token {
		t.Fatalf("premise gone: a direct request to the origin returned %q, not the token", body)
	}

	err = checkOriginSawTheTunnelledRequest(origin, decoyAuthority, path)
	if err == nil {
		t.Fatal("the proof accepted a request that went straight to the origin with no tunnel involved, " +
			"so every carriage test in this package would pass with the tunnel switched off")
	}
	t.Logf("rejected, as required: %v", err)
}

// TestTheOriginIsUnreachableWithoutTheTunnel checks the other half of the same
// control: what the client is told does not lead anywhere useful.
//
// The name is in .invalid, which RFC 6761 section 6.4 reserves so that it never
// resolves, and the port the client is given belongs to the bypass sentinel,
// which serves a different body. Both of those are asserted here rather than
// assumed, because a resolver that answers .invalid would silently weaken every
// other test in the package.
func TestTheOriginIsUnreachableWithoutTheTunnel(t *testing.T) {
	token := newToken(t, "tunnel-token")
	origin := startEndpoint(t, token)
	sentinel := startEndpoint(t, newToken(t, "bypass-sentinel"))
	path := "/" + token

	body, err := directGet(originHost, sentinel.port, path)
	switch {
	case err != nil:
		t.Logf("as required, %s:%d is not reachable directly: %v", originHost, sentinel.port, err)
	case body == token:
		t.Fatalf("a direct request to %s:%d returned this run's origin token, so the tunnel is not "+
			"required to pass any test in this package", originHost, sentinel.port)
	default:
		t.Errorf("%s resolved to a loopback address and the bypass sentinel answered with %q. "+
			"Nothing leaked, but the first bypass control is not holding on this machine",
			originHost, body)
	}

	if got := origin.requests(); len(got) != 0 {
		t.Errorf("the origin served %d request(s) during a test that never used the tunnel: %+v",
			len(got), got)
	}
}

// TestALoopbackDestinationIsRoutedDirectAndTheProofSaysSo is the reason this
// suite addresses the origin by NAME and never by address, turned into a test.
//
// internal/xcfg emits a routing rule sending every private CIDR, 127.0.0.0/8
// included, to the freedom outbound (internal/xcfg/private.go, PrivateRanges,
// and internal/xcfg/build.go, privateRule). That rule is correct and wanted: a
// hotspot client reaching 192.168.x.1 is reaching the box or its neighbours,
// and sending that into somebody else's proxy would be worse than useless. It
// also means that an end-to-end test which pointed its client at a loopback
// ADDRESS would be served by the freedom outbound, get its token back, and pass
// with the tunnel completely uninvolved.
//
// So this test does exactly that, on purpose, and requires two things: that the
// request still succeeds, because the private rule is supposed to work; and
// that checkTheProxyOutboundCarriedIt refuses to call it tunnelled. Between
// them they show that the routing check in the carriage proof is load bearing
// rather than decorative.
func TestALoopbackDestinationIsRoutedDirectAndTheProofSaysSo(t *testing.T) {
	p := protocolCases()[0]

	token := newToken(t, "tunnel-token")
	origin := startEndpoint(t, token)
	cert := makeServerCert(t)
	serverPort := freeLoopbackPort(t)
	socksPort := freeLoopbackPort(t)
	path := "/" + token

	startXrayServer(t, serverConfig(p.inbound(serverPort, cert), origin.port))

	e, err := startClient(t, p.shareLink(serverPort, p.secret, cert.pinHex), socksPort)
	if err != nil {
		t.Fatalf("the client did not come up: %v", err)
	}

	// An IP literal, which is the thing the rest of this package never does.
	body, err := socksGet(fmt.Sprintf("127.0.0.1:%d", socksPort), "127.0.0.1", origin.port, path, carryTimeout)
	if err != nil {
		t.Fatalf("the private-address rule did not carry a loopback request: %v", err)
	}
	if body != token {
		t.Fatalf("the loopback request returned %q, not the origin token", body)
	}

	authority := fmt.Sprintf("127.0.0.1:%d", origin.port)
	err = checkTheProxyOutboundCarriedIt(e, authority)
	if err == nil {
		t.Fatal("the routing check called a request tunnelled when the private-address rule sent it to the " +
			"direct outbound. Every carriage test in this package would then pass on a config that " +
			"never used the tunnel")
	}
	if !strings.Contains(err.Error(), xcfg.TagDirect) {
		t.Fatalf("the routing check complained, but not about the direct outbound, so it may be failing "+
			"for an unrelated reason: %v", err)
	}
	t.Logf("caught, as required: %v", err)

	// And the origin recorded the loopback authority, not the decoy one, which
	// is the same signal checkOriginSawTheTunnelledRequest rests on.
	if seen := origin.requests(); len(seen) != 1 || seen[0].Host != authority {
		t.Errorf("expected exactly one request addressed to %s, got %+v", authority, origin.requests())
	}
}
