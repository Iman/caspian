// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"errors"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/link"
)

// The tests in this file cover the branches the coverage profile showed
// unreached on 2026-08-30. Most are CHARACTERISATION tests: the production
// code already behaved correctly and nothing would have noticed if it stopped.
//
// The exception is TestOutboundFromDocumentRefusesAWrongTag and its
// neighbours. Those guards were genuinely unreachable from any test before
// proxyOutbound was split into "get the bytes" and "check the bytes", and a
// test in build_test.go claimed to exercise one of them while doing no such
// thing. That claim is corrected at TestLinkStillStampsTheTagThisPackageExpects.

// --- the guards in outboundFromDocument -------------------------------------

// TestOutboundFromDocumentRefusesAWrongTag is the test the old
// TestOutboundTagIsCheckedNotAssumed said it was.
//
// Why the tag matters more than it looks: every routing rule in this package
// names TagProxy. A rule pointing at an outbound that is not there does not
// fall back to something else, it closes the connection
// (app/dispatcher/default.go:481-484). So an outbound carrying the wrong tag
// produces a box that starts, reports connected, and carries nothing, which is
// the single hardest failure for a non-technical user to describe.
func TestOutboundFromDocumentRefusesAWrongTag(t *testing.T) {
	doc := `{"outbounds":[{"tag":"not-` + TagProxy + `","protocol":"vless"}]}`
	if _, err := outboundFromDocument([]byte(doc)); !errors.Is(err, ErrOutboundTag) {
		t.Errorf("an outbound tagged something else returned %v, want ErrOutboundTag", err)
	}

	// An outbound with no tag at all is the same failure: the empty string is
	// not TagProxy.
	if _, err := outboundFromDocument([]byte(`{"outbounds":[{"protocol":"vless"}]}`)); !errors.Is(err, ErrOutboundTag) {
		t.Errorf("an untagged outbound returned %v, want ErrOutboundTag", err)
	}

	// The right tag passes, so this test fails if the guard is widened into
	// refusing everything.
	ok := `{"outbounds":[{"tag":"` + TagProxy + `","protocol":"vless"}]}`
	raw, err := outboundFromDocument([]byte(ok))
	if err != nil {
		t.Fatalf("a correctly tagged outbound was refused: %v", err)
	}
	if !strings.Contains(string(raw), `"`+TagProxy+`"`) {
		t.Errorf("the returned outbound is not the one that was passed in: %s", raw)
	}
}

// TestOutboundFromDocumentRefusesAnEmptyDocument covers ErrOutboundMissing.
// An outbounds array with nothing in it builds in the engine and then has no
// handler to dispatch to, which is the same silent failure as the wrong tag.
func TestOutboundFromDocumentRefusesAnEmptyDocument(t *testing.T) {
	for _, doc := range []string{`{"outbounds":[]}`, `{}`} {
		if _, err := outboundFromDocument([]byte(doc)); !errors.Is(err, ErrOutboundMissing) {
			t.Errorf("outboundFromDocument(%s) returned %v, want ErrOutboundMissing", doc, err)
		}
	}
}

// TestOutboundFromDocumentRefusesUndecodableBytes covers the two errSerialise
// branches. They are not reachable from internal/link today; they are here
// because "internal/link always emits valid JSON" is an assumption about
// another package, and an assumption that is checked costs one branch.
func TestOutboundFromDocumentRefusesUndecodableBytes(t *testing.T) {
	// The document itself is not JSON.
	if _, err := outboundFromDocument([]byte(`not json`)); err == nil {
		t.Error("outboundFromDocument accepted bytes that are not JSON")
	}
	// The document is JSON but the outbound inside it is not an object, so the
	// tag decode is what fails.
	_, err := outboundFromDocument([]byte(`{"outbounds":["a string"]}`))
	if err == nil {
		t.Fatal("outboundFromDocument accepted an outbound that is not an object")
	}
	// Neither error may quote the bytes: this document carries the user's
	// credential material.
	if strings.Contains(err.Error(), "a string") {
		t.Errorf("the error quotes the document it was given: %v", err)
	}
}

// TestBuildRefusesALinkThatWasNeverParsed reaches proxyOutbound's error path
// through Build, which is the only way a caller could hit it: a zero
// link.Link is what &link.Link{} gives, and it carries no outbound.
//
// Build must refuse it rather than emit a document whose outbounds array is
// empty. It is separate from TestBuildRequiresALink, which covers a nil Link;
// this one is a non-nil Link with nothing in it.
func TestBuildRefusesALinkThatWasNeverParsed(t *testing.T) {
	_, err := Build(Options{Link: &link.Link{}})
	if err == nil {
		t.Fatal("Build accepted a Link that was never parsed")
	}
	if !errors.Is(err, link.ErrNoLink) {
		t.Errorf("Build with an unparsed Link returned %v, want link.ErrNoLink", err)
	}
}

// --- Options.check branches -------------------------------------------------

// TestInterfaceNameIsCheckedAgainstTheKernelRules covers checkInterfaceName.
//
// The engine does not check this: infra/conf/tun.go copies the name through
// and proxy/tun/tun_linux.go hands it to TUNSETIFF, so a bad name surfaces as
// an ioctl failure at start time. On this appliance the name also reaches
// generated firewall and route commands, so a name carrying a space or a
// newline is a correctness problem before it is a usability one.
func TestInterfaceNameIsCheckedAgainstTheKernelRules(t *testing.T) {
	bad := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"longer than IFNAMSIZ minus the NUL", strings.Repeat("a", maxInterfaceName+1)},
		{"a space", "xray 0"},
		{"a slash", "xray/0"},
		{"a newline", "xray0\ndevice"},
		{"a shell metacharacter", "xray0;reboot"},
		{"a NUL", "xray0\x00"},
		{"the current directory", "."},
		{"the parent directory", ".."},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			o := Defaults()
			o.TUN.Name = tc.in
			err := o.check()
			if !errors.Is(err, ErrTunName) {
				t.Fatalf("TUN name %q returned %v, want ErrTunName", tc.in, err)
			}
			// The name is operator input, and errors.go promises no value is
			// quoted.
			if tc.in != "" && strings.Contains(err.Error(), tc.in) {
				t.Errorf("the error quotes the offending name: %v", err)
			}
		})
	}

	good := []string{"xray0", "tun0", "a", strings.Repeat("a", maxInterfaceName), "eth-0_1.2"}
	for _, name := range good {
		o := Defaults()
		o.TUN.Name = name
		if err := o.check(); err != nil {
			t.Errorf("TUN name %q was refused: %v", name, err)
		}
	}
}

// TestInterfaceNameIsNotCheckedWhenThereIsNoTunnel pins the deliberate
// exemption: with TUN.Disabled set there is no interface to name, so the name
// is not validated. Without this test the exemption could be removed and only
// a fail-closed configuration would break.
func TestInterfaceNameIsNotCheckedWhenThereIsNoTunnel(t *testing.T) {
	o := Defaults()
	o.TUN.Disabled = true
	o.TUN.Name = "not a valid name"
	if err := o.check(); err != nil {
		t.Errorf("the TUN name was validated even though the tunnel is disabled: %v", err)
	}
}

// TestSocksPortZeroIsRefused covers ErrSocksPort.
//
// It has to go through check rather than Build, because normalise replaces a
// zero port with the default before check ever sees it. The guard is still
// worth having and worth testing: errors.go records that the engine does NOT
// catch this, since a JSON port of 0 leaves PortList.Range empty and
// InboundDetourConfig.Build accepts the empty list, so the inbound builds and
// then listens on nothing.
func TestSocksPortZeroIsRefused(t *testing.T) {
	o := Defaults()
	o.SOCKS.Port = 0
	if err := o.check(); !errors.Is(err, ErrSocksPort) {
		t.Errorf("a zero SOCKS port returned %v, want ErrSocksPort", err)
	}

	// And the reason it cannot be reached through Build: normalise fills it.
	if got := (Options{}).normalise().SOCKS.Port; got != DefaultSocksPort {
		t.Errorf("normalise left the SOCKS port at %d; a zero port would now reach check through Build", got)
	}
}

// TestEmptyResolverListIsRefused covers ErrNoResolvers, for the same reason
// and by the same route: normalise fills the list, so the guard is reached
// through check.
//
// errors.go records why the engine does not catch it: DNSConfig.Build iterates
// the servers with no minimum-length check, leaving the DNS app with no client
// at all.
func TestEmptyResolverListIsRefused(t *testing.T) {
	if err := checkResolvers(nil); !errors.Is(err, ErrNoResolvers) {
		t.Errorf("checkResolvers(nil) returned %v, want ErrNoResolvers", err)
	}
	if err := checkResolvers([]string{}); !errors.Is(err, ErrNoResolvers) {
		t.Errorf("checkResolvers of an empty slice returned %v, want ErrNoResolvers", err)
	}

	o := Defaults()
	o.DNS.Servers = nil
	if err := o.check(); !errors.Is(err, ErrNoResolvers) {
		t.Errorf("an empty resolver list returned %v, want ErrNoResolvers", err)
	}

	if got := (Options{}).normalise().DNS.Servers; len(got) == 0 {
		t.Error("normalise left the resolver list empty; the appliance would run with no resolver")
	}
}

// --- the local DNS listener -------------------------------------------------
//
// The two tests below cover branches in the LocalDNS code that arrived in this
// package on 2026-08-30 from other work in progress. They are additive and
// live in this file rather than in localdns_test.go so that they do not
// collide with whoever is still editing that file.

// TestLocalDNSPortZeroIsRefused covers ErrLocalDNSPort. It is the same trap as
// ErrSocksPort: a JSON port of 0 produces an inbound that builds and then
// listens on nothing, and the engine reports no problem.
func TestLocalDNSPortZeroIsRefused(t *testing.T) {
	o := Defaults()
	o.LocalDNS.Enabled = true
	o.LocalDNS.Listen = "127.0.0.1"
	o.LocalDNS.Port = 0
	if err := o.check(); !errors.Is(err, ErrLocalDNSPort) {
		t.Errorf("a zero local DNS port returned %v, want ErrLocalDNSPort", err)
	}

	// A non-zero port on the same settings passes, so the guard is not simply
	// refusing every local DNS configuration.
	o.LocalDNS.Port = 5353
	if err := o.check(); err != nil {
		t.Errorf("a valid local DNS configuration was refused: %v", err)
	}
}

// TestSameHostFallsBackToTextWhenAnAddressWillNotParse covers the fallback in
// sameHost.
//
// The comparison exists to stop two inbounds binding one address and port,
// which the engine does not report until Start. The fallback matters because a
// collision must not be hidden by an address neither side can parse: two
// identical unparseable strings are still the same bind.
func TestSameHostFallsBackToTextWhenAnAddressWillNotParse(t *testing.T) {
	if !sameHost("not-an-address", "not-an-address") {
		t.Error("two identical unparseable addresses were reported as different hosts, which would hide a collision")
	}
	if sameHost("not-an-address", "another-non-address") {
		t.Error("two different unparseable addresses were reported as the same host")
	}
	// The parsed path still works, including across spellings.
	if !sameHost("::1", "0:0:0:0:0:0:0:1") {
		t.Error("two spellings of the IPv6 loopback were not recognised as one address")
	}
	if sameHost("127.0.0.1", "127.0.0.2") {
		t.Error("two different loopback addresses were reported as the same host")
	}
}
