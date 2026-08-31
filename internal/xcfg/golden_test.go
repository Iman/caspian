// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"caspianbyoc.org/caspian/internal/engine"
)

// -update rewrites the golden files. Run it, then READ THE DIFF.
//
// The point of a golden file here is narrower and sharper than "the output did
// not change". This document is what a privileged process runs and what
// carries the user's credentials, and the properties that matter about it are
// invisible in a struct literal: which outbound is FIRST, what order the
// routing rules are in, which key the resolver list is under. A diff shows all
// three at once, to a person, before the change lands.
var update = flag.Bool("update", false, "rewrite the golden files in testdata")

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)

	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("could not write golden %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read golden %s: %v (run: go test ./internal/xcfg -update)", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("generated configuration does not match %s\n--- want ---\n%s\n--- got ---\n%s",
			path, want, got)
	}
}

// goldenCase is one document worth freezing.
type goldenCase struct {
	file string
	// build returns the document. It is a closure rather than an Options so
	// that the fail-closed cases can use the other entry point.
	build func(t *testing.T) []byte
	// mutate returns the Options this case sets, recovered without needing a
	// Link, so a guard can check WHICH fields the case actually touches.
	// Nil for cases that take no Options.
	mutate func() Options
}

func goldenCases() []goldenCase {
	withLink := func(raw func() string, mutate func(*Options)) func(*testing.T) []byte {
		return func(t *testing.T) []byte {
			t.Helper()
			o := Options{Link: mustParse(t, raw())}
			if mutate != nil {
				mutate(&o)
			}
			b, err := Build(o)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			return b
		}
	}

	return []goldenCase{
		// The reference document: a REALITY link and nothing overridden. This
		// is the one to read first when reviewing a change to this package.
		{file: "reality-default.json", build: withLink(vlessRealityLink, nil)},

		// The same link with the DNS interception switch on, so the two rule
		// sets can be compared side by side. Note in the diff that the
		// resolver rule stays ABOVE the port 53 rule.
		{file: "reality-intercept.json", build: withLink(vlessRealityLink, func(o *Options) {
			o.DNS.Intercept = true
		})},

		// The loopback DNS listener on its own, which is the configuration
		// that makes internal/hotspot's dnsmasq able to resolve anything.
		// Read the rule order in this file: the local DNS rule and the
		// resolver rule are both ABOVE private-direct, and that is what stops
		// a query being answered on the local network.
		{file: "reality-localdns.json", build: withLink(vlessRealityLink, func(o *Options) {
			o.LocalDNS.Enabled = true
		})},

		// Both DNS switches on at once, so the two rules can be compared and
		// so it is visible that they are separate rules with separate
		// conditions rather than one mechanism with two names.
		{file: "reality-localdns-and-intercept.json", build: withLink(vlessRealityLink, func(o *Options) {
			o.LocalDNS.Enabled = true
			o.DNS.Intercept = true
		})},

		// No TUN inbound. docs/2026-08-29-design.md section 8 step 4 is
		// exactly this document: engine plus a SOCKS inbound, used to capture
		// an exit IP before any addressing exists.
		{file: "reality-socks-only.json", build: withLink(vlessRealityLink, func(o *Options) {
			o.TUN.Disabled = true
		})},

		// Every option moved off its default at once, so that a change to any
		// default shows up in exactly one file and a change to the SHAPE
		// shows up in all of them.
		everythingOverridden(withLink),

		// One per protocol shape, so a change in how internal/link emits an
		// outbound is visible here rather than only at run time. These are
		// the mapping table in docs/2026-08-29-design.md section 4.5 frozen
		// as bytes.
		{file: "vless-tls-ws.json", build: withLink(vlessTLSWebsocketLink, nil)},
		{file: "vmess-ws-tls.json", build: withLink(vmessBase64Link, nil)},
		{file: "shadowsocks.json", build: withLink(shadowsocksSIP002Link, nil)},
		{file: "trojan.json", build: withLink(trojanLink, nil)},
		{file: "hysteria2.json", build: withLink(hysteria2Link, nil)},

		// The two schemes internal/link accepts that had no golden until
		// 2026-08-30. supportedSchemes in internal/link/link.go lists seven;
		// the five above plus these two are all of them, and
		// TestGolden_EverySupportedSchemeHasAGolden in
		// golden_transports_test.go now fails when an eighth is added without
		// one. See that file for what each is worth.
		{file: "socks.json", build: withLink(socksLink, nil)},
		{file: "hy2-alias.json", build: withLink(hy2AliasLink, nil)},

		// The fail-closed document. Its whole content is the claim that there
		// is no way out of it, so it is worth a file of its own.
		{file: "fail-closed.json", build: func(t *testing.T) []byte {
			t.Helper()
			b, err := BuildFailClosed(Options{})
			if err != nil {
				t.Fatalf("BuildFailClosed: %v", err)
			}
			return b
		}},
	}
}

func TestGolden(t *testing.T) {
	for _, c := range goldenCases() {
		t.Run(c.file, func(t *testing.T) {
			assertGolden(t, c.file, c.build(t))
		})
	}
}

// TestGoldenFilesAreAcceptedByTheEngine keeps the frozen bytes honest.
//
// A golden file records what this package produced when somebody last ran
// -update. That is not the same as saying the engine still accepts it: an
// engine upgrade can invalidate a document without changing a line of this
// package, and then TestGolden stays green while the box will not start. This
// reads the committed FILES, not the generated bytes, and puts them through
// the engine's loader.
func TestGoldenFilesAreAcceptedByTheEngine(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no golden files found; the check would pass vacuously")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		if err := engine.Validate(b); err != nil {
			t.Errorf("%s: the committed golden file is no longer accepted by the engine: %v", f, err)
		}
	}
	t.Logf("engine.Validate accepted %d committed golden files", len(files))
}

// TestGoldenCasesCoverEveryGoldenFile fails when testdata holds a file no case
// generates, which is how a stale golden survives a rename and quietly stops
// being checked by anything.
func TestGoldenCasesCoverEveryGoldenFile(t *testing.T) {
	want := map[string]bool{}
	for _, c := range goldenCases() {
		want[c.file] = true
	}
	files, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range files {
		name := filepath.Base(f)
		if !want[name] {
			t.Errorf("testdata/%s is not produced by any case in goldenCases(); it is stale", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("goldenCases() names testdata/%s, which does not exist; run: go test ./internal/xcfg -update", name)
	}
}

// TestEveryOptionsFieldIsOverriddenInTheGolden keeps the
// "reality-everything-overridden" case honest.
//
// A golden case named for covering everything is worth nothing once Options
// grows a field it does not set, and that failure is silent: the file still
// generates, still validates, and still matches itself. It happened once
// already, when LocalDNS was added and this case kept its old body.
//
// The check is that the case's own Options differs from Defaults() in every
// field the axes track, using the same field-path list and the same accessor
// as TestAxesCoverEveryOptionsField, so the two guards cannot drift apart.
func TestEveryOptionsFieldIsOverriddenInTheGolden(t *testing.T) {
	var overridden Options
	found := false
	for _, c := range goldenCases() {
		if c.file != "reality-everything-overridden.json" {
			continue
		}
		found = true
		// Re-run the case's mutation against a zero Options to recover what
		// it sets, without needing a Link.
		overridden = c.mutate()
	}
	if !found {
		t.Fatal("the reality-everything-overridden case is gone; this guard has nothing to check")
	}

	base := Options{}
	for _, path := range trackedOptionFields() {
		if path == "TUN.Disabled" {
			// See everythingOverriddenMutation: setting it would delete the
			// TUN inbound and hide the three TUN fields this case exists to
			// show. reality-socks-only.json is the case that covers it.
			continue
		}
		if fieldString(overridden, path) == fieldString(base, path) {
			t.Errorf("the everything-overridden golden case leaves Options.%s at its zero value, so the golden does not cover it", path)
		}
	}
}

// everythingOverriddenMutation is the single definition of what the
// "everything overridden" case sets. It lives in one place so that the golden
// case and the guard that checks its coverage cannot disagree.
func everythingOverriddenMutation(o *Options) {
	o.LogLevel = LogDebug
	// TUN.Disabled is deliberately NOT set here. Setting it would remove the
	// TUN inbound, and with it the only golden view of a custom interface
	// name, MTU and user level. It is covered by reality-socks-only.json
	// instead, and the guard below excludes it for this reason.
	o.TUN.Name = "csp0"
	o.TUN.MTU = 1420
	o.TUN.UserLevel = 1
	o.SOCKS.Listen = "::1"
	o.SOCKS.Port = 19999
	o.SOCKS.UDP = true
	o.DNS.Servers = []string{"9.9.9.11", "2620:fe::fe"}
	o.DNS.Strategy = QueryUseIPv4
	o.DNS.Intercept = true
	// Every field, or the name of this case is a lie. It was one for a short
	// time: LocalDNS was added to Options and not to this list, and the case
	// silently stopped covering what it claims.
	o.LocalDNS.Enabled = true
	o.LocalDNS.Listen = "::1"
	o.LocalDNS.Port = 15353
}

func everythingOverridden(withLink func(func() string, func(*Options)) func(*testing.T) []byte) goldenCase {
	return goldenCase{
		file:   "reality-everything-overridden.json",
		build:  withLink(vlessRealityLink, everythingOverriddenMutation),
		mutate: func() Options { var o Options; everythingOverriddenMutation(&o); return o },
	}
}
