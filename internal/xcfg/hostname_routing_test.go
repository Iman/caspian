// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The tests in this file pin the routing property a future consumer of the
// loopback SOCKS inbound depends on: a connection that arrives on TagSOCKSIn
// with a HOSTNAME destination can reach only the proxy outbound, and the one
// rule that sends traffic direct matches on IP literals alone.
//
// Why a hostname destination is special under this document. The router's
// domainStrategy is AsIs (assemble, build.go), and app/router/router.go:253
// and :263 in the pinned engine consult the DNS client only for IpOnDemand and
// IpIfNonMatch, so a domain-targeted connection is routed with NO IP known.
// features/routing/session/context.go:49-59 returns nil target IPs for a
// domain address, and app/router/condition.go:92-108 (IPMatcher.Apply) matches
// against that nil list, so an "ip" rule cannot match it. The reverse holds
// too: condition.go:62-68 (DomainMatcher.Apply) returns false when
// GetTargetDomain is empty, which context.go:91-104 gives for an IP-literal
// destination. And router.go:257-261 takes the FIRST rule whose conditions all
// hold, so rule order is what the assertions below reason about.
//
// The rules are decoded as generic JSON objects rather than through the typed
// parsed struct in build_test.go, because the property is about which KEYS a
// rule carries. A "domain" key that parsed does not declare would be invisible
// to a typed decode and fully visible to the engine.
//
// The checks are written as functions that RETURN an error rather than call
// t.Fatal, so that the negative test at the bottom can feed them hand-built
// documents and prove they bite. A guard that has never been seen to fail has
// not been shown to detect anything.

// rawRule is one routing rule with every key it carries, whatever the key is.
type rawRule map[string]json.RawMessage

// rawRules returns the routing rules of a document as key sets.
func rawRules(raw []byte) ([]rawRule, error) {
	var doc struct {
		Routing struct {
			Rules []rawRule `json:"rules"`
		} `json:"routing"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("document is not valid JSON: %w", err)
	}
	return doc.Routing.Rules, nil
}

// tag is the rule's ruleTag, or a placeholder when it has none, so that every
// failure below can name the rule it is about.
func (r rawRule) tag() string {
	s, err := r.str("ruleTag")
	if err != nil || s == "" {
		return "<untagged>"
	}
	return s
}

// str decodes a string-valued key, or returns "" when the key is absent.
func (r rawRule) str(key string) (string, error) {
	v, ok := r[key]
	if !ok {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return "", fmt.Errorf("rule %q: key %q is not a JSON string", r.tag(), key)
	}
	return s, nil
}

// list decodes a string-list key, or returns nil when the key is absent.
func (r rawRule) list(key string) ([]string, error) {
	v, ok := r[key]
	if !ok {
		return nil, nil
	}
	var l []string
	if err := json.Unmarshal(v, &l); err != nil {
		return nil, fmt.Errorf("rule %q: key %q is not a JSON string list", r.tag(), key)
	}
	return l, nil
}

// The rule keys the engine reads as a DOMAIN condition. Both spellings are
// accepted by parseFieldRule in the pinned engine (infra/conf/router.go:135-136,
// applied at :174 and :182), so both are forbidden here.
var domainKeys = []string{"domain", "domains"}

// productionOptions applies the two switches internal/privsvc sets on every
// start (internal/privsvc/plans.go:105 and :110), so the sweep below covers the
// document the box actually runs and not only the zero-value one.
func productionOptions(o *Options) {
	o.LocalDNS.Enabled = true
	o.DNS.Intercept = true
}

// checkHostnameReachesOnlyProxy holds one Build document's rules to the
// property. Every violation is returned, joined, and each names its rule.
func checkHostnameReachesOnlyProxy(rules []rawRule) error {
	if len(rules) == 0 {
		return errors.New("no routing rules at all")
	}
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	for i, r := range rules {
		tag := r.tag()
		out, err := r.str("outboundTag")
		if err != nil {
			return err
		}

		// No rule anywhere may match on a domain. The engine has no data
		// file for geosite (see TestNoGeoRules) and this package writes no
		// literal domain list either; a domain rule is the one shape that
		// could send a hostname destination somewhere other than the proxy.
		for _, k := range domainKeys {
			if _, has := r[k]; has {
				fail("rule %d (%q) carries a %q key; no rule in this document may match on a domain", i, tag, k)
			}
		}

		inbound, err := r.list("inboundTag")
		if err != nil {
			return err
		}
		ips, err := r.list("ip")
		if err != nil {
			return err
		}
		port, err := r.str("port")
		if err != nil {
			return err
		}

		// A rule pinned to some OTHER inbound cannot see a socks-in
		// connection at all. A rule that names socks-in must still go to the
		// proxy, or the property fails by construction.
		for _, in := range inbound {
			if in == TagSOCKSIn && out != TagProxy {
				fail("rule %d (%q) matches inbound %q and routes to %q, want %q", i, tag, TagSOCKSIn, out, TagProxy)
			}
		}
		if len(inbound) > 0 {
			continue
		}

		// The port rule is the DNS intercept. It is allowed to route to the
		// dns outbound, and only when it is exactly the port 53 intercept: a
		// wider port list here would be a second path off the proxy.
		if port != "" {
			if tag != ruleTagDNS || port != "53" || out != TagDNSOut {
				fail("rule %d (%q) has port %q routing to %q; the only port rule permitted is %q on 53 to %q",
					i, tag, port, out, ruleTagDNS, TagDNSOut)
			}
			continue
		}

		// No inbound restriction and no port restriction: this rule CAN see a
		// socks-in connection. It must be IP-only, which a hostname destination
		// cannot match (context.go:49-59 gives nil target IPs), or it must
		// send to the proxy.
		if len(ips) > 0 {
			if out == TagProxy {
				continue
			}
			// An IP rule off the proxy is fine ONLY because it cannot match a
			// hostname. Name any second condition shape this check does not
			// know about rather than let it through.
			for k := range r {
				switch k {
				case "ruleTag", "outboundTag", "ip", "network":
				default:
					fail("rule %d (%q) routes to %q on ip plus an unexpected key %q", i, tag, out, k)
				}
			}
			continue
		}
		if out != TagProxy {
			fail("rule %d (%q) has no inboundTag, no port and no ip, and routes to %q; a hostname on %q would reach it",
				i, tag, out, TagSOCKSIn)
		}
	}

	// The catch-all is where a hostname destination on socks-in actually
	// lands, and it must be the LAST rule, named, tunnelled, and on both
	// networks. "tcp,udp" is not decoration: NetworkList.Build returns TCP
	// only for a nil list (build.go, the comment on ruleTagCatchAll), so a
	// catch-all without it would leave a UDP associate unmatched.
	last := rules[len(rules)-1]
	lastTag := last.tag()
	if lastTag != ruleTagCatchAll {
		fail("last rule is %q, want %q", lastTag, ruleTagCatchAll)
	}
	if out, _ := last.str("outboundTag"); out != TagProxy {
		fail("last rule (%q) routes to %q, want %q", lastTag, out, TagProxy)
	}
	if nw, _ := last.str("network"); nw != "tcp,udp" {
		fail("last rule (%q) has network %q, want %q", lastTag, nw, "tcp,udp")
	}
	for _, k := range []string{"inboundTag", "ip", "port"} {
		if _, has := last[k]; has {
			fail("last rule (%q) carries %q, so it is not a catch-all", lastTag, k)
		}
	}
	return errors.Join(errs...)
}

// checkFailClosedRules is the fail-closed half: no proxy outbound exists, so
// every rule must route to the blackhole and none may match on a domain.
func checkFailClosedRules(rules []rawRule) error {
	if len(rules) == 0 {
		return errors.New("no routing rules at all")
	}
	var errs []error
	for i, r := range rules {
		for _, k := range domainKeys {
			if _, has := r[k]; has {
				errs = append(errs, fmt.Errorf("rule %d (%q) carries a %q key", i, r.tag(), k))
			}
		}
		if out, _ := r.str("outboundTag"); out != TagBlock {
			errs = append(errs, fmt.Errorf("rule %d (%q) routes to %q, want %q", i, r.tag(), out, TagBlock))
		}
	}
	return errors.Join(errs...)
}

// TestAHostnameDestinationOnTheSOCKSInboundReachesOnlyTheProxy asserts, over
// every fixture and every option combination, that a connection arriving on
// TagSOCKSIn with a hostname destination has exactly one place to go: the
// proxy outbound.
//
// The reasoning is structural rather than a simulated route. Under AsIs no
// rule sees an IP for a hostname destination, so the rules that could match
// such a connection are those with no inboundTag and no port condition. Each
// of them must be IP-only (unmatchable) or route to the proxy, and the
// document must end with the named catch-all to the proxy. A subscription
// refresh sent through the loopback SOCKS inbound relies on this: its request
// leaves through the tunnel or not at all.
//
// The sweep runs twice. Once with the two production switches forced on, as
// internal/privsvc sets them, because generated() cannot inject options and
// the document the box runs is the one that matters most. Once over
// generated() itself, so every combination, including the fail-closed
// documents, is covered: a fail-closed document has no proxy outbound and its
// property is the stronger one, every rule routes to the blackhole.
func TestAHostnameDestinationOnTheSOCKSInboundReachesOnlyTheProxy(t *testing.T) {
	t.Run("production-options", func(t *testing.T) {
		count := 0
		for _, f := range fixtures() {
			l := mustParse(t, f.raw())
			for _, c := range combinations() {
				o := c.opts
				o.Link = l
				productionOptions(&o)
				raw, err := Build(o)
				if err != nil {
					t.Fatalf("%s/%s: Build: %v", f.name, c.name, err)
				}
				rules, err := rawRules(raw)
				if err != nil {
					t.Fatalf("%s/%s: %v", f.name, c.name, err)
				}
				if err := checkHostnameReachesOnlyProxy(rules); err != nil {
					t.Errorf("%s/%s: %v", f.name, c.name, err)
				}
				count++
			}
		}
		if count == 0 {
			t.Fatal("no configs were generated; the check would pass vacuously")
		}
		t.Logf("checked %d documents with LocalDNS.Enabled and DNS.Intercept forced on", count)
	})

	t.Run("generated", func(t *testing.T) {
		count := 0
		generated(t, func(name string, raw []byte) {
			count++
			rules, err := rawRules(raw)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if strings.HasPrefix(name, "fail-closed/") {
				err = checkFailClosedRules(rules)
			} else {
				err = checkHostnameReachesOnlyProxy(rules)
			}
			if err != nil {
				t.Errorf("%s: %v", name, err)
			}
		})
		if count == 0 {
			t.Fatal("no configs were generated; the check would pass vacuously")
		}
	})
}

// cidrLiteralsInPrivateGo reads private.go as text and returns every quoted
// string literal in it that parses as a CIDR prefix, in file order.
//
// The list is taken from the SOURCE rather than from PrivateRanges() so that a
// later change which makes PrivateRanges compute or fetch its ranges, rather
// than spell them out, turns this test red instead of quietly agreeing with
// itself. TestPrivateRangesRouteDirect compares against the function; this
// compares against the file.
func cidrLiteralsInPrivateGo(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("private.go")
	if err != nil {
		t.Fatalf("read private.go: %v", err)
	}
	quoted := regexp.MustCompile(`"([^"\n]+)"`)
	var out []string
	for _, m := range quoted.FindAllStringSubmatch(string(src), -1) {
		if _, err := netip.ParsePrefix(m[1]); err == nil {
			out = append(out, m[1])
		}
	}
	return out
}

// checkPrivateDirectIPOnly holds the private-direct rule of one Build
// document to its shape: present exactly once, routed direct, an "ip" list
// equal to want in order, every entry a CIDR, and no other condition key.
func checkPrivateDirectIPOnly(rules []rawRule, want []string) error {
	var private []rawRule
	for _, r := range rules {
		if r.tag() == ruleTagPrivate {
			private = append(private, r)
		}
	}
	if len(private) != 1 {
		return fmt.Errorf("%d rules tagged %q, want exactly 1", len(private), ruleTagPrivate)
	}
	r := private[0]
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if out, _ := r.str("outboundTag"); out != TagDirect {
		fail("%q routes to %q, want %q", ruleTagPrivate, out, TagDirect)
	}
	ips, err := r.list("ip")
	if err != nil {
		return err
	}
	if strings.Join(ips, ",") != strings.Join(want, ",") {
		fail("%q ip list is %v, want the private.go literals %v in order", ruleTagPrivate, ips, want)
	}
	for _, ip := range ips {
		if _, err := netip.ParsePrefix(ip); err != nil {
			fail("%q entry %q is not a CIDR prefix, so the rule is not IP-only", ruleTagPrivate, ip)
		}
	}
	// The ONLY keys. Anything else is a second condition that changes which
	// connections the rule can see, in either direction.
	for k := range r {
		switch k {
		case "ruleTag", "ip", "outboundTag":
		default:
			fail("%q carries key %q; the rule must match on ip alone", ruleTagPrivate, k)
		}
	}
	return errors.Join(errs...)
}

// TestPrivateDirectMatchesByIPOnly pins the shape of the one rule that sends
// traffic off the tunnel: it matches on an "ip" list that is exactly the CIDR
// literals in private.go, and on nothing else.
//
// Two consequences follow, and a future consumer of the SOCKS inbound must
// know both. An IP-LITERAL destination inside a private range, arriving on
// ANY inbound, goes direct; that is by design (private.go, "Why direct rather
// than blocked"), and it is the case a hostname-only URL validator has to
// exclude by refusing IP literals. A HOSTNAME destination cannot match this
// rule at all, because under AsIs the router has no IP for it
// (features/routing/session/context.go:49-59, app/router/condition.go:92-108).
//
// Every entry is also checked to parse as a CIDR. A "geoip:" token or a
// hostname in the list would not be an IP-only rule and would fail here before
// TestNoGeoRules names the reason.
func TestPrivateDirectMatchesByIPOnly(t *testing.T) {
	want := cidrLiteralsInPrivateGo(t)
	if len(want) == 0 {
		t.Fatal("no CIDR literals found in private.go; the comparison would pass vacuously")
	}
	if got := PrivateRanges(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("PrivateRanges() returns %v but private.go spells out %v; the function no longer returns the file's literal list", got, want)
	}

	count := 0
	generated(t, func(name string, raw []byte) {
		rules, err := rawRules(raw)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.HasPrefix(name, "fail-closed/") {
			// No direct outbound exists there, so no rule may name it.
			for _, r := range rules {
				if r.tag() == ruleTagPrivate {
					t.Errorf("%s: a fail-closed document carries the %q rule", name, ruleTagPrivate)
				}
			}
			return
		}
		count++
		if err := checkPrivateDirectIPOnly(rules, want); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	})
	if count == 0 {
		t.Fatal("no Build documents were generated; the check would pass vacuously")
	}
}

// TestHostnameRoutingGuardsBite is the unhappy path for the two checks above.
//
// Each case is a hand-built routing document with one defect the guards must
// name, and the assertion is on the error text: it must mention the tag of
// the offending rule, because a guard that fails without saying which rule is
// wrong sends the reader back to the 41472-document sweep to find out. The
// first case is healthy and must PASS both checks, so a helper that rejects
// everything cannot make the table green.
func TestHostnameRoutingGuardsBite(t *testing.T) {
	// A well-formed rule set in this package's own order. Each defective case
	// below is this with one edit.
	healthy := func() []map[string]any {
		return []map[string]any{
			{"ruleTag": ruleTagLocalDNS, "inboundTag": []string{TagLocalDNSIn}, "outboundTag": TagDNSOut},
			{"ruleTag": ruleTagResolvers, "inboundTag": []string{TagResolverIn}, "outboundTag": TagProxy},
			{"ruleTag": ruleTagDNS, "port": "53", "network": "tcp,udp", "outboundTag": TagDNSOut},
			{"ruleTag": ruleTagPrivate, "ip": PrivateRanges(), "outboundTag": TagDirect},
			{"ruleTag": ruleTagCatchAll, "network": "tcp,udp", "outboundTag": TagProxy},
		}
	}
	const privateIdx, catchAllIdx = 3, 4

	cases := []struct {
		name string
		edit func(rules []map[string]any) []map[string]any
		// Which check must fail, and the rule tag its error must name. An
		// empty tag means the check must pass.
		hostnameNames string
		privateNames  string
	}{
		{
			name: "healthy",
			edit: func(r []map[string]any) []map[string]any { return r },
		},
		{
			name: "a rule with a domain key",
			edit: func(r []map[string]any) []map[string]any {
				bad := map[string]any{"ruleTag": "domain-rule", "domain": []string{"example.invalid"}, "outboundTag": TagProxy}
				return append([]map[string]any{bad}, r...)
			},
			hostnameNames: "domain-rule",
		},
		{
			name: "domains spelling on private-direct",
			edit: func(r []map[string]any) []map[string]any {
				r[privateIdx]["domains"] = []string{"example.invalid"}
				return r
			},
			hostnameNames: ruleTagPrivate,
			privateNames:  ruleTagPrivate,
		},
		{
			name: "private-direct restricted to an inbound",
			edit: func(r []map[string]any) []map[string]any {
				r[privateIdx]["inboundTag"] = []string{TagTUNIn}
				return r
			},
			privateNames: ruleTagPrivate,
		},
		{
			name: "private-direct with a geoip token",
			edit: func(r []map[string]any) []map[string]any {
				r[privateIdx]["ip"] = []string{"geoip:private"}
				return r
			},
			privateNames: ruleTagPrivate,
		},
		{
			name: "everything-else pointing at direct",
			edit: func(r []map[string]any) []map[string]any {
				r[catchAllIdx]["outboundTag"] = TagDirect
				return r
			},
			hostnameNames: ruleTagCatchAll,
		},
		{
			name: "everything-else without udp",
			edit: func(r []map[string]any) []map[string]any {
				r[catchAllIdx]["network"] = "tcp"
				return r
			},
			hostnameNames: ruleTagCatchAll,
		},
		{
			name: "missing everything-else",
			edit: func(r []map[string]any) []map[string]any {
				return r[:catchAllIdx]
			},
			// With the catch-all gone the last rule is private-direct, and
			// the failure names what it found in that position.
			hostnameNames: ruleTagPrivate,
		},
		{
			name: "a bare rule to direct above the catch-all",
			edit: func(r []map[string]any) []map[string]any {
				bad := map[string]any{"ruleTag": "bare-direct", "network": "tcp,udp", "outboundTag": TagDirect}
				return append(r[:catchAllIdx], bad, r[catchAllIdx])
			},
			hostnameNames: "bare-direct",
		},
		{
			name: "a socks-in rule to direct",
			edit: func(r []map[string]any) []map[string]any {
				bad := map[string]any{"ruleTag": "socks-direct", "inboundTag": []string{TagSOCKSIn}, "outboundTag": TagDirect}
				return append([]map[string]any{bad}, r...)
			},
			hostnameNames: "socks-direct",
		},
	}

	want := PrivateRanges()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := json.Marshal(map[string]any{"routing": map[string]any{"rules": tc.edit(healthy())}})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			rules, err := rawRules(doc)
			if err != nil {
				t.Fatalf("rawRules: %v", err)
			}
			expect := func(check string, err error, names string) {
				t.Helper()
				if names == "" {
					if err != nil {
						t.Errorf("%s rejected a document it should accept: %v", check, err)
					}
					return
				}
				if err == nil {
					t.Fatalf("%s accepted the defective document", check)
				}
				if !strings.Contains(err.Error(), `"`+names+`"`) {
					t.Errorf("%s failed without naming rule %q: %v", check, names, err)
				}
			}
			expect("checkHostnameReachesOnlyProxy", checkHostnameReachesOnlyProxy(rules), tc.hostnameNames)
			expect("checkPrivateDirectIPOnly", checkPrivateDirectIPOnly(rules, want), tc.privateNames)
		})
	}

	// And the fail-closed half, once: a rule to anything but the blackhole.
	doc := []byte(`{"routing":{"rules":[{"ruleTag":"leak","network":"tcp,udp","outboundTag":"` + TagDirect + `"}]}}`)
	rules, err := rawRules(doc)
	if err != nil {
		t.Fatalf("rawRules: %v", err)
	}
	if err := checkFailClosedRules(rules); err == nil || !strings.Contains(err.Error(), `"leak"`) {
		t.Errorf("checkFailClosedRules did not name the leaking rule: %v", err)
	}
}
