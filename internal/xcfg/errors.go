// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import "errors"

// The errors here name the problem and never quote a value.
//
// The document this package builds carries the user's UUID, REALITY public
// key, short id and passwords. internal/link/errors.go already records the
// engine and parser paths that print those verbatim; this package sits between
// the two, so the same rule applies on the way down. Where a list element is
// at fault the error says WHICH element by position, because an index is not
// credential material and "the third resolver" is the whole diagnosis.
var (
	// ErrNoLink means Build was called without a parsed link. Use
	// BuildFailClosed for the no-config case; it is not the same document and
	// silently substituting one for the other would hide a bug behind a
	// working-looking box.
	ErrNoLink = errors.New("no proxy link was supplied")

	// ErrOutboundMissing means internal/link produced a config document with
	// no outbound in it. Not reachable from any Link that Parse returned.
	ErrOutboundMissing = errors.New("the parsed link produced no outbound")

	// ErrOutboundTag means the outbound came back carrying a tag other than
	// the one every routing rule here refers to. Checked rather than assumed,
	// because a rule naming an outbound that does not exist is a silent
	// change of behaviour: app/dispatcher/default.go:481-484 closes the
	// connection instead of falling back, so the symptom would be a tunnel
	// that connects and carries nothing.
	ErrOutboundTag = errors.New("the parsed link produced an outbound with an unexpected tag")

	// ErrSocksAddress means the diagnostics SOCKS inbound was pointed at
	// something that is not a loopback IP literal.
	ErrSocksAddress = errors.New("the diagnostics SOCKS inbound must listen on a loopback IP literal")

	// ErrSocksPort means the diagnostics SOCKS port is zero. The engine does
	// not catch this: a JSON number 0 leaves PortList.Range empty
	// (infra/conf/common.go:243-273) and InboundDetourConfig.Build accepts the
	// empty list (infra/conf/xray.go:154-161), so the inbound builds and
	// listens on nothing.
	ErrSocksPort = errors.New("the diagnostics SOCKS port must not be zero")

	// ErrTunName means the TUN interface name is empty or not a plausible
	// Linux interface name.
	ErrTunName = errors.New("the tunnel interface name is not a usable interface name")

	// ErrTunMTU means the MTU is outside the range this appliance will emit.
	// infra/conf/tun.go:14-30 validates nothing, so an absurd value would be
	// accepted and produce an interface that cannot carry a full-size segment.
	ErrTunMTU = errors.New("the tunnel MTU is outside the range this appliance will configure")

	// ErrNoResolvers means the resolver list is empty. An empty "servers"
	// list builds: DNSConfig.Build at infra/conf/dns.go:486-492 iterates
	// c.Servers with no minimum-length check, leaving the DNS app with no client.
	ErrNoResolvers = errors.New("the resolver list is empty")

	// ErrResolverNotIP means a resolver entry is not a bare IP literal. Only
	// literals are accepted so that the no-Google rule is a set-membership
	// test on the document rather than a question about what a hostname
	// resolves to today.
	ErrResolverNotIP = errors.New("a resolver is not a plain IP address")

	// ErrGoogleResolver means a resolver is a Google Public DNS address. See
	// resolvers.go for the decision and the addresses.
	ErrGoogleResolver = errors.New("a resolver is a Google address, which this product does not use")

	// ErrQueryStrategy means the DNS query strategy is not one of the three
	// this package emits. It has to be checked here: resolveQueryStrategy at
	// infra/conf/dns.go:515-528 has a default branch that silently returns
	// USE_IP for anything it does not recognise, so a typo would quietly
	// change which address families the resolver answers with.
	ErrQueryStrategy = errors.New("the DNS query strategy is not one this appliance supports")

	// ErrLogLevel means the log level is not one of the four the engine maps
	// to a severity. Same shape of trap: LogConfig.Build at
	// infra/conf/log.go:49-62 sends every unrecognised value to the default
	// branch and silently applies Warning.
	ErrLogLevel = errors.New("the log level is not one this appliance supports")

	// errSerialise covers the encoding/json failures in Build. Everything
	// being marshalled is either a literal from this package or bytes that
	// came back out of encoding/json moments earlier, so it is not reachable
	// from any input and is deliberately not an exported sentinel.
	errSerialise = errors.New("the configuration could not be serialised")
)

// Errors for the loopback DNS listener. Added with LocalDNS; see localdns_doc.go.
var (
	// ErrLocalDNSAddress means the loopback DNS listener was pointed at
	// something that is not a loopback IP literal. Same rule as
	// ErrSocksAddress and a sharper consequence: a DNS listener on a
	// non-loopback address is an open resolver on whatever network the box is
	// plugged into.
	ErrLocalDNSAddress = errors.New("the local DNS listener must listen on a loopback IP literal")

	// ErrLocalDNSPort means the loopback DNS port is zero. Same engine
	// behaviour as ErrSocksPort: the inbound builds and listens on nothing.
	ErrLocalDNSPort = errors.New("the local DNS port must not be zero")

	// ErrInboundCollision means two inbounds were told to bind the same
	// address and port.
	//
	// This has to be caught here. engine.Validate stops after Build and opens
	// no socket (internal/engine/engine.go:300-322), so a collision is not a
	// config error to the engine at all: it surfaces at Start as a bind
	// failure, by which point the panel has already reported the config as
	// accepted and the user is looking at a box that will not come up.
	ErrInboundCollision = errors.New("two inbounds were asked to bind the same address and port")
)
