// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/engine"
)

// parsed is the decoded shape of a generated document, used by the tests that
// assert on structure rather than on bytes.
type parsed struct {
	Log struct {
		LogLevel string `json:"loglevel"`
		Access   string `json:"access"`
		DNSLog   bool   `json:"dnsLog"`
	} `json:"log"`
	DNS struct {
		Servers       []string `json:"servers"`
		QueryStrategy string   `json:"queryStrategy"`
		Tag           string   `json:"tag"`
	} `json:"dns"`
	Inbounds []struct {
		Tag      string          `json:"tag"`
		Protocol string          `json:"protocol"`
		Listen   string          `json:"listen"`
		Port     uint16          `json:"port"`
		Settings json.RawMessage `json:"settings"`
	} `json:"inbounds"`
	Outbounds []struct {
		Tag      string          `json:"tag"`
		Protocol string          `json:"protocol"`
		Settings json.RawMessage `json:"settings"`
	} `json:"outbounds"`
	Routing struct {
		DomainStrategy string `json:"domainStrategy"`
		Rules          []struct {
			RuleTag     string   `json:"ruleTag"`
			InboundTag  []string `json:"inboundTag"`
			IP          []string `json:"ip"`
			Port        string   `json:"port"`
			Network     string   `json:"network"`
			OutboundTag string   `json:"outboundTag"`
		} `json:"rules"`
	} `json:"routing"`
}

func decode(t *testing.T, raw []byte) parsed {
	t.Helper()
	var p parsed
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("generated config is not valid JSON: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// The option space, enumerated once and reused by every sweeping test.
// ---------------------------------------------------------------------------

// axis is one named setting of one option, applied to an Options value.
type axis struct {
	name  string
	apply func(*Options)
}

// axes returns the option space this package supports, one slice per
// independent dimension.
//
// Every dimension here is a real, reachable setting, and every combination of
// them is generated and validated by TestEveryOptionCombinationValidates. When
// a new option is added to Options it must be added here too, or the sweep
// silently stops covering it. TestAxesCoverEveryOptionsField is the guard on
// that: it fails when Options grows a field this file does not name.
func axes() [][]axis {
	return [][]axis{
		{
			{"tun-on", func(o *Options) { o.TUN.Disabled = false }},
			{"tun-off", func(o *Options) { o.TUN.Disabled = true }},
		},
		{
			{"tun-default", func(o *Options) {}},
			{"tun-custom", func(o *Options) { o.TUN.Name = "csp0"; o.TUN.MTU = 1420; o.TUN.UserLevel = 1 }},
			{"tun-mtu-min", func(o *Options) { o.TUN.MTU = MinTunMTU }},
			{"tun-mtu-max", func(o *Options) { o.TUN.MTU = MaxTunMTU }},
		},
		{
			// Left EMPTY so the sweep exercises the DEFAULT bind address.
			// Without this entry every generated document carries an address
			// an axis put there, and a wildcard default would go unnoticed by
			// TestSocksNeverBindsAWildcard/generated. Found by mutating
			// DefaultSocksListen to 0.0.0.0 and watching that subtest stay
			// green.
			{"socks-default", func(o *Options) { o.SOCKS.Listen = "" }},
			{"socks-v4-alt", func(o *Options) { o.SOCKS.Listen = "127.0.0.53"; o.SOCKS.Port = 19999 }},
			{"socks-v6", func(o *Options) { o.SOCKS.Listen = "::1" }},
		},
		{
			{"socks-tcp", func(o *Options) { o.SOCKS.UDP = false }},
			{"socks-udp", func(o *Options) { o.SOCKS.UDP = true }},
		},
		{
			{"dns-default", func(o *Options) {}},
			{"dns-custom", func(o *Options) { o.DNS.Servers = []string{"9.9.9.11", "2620:fe::fe"} }},
		},
		{
			{"q-useip", func(o *Options) { o.DNS.Strategy = QueryUseIP }},
			{"q-v4", func(o *Options) { o.DNS.Strategy = QueryUseIPv4 }},
			{"q-v6", func(o *Options) { o.DNS.Strategy = QueryUseIPv6 }},
		},
		{
			{"no-intercept", func(o *Options) { o.DNS.Intercept = false }},
			{"intercept", func(o *Options) { o.DNS.Intercept = true }},
		},
		{
			{"localdns-off", func(o *Options) { o.LocalDNS.Enabled = false }},
			// Enabled with Listen and Port left EMPTY, so the sweep exercises
			// both LocalDNS defaults rather than only values an axis set. This
			// is the same blind spot that let a wildcard SOCKS default pass
			// unnoticed until it was found by mutation; the listener gets the
			// same treatment from the start.
			{"localdns-default", func(o *Options) { o.LocalDNS.Enabled = true }},
			{"localdns-custom", func(o *Options) {
				o.LocalDNS.Enabled = true
				o.LocalDNS.Listen = "::1"
				o.LocalDNS.Port = 15353
			}},
		},
		{
			{"log-debug", func(o *Options) { o.LogLevel = LogDebug }},
			{"log-info", func(o *Options) { o.LogLevel = LogInfo }},
			{"log-warning", func(o *Options) { o.LogLevel = LogWarning }},
			{"log-error", func(o *Options) { o.LogLevel = LogError }},
		},
	}
}

// combination is one point in the option space.
type combination struct {
	name string
	opts Options
}

// combinations walks the full cartesian product of axes(). It is a product,
// not a sample: the brief is that every combination validates, and a sample
// cannot show that.
func combinations() []combination {
	all := axes()
	out := []combination{{name: "", opts: Options{}}}
	for _, dimension := range all {
		next := make([]combination, 0, len(out)*len(dimension))
		for _, sofar := range out {
			for _, a := range dimension {
				o := sofar.opts
				// Copy the resolver slice so two combinations sharing a
				// prefix cannot alias each other's list.
				o.DNS.Servers = append([]string(nil), o.DNS.Servers...)
				a.apply(&o)
				name := sofar.name
				if name != "" {
					name += "/"
				}
				next = append(next, combination{name: name + a.name, opts: o})
			}
		}
		out = next
	}
	return out
}

// TestAxesCoverEveryOptionsField fails when Options grows a field that the
// sweep above does not vary.
//
// Without it, "every option combination validates" quietly becomes "every
// combination of the options somebody remembered to list", which is the shape
// of test that reports success while covering less than it claims.
func TestAxesCoverEveryOptionsField(t *testing.T) {
	want := trackedOptionFields()

	// Collect the field paths the axes actually touch, by applying each axis
	// to a zero Options and diffing against a second zero Options.
	touched := map[string]bool{}
	for _, dimension := range axes() {
		for _, a := range dimension {
			var got, zero Options
			a.apply(&got)
			for _, path := range want {
				if fieldString(got, path) != fieldString(zero, path) {
					touched[path] = true
				}
			}
		}
	}
	for _, path := range want {
		if !touched[path] {
			t.Errorf("no axis in axes() varies Options.%s, so the combination sweep does not cover it", path)
		}
	}
}

// trackedOptionFields is every leaf field of Options, spelled as a path. Link
// is excluded because it is varied by the fixture loop rather than by an axis.
//
// Two guards consume it: TestAxesCoverEveryOptionsField, which checks the
// combination sweep varies each one, and
// TestEveryOptionsFieldIsOverriddenInTheGolden, which checks the
// everything-overridden golden sets each one. Sharing the list means adding a
// field here turns both red at once rather than one of them.
func trackedOptionFields() []string {
	return []string{
		"LogLevel",
		"TUN.Disabled", "TUN.Name", "TUN.MTU", "TUN.UserLevel",
		"SOCKS.Listen", "SOCKS.Port", "SOCKS.UDP",
		"DNS.Servers", "DNS.Strategy", "DNS.Intercept",
		"LocalDNS.Enabled", "LocalDNS.Listen", "LocalDNS.Port",
	}
}

// fieldString renders one Options field as a string, for the coverage check
// above. Reflection is avoided on purpose: an explicit switch fails to compile
// when a field is renamed, which is a better failure than a silently missing
// path at run time.
func fieldString(o Options, path string) string {
	switch path {
	case "LogLevel":
		return string(o.LogLevel)
	case "TUN.Disabled":
		return fmt.Sprint(o.TUN.Disabled)
	case "TUN.Name":
		return o.TUN.Name
	case "TUN.MTU":
		return fmt.Sprint(o.TUN.MTU)
	case "TUN.UserLevel":
		return fmt.Sprint(o.TUN.UserLevel)
	case "SOCKS.Listen":
		return o.SOCKS.Listen
	case "SOCKS.Port":
		return fmt.Sprint(o.SOCKS.Port)
	case "SOCKS.UDP":
		return fmt.Sprint(o.SOCKS.UDP)
	case "DNS.Servers":
		return strings.Join(o.DNS.Servers, ",")
	case "DNS.Strategy":
		return string(o.DNS.Strategy)
	case "DNS.Intercept":
		return fmt.Sprint(o.DNS.Intercept)
	case "LocalDNS.Enabled":
		return fmt.Sprint(o.LocalDNS.Enabled)
	case "LocalDNS.Listen":
		return o.LocalDNS.Listen
	case "LocalDNS.Port":
		return fmt.Sprint(o.LocalDNS.Port)
	default:
		panic("unknown Options path in test: " + path)
	}
}

// ---------------------------------------------------------------------------
// The sweep.
// ---------------------------------------------------------------------------

// TestEveryOptionCombinationValidates runs the engine's real config loader
// over every document this package can produce.
//
// engine.Validate calls infra/conf/serial.LoadJSONConfig, which decodes the
// JSON into a conf.Config and calls Build. That is the same code path a start
// takes, minus core.New; it opens no socket and dials nothing. See
// internal/engine/engine.go:300-322.
//
// This is a schema check and not a completeness check, and internal/engine
// says so in its own words. It will not catch a misspelled key, because the
// loader sets no DisallowUnknownFields. What it does catch, and what nothing
// else here would, is a value this package emits that the engine refuses:
// every combination is put through the engine's own parser rather than
// through a reimplementation of it.
func TestEveryOptionCombinationValidates(t *testing.T) {
	combos := combinations()
	if len(combos) == 0 {
		t.Fatal("the combination sweep is empty; it would pass vacuously")
	}

	checked := 0
	for _, f := range fixtures() {
		l := mustParse(t, f.raw())
		for _, c := range combos {
			o := c.opts
			o.Link = l
			raw, err := Build(o)
			if err != nil {
				t.Fatalf("%s/%s: Build: %v", f.name, c.name, err)
			}
			if err := engine.Validate(raw); err != nil {
				t.Fatalf("%s/%s: engine.Validate rejected the generated config: %v", f.name, c.name, err)
			}
			checked++
		}
	}

	// The fail-closed document does not depend on a link, so it is swept once
	// over the same option space rather than once per fixture.
	for _, c := range combos {
		raw, err := BuildFailClosed(c.opts)
		if err != nil {
			t.Fatalf("fail-closed/%s: BuildFailClosed: %v", c.name, err)
		}
		if err := engine.Validate(raw); err != nil {
			t.Fatalf("fail-closed/%s: engine.Validate rejected the generated config: %v", c.name, err)
		}
		checked++
	}

	t.Logf("engine.Validate accepted %d generated configs (%d combinations x %d links, plus %d fail-closed)",
		checked, len(combos), len(fixtures()), len(combos))
}

// ---------------------------------------------------------------------------
// The named properties.
// ---------------------------------------------------------------------------

// generated yields every document the property tests below run over: every
// fixture against every combination, plus the fail-closed documents.
func generated(t *testing.T, fn func(name string, raw []byte)) {
	t.Helper()
	combos := combinations()
	for _, f := range fixtures() {
		l := mustParse(t, f.raw())
		for _, c := range combos {
			o := c.opts
			o.Link = l
			raw, err := Build(o)
			if err != nil {
				t.Fatalf("%s/%s: Build: %v", f.name, c.name, err)
			}
			fn(f.name+"/"+c.name, raw)
		}
	}
	for _, c := range combos {
		raw, err := BuildFailClosed(c.opts)
		if err != nil {
			t.Fatalf("fail-closed/%s: BuildFailClosed: %v", c.name, err)
		}
		fn("fail-closed/"+c.name, raw)
	}
}

// TestNoGoogleAnywhereInGeneratedConfigs is the enforcement of
// docs/2026-08-29-design.md section 2 and section 6: Google is not used
// anywhere, including as a resolver default.
//
// It scans the generated BYTES rather than the resolver list, so it catches a
// Google address that arrives by any route: a default, an override, a hosts
// entry, a routing rule, or a value that came in through the pasted link.
func TestNoGoogleAnywhereInGeneratedConfigs(t *testing.T) {
	// Assembled from fragments so this test file does not fail its own check
	// when something greps the tree for these strings.
	needles := []string{
		"8.8." + "8.8",
		"8.8." + "4.4",
		"8.8.8." + "0/24",
		"8.8.4." + "0/24",
		"2001:4860:" + "4860",
		"dns." + "google",
		"google" + "apis.com",
		"google" + ".com",
	}

	count := 0
	generated(t, func(name string, raw []byte) {
		s := strings.ToLower(string(raw))
		for _, n := range needles {
			if strings.Contains(s, strings.ToLower(n)) {
				t.Errorf("%s: generated config contains the Google marker %q", name, n)
			}
		}
		count++
	})
	if count == 0 {
		t.Fatal("no configs were generated; the scan would pass vacuously")
	}
	t.Logf("scanned %d generated configs for Google addresses and hostnames", count)
}

// TestGoogleResolverIsRejectedAtTheSource proves the scan above has something
// to catch: the code refuses a Google resolver rather than relying on nobody
// ever configuring one.
//
// Without this, the scan is a test of the DEFAULTS only, and an operator
// override would walk straight past it.
func TestGoogleResolverIsRejectedAtTheSource(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	cases := []struct {
		name    string
		servers []string
	}{
		{"v4-primary", []string{"8.8." + "8.8"}},
		{"v4-secondary", []string{"9.9.9.9", "8.8." + "4.4"}},
		{"v4-neighbour", []string{"8.8.8." + "44"}},
		{"v6", []string{"2001:4860:" + "4860::8888"}},
		{"v6-dns64", []string{"2001:4860:" + "4860::64"}},
		{"v4-mapped-v6", []string{"::ffff:8.8." + "8.8"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := Options{Link: l, DNS: DNS{Servers: c.servers}}
			if _, err := Build(o); !errors.Is(err, ErrGoogleResolver) {
				t.Fatalf("Build accepted a Google resolver, or failed for the wrong reason: %v", err)
			}
			if _, err := BuildFailClosed(o); !errors.Is(err, ErrGoogleResolver) {
				t.Fatalf("BuildFailClosed accepted a Google resolver, or failed for the wrong reason: %v", err)
			}
		})
	}
}

// TestPrivateRangesRouteDirect asserts requirement five: private address space
// stays off the tunnel.
//
// It checks three things together, because any one of them alone can be true
// while the rule does nothing: the rule exists and names the direct outbound,
// it carries every range PrivateRanges lists, and precedes the catch-all.
// DNS alone is intercepted even when its destination is private.
func TestPrivateRangesRouteDirect(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	for _, c := range combinations() {
		o := c.opts
		o.Link = l
		raw, err := Build(o)
		if err != nil {
			t.Fatalf("%s: Build: %v", c.name, err)
		}
		p := decode(t, raw)

		if len(p.Routing.Rules) == 0 {
			t.Fatalf("%s: no routing rules at all", c.name)
		}
		// Internal DNS tags and the narrow TCP/UDP port 53 interception may
		// precede local access. No broader client-traffic rule may do so.
		idx := -1
		for i, r := range p.Routing.Rules {
			if r.RuleTag == ruleTagPrivate {
				idx = i
				break
			}
		}
		if idx < 0 {
			t.Fatalf("%s: no private-direct rule at all", c.name)
		}
		for i, above := range p.Routing.Rules[:idx] {
			if above.RuleTag == ruleTagDNS {
				if above.Port != "53" || above.Network != "tcp,udp" || above.OutboundTag != TagDNSOut || len(above.IP) != 0 || len(above.InboundTag) != 0 {
					t.Fatal("DNS exception must match only TCP/UDP port 53 and use Xray DNS")
				}
				continue
			}
			if len(above.InboundTag) == 0 {
				t.Errorf("%s: rule %d (%q) sits above the private rule but does not match on inboundTag, so it can divert client traffic",
					c.name, i, above.RuleTag)
			}
			if len(above.IP) != 0 || above.Port != "" || above.Network != "" {
				t.Errorf("%s: rule %d (%q) sits above the private rule and carries an ip/port/network condition",
					c.name, i, above.RuleTag)
			}
		}
		first := p.Routing.Rules[idx]
		if first.OutboundTag != TagDirect {
			t.Errorf("%s: private rule sends traffic to %q, want %q", c.name, first.OutboundTag, TagDirect)
		}
		have := map[string]bool{}
		for _, ip := range first.IP {
			have[ip] = true
		}
		for _, want := range PrivateRanges() {
			if !have[want] {
				t.Errorf("%s: private rule is missing %s", c.name, want)
			}
		}
		if len(first.IP) != len(PrivateRanges()) {
			t.Errorf("%s: private rule carries %d ranges, PrivateRanges has %d",
				c.name, len(first.IP), len(PrivateRanges()))
		}
		// The named RFC 1918 ranges, spelled out here rather than taken from
		// PrivateRanges, so that deleting one from that function fails this
		// test instead of quietly agreeing with it.
		for _, want := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "169.254.0.0/16"} {
			if !have[want] {
				t.Errorf("%s: private rule is missing %s", c.name, want)
			}
		}
	}
}

// TestSocksNeverBindsAWildcard asserts "never on a wildcard address" from both
// directions, for EVERY inbound this package binds: every generated document
// binds a loopback literal on a non-zero port with no two inbounds colliding,
// and every wildcard-shaped input is refused.
func TestSocksNeverBindsAWildcard(t *testing.T) {
	t.Run("generated", func(t *testing.T) {
		count := 0
		generated(t, func(name string, raw []byte) {
			p := decode(t, raw)
			foundSocks := false
			// EVERY inbound that binds an address, not just the socks one.
			// Written this way so a listener added later is covered the day it
			// is added rather than the day somebody remembers to extend this
			// test. The TUN inbound has no listen field and is skipped by the
			// same rule that covers the others.
			seen := map[string]bool{}
			for _, in := range p.Inbounds {
				if in.Protocol == "socks" {
					foundSocks = true
				}
				if in.Listen == "" {
					if in.Protocol != "tun" {
						t.Errorf("%s: inbound %q (%s) has no listen address; only the tun inbound may omit one",
							name, in.Tag, in.Protocol)
					}
					continue
				}
				// The PROPERTY, not a list of the literals the axes happen to
				// set. Anything that is not a loopback IP is a wildcard or an
				// interface address, including the forms nobody enumerated.
				addr, err := netip.ParseAddr(in.Listen)
				switch {
				case err != nil:
					t.Errorf("%s: inbound %q listens on %q, which is not an IP literal", name, in.Tag, in.Listen)
				case !addr.Unmap().IsLoopback():
					t.Errorf("%s: inbound %q listens on %q, which is not a loopback address", name, in.Tag, in.Listen)
				}
				if in.Port == 0 {
					t.Errorf("%s: inbound %q has port 0, which the engine accepts and which listens on nothing", name, in.Tag)
				}
				// A collision would build cleanly and fail at Start with a bind
				// error, long after the panel reported the config accepted.
				key := fmt.Sprintf("%s/%d", addr.Unmap(), in.Port)
				if seen[key] {
					t.Errorf("%s: two inbounds bind the same address and port (%s)", name, key)
				}
				seen[key] = true
			}
			if !foundSocks {
				t.Errorf("%s: no socks inbound in the generated config", name)
			}
			count++
		})
		if count == 0 {
			t.Fatal("no configs were generated; the check would pass vacuously")
		}
	})

	t.Run("rejected", func(t *testing.T) {
		l := mustParse(t, vlessRealityLink())
		// Every one of these is a way somebody reaches a wildcard or a
		// non-loopback bind without typing "0.0.0.0".
		bad := []string{
			"0.0.0.0",
			"::",
			"::ffff:0.0.0.0",
			"0000:0000:0000:0000:0000:0000:0000:0000",
			"localhost",     // infra/conf/xray.go:153 accepts this as an address
			"192.168.1.10",  // a real interface address
			"0.0.0.0.0",     // typo
			"",              // handled by normalise, listed so a change there is caught
			"127.0.0.1:100", // host:port pasted into an address field
		}
		for _, listen := range bad {
			o := Options{Link: l, SOCKS: SOCKS{Listen: listen}}
			_, err := Build(o)
			if listen == "" {
				// normalise fills the default in, so this one must SUCCEED.
				if err != nil {
					t.Errorf("empty listen should normalise to the loopback default, got %v", err)
				}
				continue
			}
			if !errors.Is(err, ErrSocksAddress) {
				t.Errorf("Build accepted listen %q, or failed for the wrong reason: %v", listen, err)
			}
		}
		if _, err := Build(Options{Link: l, SOCKS: SOCKS{Listen: "0.0.0.0"}}); err == nil {
			t.Error("Build accepted a wildcard SOCKS bind")
		}
	})
}

// TestNoGeoRules asserts the constraint that makes the private CIDR list
// necessary: no geoip: or geosite: value anywhere.
//
// A geo rule sends the engine to a .dat file on disk located by
// xray.location.asset (infra/conf/router.go:445-458 and :180-192), which would
// reintroduce a downloaded artefact this product removed. See private.go.
func TestNoGeoRules(t *testing.T) {
	count := 0
	generated(t, func(name string, raw []byte) {
		s := string(raw)
		for _, needle := range []string{"geoip:", "geosite:", "ext:", "ext-ip:", "ext-domain:"} {
			if strings.Contains(s, needle) {
				t.Errorf("%s: generated config contains %q, which makes the engine read a data file from disk", name, needle)
			}
		}
		count++
	})
	if count == 0 {
		t.Fatal("no configs were generated; the scan would pass vacuously")
	}
}

// TestFirstOutboundIsNeverDirect guards the property the package doc calls
// load-bearing.
//
// app/proxyman/outbound/outbound.go:109-110 makes the first outbound the
// manager's defaultHandler, and app/dispatcher/default.go:491-492 sends every
// unmatched connection to it. A freedom outbound in that slot turns any
// routing mistake into a leak, so the position is asserted rather than
// remembered.
func TestFirstOutboundIsNeverDirect(t *testing.T) {
	count := 0
	generated(t, func(name string, raw []byte) {
		p := decode(t, raw)
		if len(p.Outbounds) == 0 {
			t.Fatalf("%s: no outbounds", name)
		}
		first := p.Outbounds[0]
		if first.Protocol == "freedom" || first.Tag == TagDirect {
			t.Errorf("%s: the first outbound is %q/%q, so unmatched traffic would leave untunnelled",
				name, first.Tag, first.Protocol)
		}
		count++
	})
	if count == 0 {
		t.Fatal("no configs were generated; the check would pass vacuously")
	}
}

// TestFailClosedCarriesNoWayOut is the fail-closed property stated as
// something checkable: the document has exactly one outbound, it is the
// blackhole, and there is no freedom or proxy outbound to reach even if every
// rule were deleted.
func TestFailClosedCarriesNoWayOut(t *testing.T) {
	for _, c := range combinations() {
		raw, err := BuildFailClosed(c.opts)
		if err != nil {
			t.Fatalf("%s: BuildFailClosed: %v", c.name, err)
		}
		p := decode(t, raw)
		if len(p.Outbounds) != 1 {
			t.Fatalf("%s: fail-closed config has %d outbounds, want 1", c.name, len(p.Outbounds))
		}
		if p.Outbounds[0].Protocol != "blackhole" || p.Outbounds[0].Tag != TagBlock {
			t.Errorf("%s: the only outbound is %q/%q, want %q/blackhole",
				c.name, p.Outbounds[0].Tag, p.Outbounds[0].Protocol, TagBlock)
		}
		for _, r := range p.Routing.Rules {
			if r.OutboundTag != TagBlock {
				t.Errorf("%s: rule %q points at %q in a fail-closed config", c.name, r.RuleTag, r.OutboundTag)
			}
		}
		if !strings.Contains(string(raw), `"protocol": "blackhole"`) {
			t.Errorf("%s: no blackhole outbound in the emitted bytes", c.name)
		}
		for _, protocol := range []string{"freedom", "vless", "vmess", "trojan", "shadowsocks", "hysteria"} {
			if strings.Contains(string(raw), `"protocol": "`+protocol+`"`) {
				t.Errorf("%s: fail-closed config carries a %s outbound", c.name, protocol)
			}
		}
	}
}

// TestEveryRuleNamesAnOutboundThatExists catches the failure that does not
// look like one.
//
// app/dispatcher/default.go:481-484 does NOT fall back when a rule names a
// missing outbound; it closes the connection, with a comment saying so. The
// symptom is a tunnel that connects and carries nothing, which is the hardest
// state to diagnose from a panel.
func TestEveryRuleNamesAnOutboundThatExists(t *testing.T) {
	generated(t, func(name string, raw []byte) {
		p := decode(t, raw)
		have := map[string]bool{}
		for _, ob := range p.Outbounds {
			have[ob.Tag] = true
		}
		for _, r := range p.Routing.Rules {
			if !have[r.OutboundTag] {
				t.Errorf("%s: rule %q names outbound %q, which is not in the config", name, r.RuleTag, r.OutboundTag)
			}
		}
	})
}

// TestResolverQueriesArePinnedToTheTunnel asserts that the DNS app's own
// upstream traffic goes to the proxy, and that the rule doing it comes before
// the intercept rule.
//
// The order is the whole substance. app/dns/nameserver.go:274 stamps the DNS
// app's outbound queries with dns.tag. If the port 53 rule matched first,
// those queries would be handed to the dns outbound that issued them.
func TestResolverQueriesArePinnedToTheTunnel(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	for _, intercept := range []bool{false, true} {
		o := Options{Link: l, DNS: DNS{Intercept: intercept}}
		raw, err := Build(o)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		p := decode(t, raw)

		if p.DNS.Tag != TagResolverIn {
			t.Fatalf("dns.tag is %q, want %q; an empty tag makes app/dns generate a random one that no rule can match",
				p.DNS.Tag, TagResolverIn)
		}

		resolverIdx, dnsIdx, privateIdx := -1, -1, -1
		for i, r := range p.Routing.Rules {
			if r.OutboundTag == TagDirect {
				privateIdx = i
			}
			switch r.RuleTag {
			case ruleTagResolvers:
				resolverIdx = i
				if len(r.InboundTag) != 1 || r.InboundTag[0] != TagResolverIn {
					t.Errorf("resolver rule matches inboundTag %v, want [%s]", r.InboundTag, TagResolverIn)
				}
				if r.OutboundTag != TagProxy {
					t.Errorf("resolver rule sends queries to %q, want %q", r.OutboundTag, TagProxy)
				}
			case ruleTagDNS:
				dnsIdx = i
			}
		}
		if resolverIdx < 0 {
			t.Fatal("no resolver-through-tunnel rule")
		}
		if intercept {
			if privateIdx < 0 || dnsIdx >= privateIdx {
				t.Fatal("DNS interception must precede the private-network bypass")
			}
			if dnsIdx < 0 {
				t.Fatal("Intercept is set but there is no intercept rule")
			}
			if resolverIdx > dnsIdx {
				t.Errorf("the resolver rule is at %d and the intercept rule at %d; that order makes the DNS app answer its own upstream queries",
					resolverIdx, dnsIdx)
			}
		} else if dnsIdx >= 0 {
			t.Error("Intercept is unset but an intercept rule was emitted")
		}
	}
}

// TestDNSAppIsUnreachableWithoutIntercept holds the package doc to its word.
//
// "Nothing consults the DNS app" is a claim about every path in, so all four
// are checked: the router's domainStrategy, the freedom outbound's
// domainStrategy, the presence of a dns outbound, and the presence of a
// sniffing section that could enable fakedns.
//
// The precondition is that BOTH DNS.Intercept and LocalDNS.Enabled are unset,
// which is what the zero Options below gives. Either one on its own adds the
// dns outbound and makes the claim false, which is correct behaviour and is
// covered by TestLocalDNSQueriesCannotFallOutToTheUplink instead.
func TestDNSAppIsUnreachableWithoutIntercept(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	raw, err := Build(Options{Link: l})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	p := decode(t, raw)

	if p.Routing.DomainStrategy != "AsIs" {
		t.Errorf("routing.domainStrategy is %q; anything but AsIs sends the router to the DNS client (app/router/router.go:253,:263)",
			p.Routing.DomainStrategy)
	}
	for _, ob := range p.Outbounds {
		if ob.Protocol == "dns" {
			t.Error("a dns outbound was emitted with Intercept unset")
		}
		if ob.Protocol != "freedom" {
			continue
		}
		var s struct {
			DomainStrategy string `json:"domainStrategy"`
		}
		if err := json.Unmarshal(ob.Settings, &s); err != nil {
			t.Fatalf("freedom settings: %v", err)
		}
		if s.DomainStrategy != "AsIs" {
			t.Errorf("the direct outbound's domainStrategy is %q; anything but AsIs makes freedom resolve through the DNS app (infra/conf/freedom.go:49-51)",
				s.DomainStrategy)
		}
	}
	if strings.Contains(string(raw), "sniffing") || strings.Contains(string(raw), "fakedns") {
		t.Error("a sniffing or fakedns section was emitted; either can reach the DNS app")
	}
}

// TestErrorsNameNoValue is the log-redaction property applied to this
// package's own errors: an error may say which field or which position is
// wrong, and never what was in it.
func TestErrorsNameNoValue(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	bad := []Options{
		{Link: l, SOCKS: SOCKS{Listen: "0.0.0.0"}},
		{Link: l, SOCKS: SOCKS{Listen: fakePassword}},
		{Link: l, DNS: DNS{Servers: []string{fakeUUID}}},
		{Link: l, DNS: DNS{Servers: []string{fakePublicKey()}}},
		{Link: l, DNS: DNS{Servers: []string{fakeMldsa65Verify()}}},
		{Link: l, DNS: DNS{Servers: []string{"8.8." + "8.8"}}},
		{Link: l, DNS: DNS{Strategy: QueryStrategy(fakeShortID)}},
		{Link: l, LogLevel: LogLevel(fakeAuth)},
		{Link: l, TUN: TUN{Name: fakePassword}},
		{Link: l, TUN: TUN{MTU: 1}},
	}
	for i, o := range bad {
		_, err := Build(o)
		if err == nil {
			t.Errorf("case %d: Build accepted an invalid Options", i)
			continue
		}
		msg := err.Error()
		for _, secret := range secretsIn() {
			if strings.Contains(msg, secret) {
				t.Errorf("case %d: the error quotes a fixture value", i)
			}
		}
		if engine.ContainsSecretShape(msg) {
			t.Errorf("case %d: the error still looks like it carries key material: %q", i, msg)
		}
	}
}

// TestBuildRequiresALink checks that the two entry points stay distinct. A
// Build that quietly produced the fail-closed document when handed no link
// would give a box that looks configured and carries nothing.
func TestBuildRequiresALink(t *testing.T) {
	if _, err := Build(Options{}); !errors.Is(err, ErrNoLink) {
		t.Fatalf("Build with no link returned %v, want ErrNoLink", err)
	}
	if _, err := BuildFailClosed(Options{}); err != nil {
		t.Fatalf("BuildFailClosed with no link: %v", err)
	}
}

// TestLinkStillStampsTheTagThisPackageExpects checks the CONTRACT between the
// two packages: internal/link tags its outbound TagProxy, and every routing
// rule in this package names that tag.
//
// It does NOT exercise the guard in outboundFromDocument, and it never did.
// Until 2026-08-30 this test was called TestOutboundTagIsCheckedNotAssumed and
// its comment said it proved the guard fires "by handing Build a Link whose
// config document carries a different tag ... built by parsing a real link and
// then rewriting the tag in the bytes". The body below does no rewriting and
// never calls Build; the coverage profile confirmed the ErrOutboundTag branch
// had an execution count of zero. The name and comment were corrected rather
// than the body, because what the body does is worth doing: it is the
// cross-package contract check. The guard itself is now exercised by
// TestOutboundFromDocumentRefusesAWrongTag.
func TestLinkStillStampsTheTagThisPackageExpects(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	raw, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	if !strings.Contains(string(raw), `"tag":"`+TagProxy+`"`) {
		t.Fatalf("internal/link no longer tags its outbound %q; the guard in proxyOutbound is checking for the wrong value", TagProxy)
	}
}

// TestDefaultsAreTheDefaults checks that Defaults() and the normalisation path
// agree, so the panel cannot show one set of values while the engine runs
// another.
func TestDefaultsAreTheDefaults(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	fromZero, err := Build(Options{Link: l})
	if err != nil {
		t.Fatalf("Build with zero Options: %v", err)
	}
	d := Defaults()
	d.Link = l
	fromDefaults, err := Build(d)
	if err != nil {
		t.Fatalf("Build with Defaults(): %v", err)
	}
	if string(fromZero) != string(fromDefaults) {
		t.Error("a zero Options and Defaults() produce different configs; the panel would show values the engine is not running")
	}
}

// TestGeneratedConfigCarriesTheCredential is the negative of the redaction
// tests: the document handed to the engine MUST carry the key material, or
// the tunnel cannot authenticate. It is here so that a later attempt to
// "redact the config" fails loudly instead of producing a box that connects
// as somebody else.
func TestGeneratedConfigCarriesTheCredential(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	raw, err := Build(Options{Link: l})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	s := string(raw)
	for _, want := range []string{fakeUUID, fakePublicKey(), fakeShortID, fakeMldsa65Verify()} {
		if !strings.Contains(s, want) {
			t.Error("the generated config is missing a credential the engine needs to authenticate")
		}
	}
}

// TestLinkIsTheOnlyUserTextInTheDocument checks that the display name a user
// typed in a link's #fragment does not reach the config.
//
// internal/link moves it out of sendThrough and keeps it in Link.Tag for the
// panel; if it reached the document it would be a user-supplied string in a
// config identifier, which docs/2026-08-29-design.md section 6 forbids.
func TestLinkIsTheOnlyUserTextInTheDocument(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	if l.Tag == "" {
		t.Fatal("the fixture no longer carries a display name; this test would pass vacuously")
	}
	raw, err := Build(Options{Link: l})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if strings.Contains(string(raw), l.Tag) {
		t.Errorf("the link's display name reached the generated config")
	}
	if strings.Contains(string(raw), "sendThrough") {
		t.Error("sendThrough reached the generated config; the engine reads it as a bind address")
	}
}
