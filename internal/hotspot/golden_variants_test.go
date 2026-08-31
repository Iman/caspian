// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package hotspot

// The generated hostapd and dnsmasq configurations, one file per setting that
// changes them.
//
// WHAT WAS ALREADY HERE, AND WHAT WAS MISSING. golden_test.go, hostapd_test.go
// and dnsmasq_test.go pin ONE arrangement each: testAP() and testDNS(), the
// default. That answers "did the default output change" and nothing else. Every
// advanced setting the panel exposes reaches one of these two files, and until
// this file existed a change to how any of them renders was invisible unless it
// happened to also move the default.
//
// The distinction matters most for hostapd, for the reason golden_test.go
// already states at length: hostapd has no validate-and-exit flag, so a hostapd
// configuration is proven only by a bring-up on hardware. That makes these files
// change detectors and nothing more. A green run here says the bytes have not
// moved. It does not say hostapd would accept them, and on a developer machine
// nothing has checked that at all.
//
// dnsmasq is different and better off: TestGoldenDnsmasqConfigIsAcceptedByDnsmasq
// in external_test.go puts the committed default file through the real dnsmasq
// where one is installed. TestGoldenVariants_DnsmasqFilesAreAcceptedByDnsmasq
// below extends that to every variant this file adds, so the variants are not a
// weaker class of evidence than the default they vary from.
//
// Regenerate every golden in the repository with one command:
//
//	bash scripts/golden-update.sh
//
// or this package alone:
//
//	go test ./internal/hotspot -run Golden -update
//
// then READ THE DIFF.

import (
	"crypto/sha256"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// variantPassphrase is the WPA passphrase every variant renders.
//
// It is deliberately NOT testAP()'s "correct-horse-battery". That value is also
// panel_test.go's testPassword, so a scanner that found it could not tell which
// fixture it came from. This one occurs nowhere else in the repository, which is
// what lets test/goldenscan report a hit with no ambiguity. It is not, and has
// never been, a working credential.
const variantPassphrase = "sentinelwpa-hotspot-9d4"

// variantAP is the reference access point for this file: testAP with the
// sentinel passphrase.
func variantAP() APConfig {
	c := testAP()
	c.Passphrase = variantPassphrase
	return c
}

// apVariant is one hostapd configuration worth freezing.
type apVariant struct {
	// file is the golden's name in testdata.
	file string

	// setting names the advanced setting this variant moves, in the words the
	// panel uses, so the file list reads as the settings list.
	setting string

	// mutate moves exactly that setting off its default.
	mutate func(*APConfig)
}

// apVariants is every advanced setting that reaches the hostapd file.
//
// The set is derived from state.Advanced, whose fields are the settings the
// panel exposes. Of those, InternetInterface, DNSMode, OnTunnelDown,
// EngineLogLevel and PanelOnLAN reach netcfg, xcfg or the panel's own listener
// and never this file; Subnet reaches dnsmasq only. What is left is the four
// below plus the interface name, and
// TestGoldenVariants_CoverEveryAdvancedSettingThatReachesHostapd holds that
// reasoning in place rather than leaving it as a claim in this comment.
func apVariants() []apVariant {
	return []apVariant{
		{
			file:    "hostapd-default.golden",
			setting: "(none: the default arrangement)",
			mutate:  func(*APConfig) {},
		},
		{
			file:    "hostapd-band-5ghz.golden",
			setting: "Band",
			mutate: func(c *APConfig) {
				c.Band = Band5GHz
				// A 2.4 GHz channel number is not valid on 5 GHz, so the band
				// cannot be moved on its own. Channel 36 is the lowest UNII-1
				// channel and is allowed in every regulatory domain that
				// allows 5 GHz at all.
				c.Channel = 36
			},
		},
		{
			file:    "hostapd-channel.golden",
			setting: "Channel",
			mutate:  func(c *APConfig) { c.Channel = 1 },
		},
		{
			file:    "hostapd-country.golden",
			setting: "Country",
			mutate:  func(c *APConfig) { c.CountryCode = "IR" },
		},
		{
			file:    "hostapd-interface.golden",
			setting: "HotspotInterface",
			mutate:  func(c *APConfig) { c.Interface = "ap0" },
		},
		{
			file:    "hostapd-utf8-ssid.golden",
			setting: "(not a setting: a non-Latin network name)",
			// An SSID is 32 OCTETS, not 32 characters, and a Persian name
			// reaches the limit in roughly sixteen. This is the product's
			// primary audience typing its own language into the field, and it
			// is pinned so that a change to utf8_ssid or to the escaping shows
			// up rather than arriving on a user's box.
			mutate: func(c *APConfig) { c.SSID = "کاسپین" },
		},
		{
			file:    "hostapd-control-dir.golden",
			setting: "(not a panel setting: the control socket path)",
			mutate:  func(c *APConfig) { c.ControlDir = "/run/caspian/hostapd" },
		},
	}
}

// redactPassphrase replaces the value on the wpa_passphrase line with a digest
// of itself.
//
// WHY A GOLDEN OF A HOSTAPD FILE MUST NOT CARRY THE PASSPHRASE. The line is the
// WiFi key in the clear, and a golden is committed, which makes it permanent:
// once it reaches git history, deleting it from the working tree deletes
// nothing. The value here is an invented fixture, so nothing is disclosed
// today. What is at stake is the PATTERN. A committed file with a real-looking
// wpa_passphrase line in it normalises the shape, and the day somebody runs
// -update with a configuration built from a live box, the key lands in a commit
// and nobody notices, because the diff looks exactly like every previous one.
//
// The digest keeps the property that matters: a change to the passphrase is
// still a diff, so the golden still detects a change to what the appliance
// broadcasts. It just does not carry the key. This is the same trade the panel
// layer makes with the join QR.
//
// testdata/hostapd.golden, which predates this file, still carries its fixture
// passphrase in the clear. That is recorded in testdata/PROVENANCE.md and
// allowlisted by name in test/goldenscan rather than changed here, because that
// file is the one the external-evidence record in external_test.go is written
// against and rewriting it would invalidate that record.
func redactPassphrase(s string) string {
	const key = "wpa_passphrase="
	out := make([]string, 0, 128)
	for _, line := range strings.Split(s, "\n") {
		if v, ok := strings.CutPrefix(line, key); ok && v != "" {
			sum := sha256.Sum256([]byte(v))
			line = fmt.Sprintf("%s<redacted:wpa-passphrase sha256=%x len=%d>", key, sum[:8], len(v))
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func TestGoldenVariants_Hostapd(t *testing.T) {
	for _, v := range apVariants() {
		t.Run(v.file, func(t *testing.T) {
			cfg := variantAP()
			v.mutate(&cfg)
			got, err := RenderHostapd(cfg)
			if err != nil {
				t.Fatalf("RenderHostapd: %v", err)
			}
			assertGolden(t, v.file, redactPassphrase(got))
		})
	}
}

// TestGoldenVariants_RedactionStillDetectsAPassphraseChange proves the digest
// is not a constant.
//
// A redactor that replaced the value with a fixed placeholder would make every
// passphrase produce the same golden, and a change to what the appliance
// broadcasts would stop being a diff. That is the failure mode of redaction
// done badly: the file looks safe and detects nothing.
func TestGoldenVariants_RedactionStillDetectsAPassphraseChange(t *testing.T) {
	a := variantAP()
	b := variantAP()
	b.Passphrase = variantPassphrase + "-changed"

	ra, err := RenderHostapd(a)
	if err != nil {
		t.Fatalf("RenderHostapd: %v", err)
	}
	rb, err := RenderHostapd(b)
	if err != nil {
		t.Fatalf("RenderHostapd: %v", err)
	}
	if redactPassphrase(ra) == redactPassphrase(rb) {
		t.Error("two different passphrases redact to the same bytes, so the golden would no " +
			"longer detect a change to what the appliance broadcasts")
	}
	if strings.Contains(redactPassphrase(ra), variantPassphrase) {
		t.Error("the redactor left the passphrase in its output")
	}
}

// dnsVariant is one dnsmasq configuration worth freezing.
type dnsVariant struct {
	file    string
	setting string
	mutate  func(*DNSConfig)
}

// dnsVariants is every setting that reaches the dnsmasq file.
func dnsVariants() []dnsVariant {
	return []dnsVariant{
		{
			file:    "dnsmasq-default.golden",
			setting: "(none: the default arrangement)",
			mutate:  func(*DNSConfig) {},
		},
		{
			file:    "dnsmasq-subnet.golden",
			setting: "Subnet",
			// Every address in the file moves together: a subnet override that
			// changed the listen address and not the DHCP pool would be a
			// hotspot that hands out addresses nobody can route.
			mutate: func(c *DNSConfig) {
				c.Subnet = netip.MustParsePrefix("10.62.0.0/24")
				c.Gateway = netip.MustParseAddr("10.62.0.1")
				c.RangeStart = netip.MustParseAddr("10.62.0.50")
				c.RangeEnd = netip.MustParseAddr("10.62.0.150")
			},
		},
		{
			file:    "dnsmasq-interface.golden",
			setting: "HotspotInterface",
			mutate:  func(c *DNSConfig) { c.Interface = "ap0" },
		},
		{
			file:    "dnsmasq-no-cache.golden",
			setting: "(not a panel setting: CacheSize 0 disables the cache)",
			mutate:  func(c *DNSConfig) { c.CacheSize = 0 },
		},
		{
			file:    "dnsmasq-filter-aaaa.golden",
			setting: "(not a panel setting: FilterAAAA)",
			// Off by default because filter-AAAA is a dnsmasq 2.81 addition and
			// an older dnsmasq treats an unknown option as FATAL, so the
			// hotspot would not start at all. Pinned because the difference
			// between "no IPv6 answers" and "no hotspot" is one line in this
			// file.
			mutate: func(c *DNSConfig) { c.FilterAAAA = true },
		},
		{
			file:    "dnsmasq-lease-time.golden",
			setting: "(not a panel setting: LeaseTime)",
			mutate:  func(c *DNSConfig) { c.LeaseTime = 1 * time.Hour },
		},
		{
			file:    "dnsmasq-service-account.golden",
			setting: "(not a panel setting: the account dnsmasq drops to)",
			mutate: func(c *DNSConfig) {
				c.ServiceUser = "nobody"
				c.ServiceGroup = "nogroup"
			},
		},
		{
			file:    "dnsmasq-upstream-port.golden",
			setting: "(not a panel setting: the local resolver this box runs)",
			mutate:  func(c *DNSConfig) { c.Upstream = netip.MustParseAddrPort("127.0.0.1:15353") },
		},
	}
}

func TestGoldenVariants_Dnsmasq(t *testing.T) {
	for _, v := range dnsVariants() {
		t.Run(v.file, func(t *testing.T) {
			cfg := testDNS()
			v.mutate(&cfg)
			got, err := RenderDnsmasq(cfg)
			if err != nil {
				t.Fatalf("RenderDnsmasq: %v", err)
			}
			assertGolden(t, v.file, got)
		})
	}
}

// TestGoldenVariants_DnsmasqFilesAreAcceptedByDnsmasq puts every COMMITTED
// dnsmasq variant through the real dnsmasq.
//
// It reads the files rather than re-rendering, for the reason
// TestGoldenDnsmasqConfigIsAcceptedByDnsmasq gives: a golden records what this
// package produced when somebody last ran -update, and an upgrade of the
// consuming program can invalidate that file without this package changing at
// all. Then every golden test stays green and the hotspot does not come up.
//
// SKIPS ON A DEVELOPER MAC, which is where this is usually run. A skip is not a
// pass: after a green run on darwin, NOTHING has checked these files against
// dnsmasq. requireDnsmasq says so in its skip message and this comment says so
// here, because a reader who assumes otherwise has assumed the opposite of the
// truth.
func TestGoldenVariants_DnsmasqFilesAreAcceptedByDnsmasq(t *testing.T) {
	bin := requireDnsmasq(t)
	names, err := filepath.Glob(filepath.Join("testdata", "dnsmasq-*.golden"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no dnsmasq variant goldens found, so this check would pass vacuously")
	}
	for _, n := range names {
		assertDnsmasqAccepts(t, bin, n, n)
	}
	t.Logf("dnsmasq accepted %d committed variant goldens", len(names))
}

// TestGoldenVariants_EveryVariantActuallyDiffersFromTheDefault is the guard
// that stops a variant from being decorative.
//
// A variant whose output equals the default's proves nothing and reads in a
// listing as though it proves something. That is not hypothetical: the band
// override alone would have produced a file identical to the default if
// RenderHostapd ignored Band, and the identical file would have sat there
// looking like coverage.
func TestGoldenVariants_EveryVariantActuallyDiffersFromTheDefault(t *testing.T) {
	base, err := RenderHostapd(variantAP())
	if err != nil {
		t.Fatalf("RenderHostapd: %v", err)
	}
	for _, v := range apVariants() {
		if strings.HasSuffix(v.file, "-default.golden") {
			continue
		}
		cfg := variantAP()
		v.mutate(&cfg)
		got, err := RenderHostapd(cfg)
		if err != nil {
			t.Fatalf("RenderHostapd(%s): %v", v.file, err)
		}
		if got == base {
			t.Errorf("%s: moving %s produced a hostapd file byte-identical to the default, so "+
				"either the setting does not reach hostapd or the renderer is ignoring it. "+
				"Either way this golden is not covering anything", v.file, v.setting)
		}
	}

	dbase, err := RenderDnsmasq(testDNS())
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	for _, v := range dnsVariants() {
		if strings.HasSuffix(v.file, "-default.golden") {
			continue
		}
		cfg := testDNS()
		v.mutate(&cfg)
		got, err := RenderDnsmasq(cfg)
		if err != nil {
			t.Fatalf("RenderDnsmasq(%s): %v", v.file, err)
		}
		if got == dbase {
			t.Errorf("%s: moving %s produced a dnsmasq file byte-identical to the default, so "+
				"either the setting does not reach dnsmasq or the renderer is ignoring it",
				v.file, v.setting)
		}
	}
}

// TestGoldenVariants_DefaultVariantMatchesTheExistingGolden ties this file to
// the one that was already here.
//
// hostapd-default.golden and testdata/hostapd.golden are the same configuration
// apart from the passphrase, and hostapd.golden is the file external evidence
// is recorded against. If the two ever diverge for any other reason, this file's
// whole variant set is varying from a different baseline than the one the rest
// of the package pins, and every diff in it would be misread.
func TestGoldenVariants_DefaultVariantMatchesTheExistingGolden(t *testing.T) {
	for _, c := range []struct {
		name         string
		variant, old string
		render       func() (string, error)
		oldRender    func() (string, error)
	}{
		{
			name: "hostapd", variant: "hostapd-default.golden", old: "hostapd.golden",
			render:    func() (string, error) { return RenderHostapd(variantAP()) },
			oldRender: func() (string, error) { return RenderHostapd(testAP()) },
		},
		{
			name: "dnsmasq", variant: "dnsmasq-default.golden", old: "dnsmasq.golden",
			render:    func() (string, error) { return RenderDnsmasq(testDNS()) },
			oldRender: func() (string, error) { return RenderDnsmasq(testDNS()) },
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.render()
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			old, err := c.oldRender()
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			// Compare with the only intended difference normalised away.
			gotNorm := strings.ReplaceAll(got, variantPassphrase, "<passphrase>")
			oldNorm := strings.ReplaceAll(old, testAP().Passphrase, "<passphrase>")
			if gotNorm != oldNorm {
				t.Errorf("%s and %s differ by more than the passphrase, so the variants in this "+
					"file no longer vary from the configuration the rest of the package pins",
					c.variant, c.old)
			}
		})
	}
}

// TestGoldenVariants_NoGoldenCarriesTheSentinelPassphrase.
//
// The renderer necessarily writes the passphrase into the hostapd file, which is
// the whole point of a wpa_passphrase line, so the sentinel DOES land in these
// goldens. That is a deliberate, documented exception recorded in
// testdata/PROVENANCE.md and allowlisted by name in test/goldenscan.
//
// This test exists to keep the exception narrow: the sentinel may appear in the
// hostapd goldens and in NO dnsmasq golden. A dnsmasq configuration that started
// carrying the WPA passphrase would be a real leak into a file that is
// world-readable on the box.
func TestGoldenVariants_NoGoldenCarriesTheSentinelPassphrase(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "*.golden"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no goldens found, so this guard passed vacuously")
	}
	for _, n := range names {
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatalf("reading %s: %v", n, err)
		}
		if strings.Contains(string(b), variantPassphrase) {
			t.Errorf("%s carries the sentinel WPA passphrase in the clear", n)
		}
		// The pre-existing hostapd.golden carries testAP()'s passphrase and is
		// excluded by name rather than by pattern, so that a NEW file cannot
		// inherit the exception by being named similarly. See the note on
		// redactPassphrase and testdata/PROVENANCE.md.
		if filepath.Base(n) == "hostapd.golden" {
			continue
		}
		if strings.Contains(string(b), testAP().Passphrase) {
			t.Errorf("%s carries the default fixture passphrase in the clear; only the "+
				"pre-existing testdata/hostapd.golden is allowed to", n)
		}
	}
	t.Logf("scanned %d goldens", len(names))
}

// TestGoldenVariants_CoverEveryAdvancedSettingThatReachesHostapd turns the
// reasoning in the apVariants comment into a check.
//
// A comment claiming a set is complete is worth nothing the moment a field is
// added. This asserts that every field of APConfig is moved by some variant, so
// a new field arrives as a failure naming itself rather than as silent
// under-coverage. Passphrase is excluded because every variant already carries a
// non-default one, and there is nothing to learn from a golden that differs only
// in a redacted line.
func TestGoldenVariants_CoverEveryAdvancedSettingThatReachesHostapd(t *testing.T) {
	base := variantAP()
	moved := map[string]bool{}
	for _, v := range apVariants() {
		cfg := variantAP()
		v.mutate(&cfg)
		if cfg.Interface != base.Interface {
			moved["Interface"] = true
		}
		if cfg.SSID != base.SSID {
			moved["SSID"] = true
		}
		if cfg.CountryCode != base.CountryCode {
			moved["CountryCode"] = true
		}
		if cfg.Channel != base.Channel {
			moved["Channel"] = true
		}
		if cfg.Band != base.Band {
			moved["Band"] = true
		}
		if cfg.ControlDir != base.ControlDir {
			moved["ControlDir"] = true
		}
	}
	for _, field := range []string{"Interface", "SSID", "CountryCode", "Channel", "Band", "ControlDir"} {
		if !moved[field] {
			t.Errorf("no variant moves APConfig.%s off its default, so no golden covers what that "+
				"field does to the hostapd file", field)
		}
	}
}

// TestGoldenVariants_CoverEveryFieldThatReachesDnsmasq is the same guard for
// the other file.
func TestGoldenVariants_CoverEveryFieldThatReachesDnsmasq(t *testing.T) {
	base := testDNS()
	moved := map[string]bool{}
	for _, v := range dnsVariants() {
		cfg := testDNS()
		v.mutate(&cfg)
		if cfg.Interface != base.Interface {
			moved["Interface"] = true
		}
		if cfg.Subnet != base.Subnet {
			moved["Subnet"] = true
		}
		if cfg.Gateway != base.Gateway {
			moved["Gateway"] = true
		}
		if cfg.RangeStart != base.RangeStart || cfg.RangeEnd != base.RangeEnd {
			moved["Range"] = true
		}
		if cfg.LeaseTime != base.LeaseTime {
			moved["LeaseTime"] = true
		}
		if cfg.Upstream != base.Upstream {
			moved["Upstream"] = true
		}
		if cfg.CacheSize != base.CacheSize {
			moved["CacheSize"] = true
		}
		if cfg.FilterAAAA != base.FilterAAAA {
			moved["FilterAAAA"] = true
		}
		if cfg.ServiceUser != base.ServiceUser || cfg.ServiceGroup != base.ServiceGroup {
			moved["ServiceAccount"] = true
		}
	}
	for _, field := range []string{
		"Interface", "Subnet", "Gateway", "Range", "LeaseTime",
		"Upstream", "CacheSize", "FilterAAAA", "ServiceAccount",
	} {
		if !moved[field] {
			t.Errorf("no variant moves DNSConfig.%s off its default, so no golden covers what that "+
				"field does to the dnsmasq file", field)
		}
	}
	// LeaseFile is deliberately absent from the list above: it is a path chosen
	// by the installer and not a setting, and moving it would only change one
	// literal. Stated here so its absence reads as a decision rather than an
	// omission.
}
