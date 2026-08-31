// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// Package xcfg composes the complete engine configuration this appliance runs.
//
// internal/link turns the text a user pasted into one outbound. This package
// produces everything around it: the TUN inbound that client traffic arrives
// on, a loopback SOCKS inbound for diagnostics, the resolver policy, the
// routing rules, and the reject path. Nothing here reads user text; the only
// user-supplied bytes that reach the document are the outbound object link
// already parsed and re-serialised.
//
// Two entry points, and the difference between them is the product's
// fail-closed promise:
//
//   - Build emits the working configuration. The proxy outbound is FIRST.
//   - BuildFailClosed emits a configuration with no proxy outbound at all,
//     where the blackhole is the only outbound. Nothing it can carry leaves.
//
// # Why outbound ORDER is load-bearing and not cosmetic
//
// app/proxyman/outbound/outbound.go:109-110 sets the manager's defaultHandler
// to the FIRST handler added and never changes it, and
// app/dispatcher/default.go:491-492 falls back to that handler for any
// connection no routing rule matched. So the first outbound in the document is
// what carries traffic when the rules are wrong, incomplete, or edited by
// somebody later. A freedom outbound in that position would turn every routing
// mistake into a leak. Both builders therefore put a tunnelling or rejecting
// outbound first, and TestFirstOutboundIsNeverDirect asserts it.
//
// # No geo rules, and why that is a constraint rather than a preference
//
// The engine embeds no geo data. The only go:embed in xray-core v1.260327.0 is
// transport/internet/browser_dialer/dialer.go:18, which embeds an HTML file.
// A "geoip:" prefix in a routing rule reaches ToCidrList at
// infra/conf/router.go:445-458, which calls loadIP("geoip.dat"), which is
// loadFile at router.go:180-192 opening the file through filesystem.OpenAsset,
// located by the "xray.location.asset" environment variable
// (common/platform/platform.go:13). "geosite:" is the same story at
// router.go:373 and dns.go:318.
//
// So one geo rule reintroduces a downloaded data file to a product whose whole
// installer story is one verified binary. This package emits none, and the
// private address ranges are written out as literal CIDRs instead. See
// private.go. TestNoGeoRules asserts the negative on every generated config.
//
// # Two DNS switches, and they are not the same switch
//
// LocalDNS.Enabled adds a loopback DNS listener that a local caller can send
// queries to. It exists because internal/hotspot writes a dnsmasq
// configuration whose only permitted forwarding target is a loopback address
// (internal/hotspot/dnsmasq.go:289, validated at :153-168), and until this
// listener existed nothing in this program was listening there. See
// localdns_doc.go.
//
// DNS.Intercept is a different thing: it hijacks any client traffic bound for
// port 53 and answers it here. That takes a decision away from the client; the
// listener serves a caller that asked for it. Both default to off, both are
// independent, and neither implies the other.
//
// Rule ORDER is what makes either safe. Both DNS rules match on inboundTag
// alone, so neither can match ordinary client traffic, and both sit ABOVE the
// private-address rule so that a query whose destination happens to be private
// cannot be answered on the local network instead of through the tunnel.
// TestLocalDNSQueriesCannotFallOutToTheUplink and TestPrivateRangesRouteDirect
// assert the two halves of that.
//
// # No Google, in any default
//
// docs/2026-08-29-design.md, section 2 and section 6: Google is not used
// anywhere, including as a resolver default. resolvers.go carries the default
// chain and the check that enforces it; TestNoGoogleAnywhereInGeneratedConfigs
// scans every generated document for Google addresses and hostnames.
//
// # This package logs nothing
//
// It has no logger, no fmt.Print and no log import, and its errors name the
// problem without quoting a value: an unusable resolver is reported by its
// position in the list, not by its text. The document being built carries the
// user's key material, so an error path that echoes an input is an error path
// that writes a credential to whatever catches it. internal/engine's Redact
// exists for the errors that come back UP from the engine; this is the same
// discipline applied on the way down.
//
// # Booleans have honest zero values
//
// Every bool in Options is written so that its zero value is the intended
// default: TUN.Disabled false means the TUN inbound is present, SOCKS.UDP
// false means UDP associate is off, DNS.Intercept false means client DNS is
// not hijacked by the engine, and LocalDNS.Enabled false means no loopback
// listener. A caller that fills in nothing gets the intended configuration,
// and normalising defaults cannot silently overrule a caller who meant false.
//
// # A note on the line numbers in this package
//
// Citations into the engine are stable, because the engine is a pinned module
// version. Citations into SIBLING PACKAGES are not: internal/link and
// internal/hotspot both moved by more than a hundred lines while this package
// was being written, and every number pointing into them rotted at least once.
// They are kept current and they are not what holds the contracts together.
// The contracts are held by tests that read the other package:
// TestFixturesStillMatchTheLinkPackage, TestLinkStillStampsTheTagThisPackageExpects
// and TestLocalDNSDefaultMatchesTheHotspotUpstream all fail on a real
// divergence rather than on a moved line.
package xcfg
