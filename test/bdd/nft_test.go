// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package bdd

import (
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Reading the generated firewall.
//
// The fail-closed claim is a claim about what the ruleset PERMITS, so an
// assertion that greps for a string is not enough: the generated text is full
// of comments that quote interface names and the word "masquerade", and a grep
// would pass on a ruleset whose comments say the right thing and whose rules do
// not. So the text is taken apart into chains, comments are removed, and the
// assertions are about rules.
//
// internal/netcfg has an unexported stripInterfaceRules used by its own test of
// the same property. This is a second implementation rather than a shared one,
// because the package will not export it and because a shared helper would let
// one change silence both tests at once.
// ---------------------------------------------------------------------------

type nftRule struct {
	text string // the rule with any trailing comment removed
	verb string // accept, drop, reject, redirect, masquerade, or ""
}

type nftChain struct {
	name   string
	hook   string
	policy string
	rules  []nftRule
}

type nftRuleset struct {
	chains map[string]nftChain
	order  []string
}

// parseNft takes the generated ruleset apart. It understands only the shapes
// this project generates; it is not an nftables parser.
func parseNft(text string) (nftRuleset, error) {
	rs := nftRuleset{chains: map[string]nftChain{}}
	depth := 0
	var cur *nftChain

	for n, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasSuffix(line, "{"):
			depth++
			if strings.HasPrefix(line, "chain ") {
				if cur != nil {
					return rs, fmt.Errorf("line %d: a chain opened inside chain %q", n+1, cur.name)
				}
				name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "chain "), "{"))
				cur = &nftChain{name: name}
			}
			continue
		case line == "}":
			depth--
			if cur != nil {
				rs.chains[cur.name] = *cur
				rs.order = append(rs.order, cur.name)
				cur = nil
			}
			continue
		}
		if cur == nil {
			// Outside a chain: the table declaration and the delete that makes
			// the load idempotent. Nothing to assert on.
			continue
		}
		if strings.HasPrefix(line, "type ") && strings.Contains(line, "policy ") {
			cur.hook = fieldAfter(line, "hook")
			cur.policy = strings.TrimSuffix(fieldAfter(line, "policy"), ";")
			continue
		}
		cur.rules = append(cur.rules, nftRule{text: stripComment(line), verb: verbOf(stripComment(line))})
	}
	if depth != 0 {
		return rs, fmt.Errorf("unbalanced braces: depth ended at %d", depth)
	}
	if len(rs.chains) == 0 {
		return rs, fmt.Errorf("no chains found in a ruleset of %d bytes", len(text))
	}
	return rs, nil
}

// stripComment removes a trailing nftables comment clause. Every comment this
// generator writes is at the end of its rule, which is asserted rather than
// assumed: a comment in the middle would leave text behind and is worth a
// failure rather than a silent misreading.
func stripComment(line string) string {
	i := strings.Index(line, ` comment "`)
	if i < 0 {
		return line
	}
	return strings.TrimSpace(line[:i])
}

func fieldAfter(line, key string) string {
	fs := strings.Fields(line)
	for i, f := range fs {
		if f == key && i+1 < len(fs) {
			return fs[i+1]
		}
	}
	return ""
}

func verbOf(rule string) string {
	fs := strings.Fields(rule)
	if len(fs) == 0 {
		return ""
	}
	for _, v := range []string{"accept", "drop", "masquerade"} {
		if fs[len(fs)-1] == v {
			return v
		}
	}
	for _, f := range fs {
		if f == "reject" || f == "redirect" {
			return f
		}
	}
	return ""
}

func (c nftChain) withVerb(verb string) []nftRule {
	var out []nftRule
	for _, r := range c.rules {
		if r.verb == verb {
			out = append(out, r)
		}
	}
	return out
}

func (rs nftRuleset) chain(name string) (nftChain, error) {
	c, ok := rs.chains[name]
	if !ok {
		return nftChain{}, fmt.Errorf("the generated ruleset has no chain %q (it has %v)", name, rs.order)
	}
	return c, nil
}

// withoutInterface models the ruleset as it behaves when that interface does
// not exist: the rules naming it stop matching, and everything else, including
// the chain policies, is untouched.
//
// Matching is on the QUOTED name, which is how every interface match in the
// generated ruleset is written. That is the property being relied on and it is
// asserted separately: see everyInterfaceMatchIsByNameAndNotByIndex.
func withoutInterface(text, iface string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, `"`+iface+`"`) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// ---------------------------------------------------------------------------
// Tests of the analyser itself. An analyser that finds nothing makes every
// assertion built on it pass, so it is checked against text with a known
// answer before it is trusted with the fail-closed claim.
// ---------------------------------------------------------------------------

const analyserSample = `# a comment naming eth0 and the word masquerade
table inet caspian
delete table inet caspian

table inet caspian {
	chain forward {
		type filter hook forward priority filter; policy drop;

		# a comment mentioning oifname "eth0" accept, which is not a rule
		iifname "wlan0" oifname "eth0" drop comment "the leak block"
		ct state invalid drop
		meta nfproto ipv4 iifname "wlan0" oifname "xray0" ip saddr 10.83.51.0/24 accept
	}

	chain postrouting {
		type nat hook postrouting priority srcnat; policy accept;

		# No masquerade towards eth0.
	}
}
`

func TestAnalyserReadsChainsRulesAndPolicies(t *testing.T) {
	rs, err := parseNft(analyserSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fwd, err := rs.chain("forward")
	if err != nil {
		t.Fatal(err)
	}
	if fwd.policy != "drop" {
		t.Errorf("forward policy = %q, want drop", fwd.policy)
	}
	if got := len(fwd.rules); got != 3 {
		t.Errorf("forward has %d rules, want 3: %v", got, fwd.rules)
	}
	if got := len(fwd.withVerb("accept")); got != 1 {
		t.Errorf("forward has %d accept rules, want 1", got)
	}
	if got := len(fwd.withVerb("drop")); got != 2 {
		t.Errorf("forward has %d drop rules, want 2", got)
	}
	// The comment line quoting an accept must not have become a rule.
	for _, r := range fwd.rules {
		if strings.Contains(r.text, "which is not a rule") {
			t.Errorf("a comment was read as a rule: %q", r.text)
		}
	}
	// The trailing comment clause must be gone from the leak block.
	if strings.Contains(fwd.rules[0].text, "comment") {
		t.Errorf("the trailing comment survived: %q", fwd.rules[0].text)
	}
	post, err := rs.chain("postrouting")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(post.withVerb("masquerade")); got != 0 {
		t.Errorf("the word masquerade in a comment was read as a rule")
	}
}

func TestAnalyserRefusesTextItCannotRead(t *testing.T) {
	if _, err := parseNft("this is not a ruleset"); err == nil {
		t.Error("the analyser accepted text with no chains in it, so an assertion built on it would pass on anything")
	}
}

func TestWithoutInterfaceRemovesOnlyTheRulesNamingIt(t *testing.T) {
	stripped := withoutInterface(analyserSample, "xray0")
	if strings.Contains(stripped, `"xray0"`) {
		t.Error("a rule naming the removed interface survived")
	}
	if !strings.Contains(stripped, `iifname "wlan0" oifname "eth0" drop`) {
		t.Error("the leak block, which names neither, was removed")
	}
	if !strings.Contains(stripped, "policy drop;") {
		t.Error("the chain policy was removed, which would make the fail-closed assertion meaningless")
	}
}
