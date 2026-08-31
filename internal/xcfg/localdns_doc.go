// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

// LocalDNS configures a loopback DNS listener that a local caller can send
// queries to, and whose answers are resolved through the tunnel.
//
// # The hole this closes
//
// internal/hotspot writes a dnsmasq configuration whose ONLY permitted
// forwarding target is a loopback address: internal/hotspot/dnsmasq.go:289
// emits "server=<addr>#<port>" from Config.Upstream, and Validate at :132-146
// refuses any Upstream that is not loopback, with the comment that a
// non-loopback target would be "a query leaving the box outside the tunnel,
// for every name every client asks for".
//
// The field's own doc at dnsmasq.go:75-78 calls it "the local resolver this
// program runs, the one that sends queries through the tunnel". Until this
// type existed, this program ran no such resolver. dnsmasq forwarded to a port
// nothing was listening on, so the chain was: client asks dnsmasq, dnsmasq
// asks nothing, resolution fails.
//
// # It is NOT DNS.Intercept, and the difference is the point
//
// DNS.Intercept hijacks traffic that was going somewhere else: it matches
// destination port 53 from any client and redirects it. This answers a caller
// that deliberately asked this address, because the operator configured it to.
// One takes a decision away from the client; the other serves a local
// component of this same program. They are separate switches with separate
// defaults and they compose, so turning this on does not reopen the question
// of who owns client DNS.
//
// # Where the port comes from, and a coordination gap worth naming
//
// Neither package owns this value as a constant. internal/hotspot takes
// Config.Upstream from its caller and defines no default; 127.0.0.1:5354
// appears only in internal/hotspot/dnsmasq_test.go:25 and in the generated
// internal/hotspot/testdata/dnsmasq.golden:86. So the two halves of this chain
// currently agree by way of a test fixture rather than by a shared constant,
// which is precisely the kind of agreement that breaks silently.
//
// DefaultLocalDNSPort below adopts that value, and
// TestLocalDNSDefaultMatchesTheHotspotUpstream reads the hotspot golden and
// fails if the two ever diverge. That converts an unowned coincidence into a
// red test. It does not decide WHICH package should own the constant; that is
// a maintainer's call and is flagged rather than quietly settled here.
type LocalDNS struct {
	// Enabled adds the listener and its routing rule.
	//
	// Off by default, so the zero value is the current shipped behaviour and
	// turning it on is a deliberate act. It has to be turned on for the
	// hotspot's dnsmasq to resolve anything.
	Enabled bool

	// Listen is the address to bind. Empty means DefaultLocalDNSListen.
	//
	// Loopback IP literal only, same rule and same reason as SOCKS.Listen:
	// this listener answers DNS for anyone who can reach it, and on a
	// non-loopback address that is an open resolver on whatever network the
	// box is plugged into. internal/hotspot enforces the mirror image of this
	// on its side (dnsmasq.go:162-168).
	Listen string

	// Port is the port to bind. Zero means DefaultLocalDNSPort.
	Port uint16
}

// DefaultLocalDNSListen and DefaultLocalDNSPort are what internal/hotspot's
// dnsmasq configuration currently forwards to. See the type doc above for why
// this is adopted from a test fixture and what guards it.
const (
	DefaultLocalDNSListen = "127.0.0.1"
	DefaultLocalDNSPort   = 5354
)
