// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"strings"
	"testing"
)

// TestRenderHostapdGolden is a CHANGE DETECTOR, not a correctness check.
//
// Unlike the dnsmasq golden, which external_test.go puts through the real
// dnsmasq, nothing here can tell you hostapd would accept this file. hostapd
// 2.10 has no validate-and-exit flag (its options were read off the box on
// 2026-08-30: h, d, B, K, t, v, P, e, g, G, f, T, i, S; -t sets debug
// timestamps and is not a syntax check). Proving a hostapd configuration
// requires starting hostapd, which takes the radio.
//
// Treat a green run here as "the bytes are the bytes that were reviewed", and
// nothing more, until a bring-up on hardware. See the note above the -update
// flag in golden_test.go.
func TestRenderHostapdGolden(t *testing.T) {
	got, err := RenderHostapd(testAP())
	if err != nil {
		t.Fatalf("RenderHostapd: %v", err)
	}
	assertGolden(t, "hostapd.golden", got)
}

// TestRenderHostapdNeverEmitsTKIP is the regression test for the defect at
// 004-hotspot/install.sh:428, which shipped wpa_pairwise=TKIP on every box.
//
// It asserts on the absence of both the cipher and the directive, because
// wpa_pairwise with any value at all enables the WPA1 cipher suite.
func TestRenderHostapdNeverEmitsTKIP(t *testing.T) {
	for _, band := range []struct {
		band Band
		ch   int
	}{{Band2GHz, 10}, {Band5GHz, 36}} {
		cfg := testAP()
		cfg.Band = band.band
		cfg.Channel = band.ch

		got, err := RenderHostapd(cfg)
		if err != nil {
			t.Fatalf("RenderHostapd(%s): %v", band.band, err)
		}
		for _, forbidden := range []string{"TKIP", "WEP", "wpa=1", "wpa=3"} {
			if containsDirective(got, forbidden) {
				t.Errorf("%s appears in the generated hostapd configuration for %s", forbidden, band.band)
			}
		}
		if !strings.Contains(got, "\nrsn_pairwise=CCMP\n") {
			t.Errorf("rsn_pairwise=CCMP is missing for %s", band.band)
		}
	}
}

// containsDirective looks for a string outside the comment lines, so that a
// comment naming TKIP as the thing not to do does not fail the test that TKIP
// is not configured.
func containsDirective(conf, needle string) bool {
	for _, line := range strings.Split(conf, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.Contains(line, needle) {
			return true
		}
	}
	return false
}

func TestRenderHostapdRequiredDirectives(t *testing.T) {
	got, err := RenderHostapd(testAP())
	if err != nil {
		t.Fatalf("RenderHostapd: %v", err)
	}
	for _, want := range []string{
		"country_code=GB", // omitting it is the top cause of the AP never starting
		"ieee80211d=1",
		"ieee80211n=1",  // without it the radio stays at 54 Mbps
		"wmm_enabled=1", // without it ieee80211n is silently ineffective
		"ap_isolate=1",  // joined clients must not reach each other
		"rsn_pairwise=CCMP",
		"wpa=2",
		"auth_algs=1",
		"channel=10",
		"interface=wlan0",
	} {
		if !containsDirective(got, want) {
			t.Errorf("%q is missing from the generated hostapd configuration", want)
		}
	}
}

func TestPassphraseTooShortIsRefused(t *testing.T) {
	cfg := testAP()
	cfg.Passphrase = "short12" // 7 characters, one below the WPA2 minimum

	if _, err := RenderHostapd(cfg); err == nil {
		t.Fatal("a 7 character passphrase was accepted; WPA2 needs at least 8")
	} else if !strings.Contains(err.Error(), "at least 8") {
		t.Errorf("the error does not say what the minimum is: %v", err)
	}

	cfg.Passphrase = "12345678" // exactly 8, but a well-known default
	if err := ValidatePassphrase("abcdefgh"); err != nil {
		t.Errorf("an 8 character passphrase should be accepted: %v", err)
	}
	if err := ValidatePassphrase(strings.Repeat("a", 64)); err == nil {
		t.Error("a 64 character passphrase was accepted; the maximum is 63")
	}
	if err := ValidatePassphrase(""); err == nil {
		t.Error("an empty passphrase was accepted")
	}
}

// TestFixedDefaultPassphraseIsRefused names the failure: every box the
// reference implementation built shipped with the same WPA2 key
// (004-hotspot/install.sh:48).
func TestFixedDefaultPassphraseIsRefused(t *testing.T) {
	for _, p := range []string{"SecurePass123", "securepass123", "SECUREPASS123", "password", "raspberry"} {
		if err := ValidatePassphrase(p); err == nil {
			t.Errorf("the well-known default %q was accepted", p)
		}
	}
}

func TestPassphraseRejectsConfigInjection(t *testing.T) {
	// A newline in a value would end the wpa_passphrase line and let the rest
	// be read by hostapd, running as root, as further directives.
	for _, p := range []string{
		"goodpass\nap_isolate=0",
		"goodpass\r\nssid=evil",
		"goodpass\x00more",
	} {
		if err := ValidatePassphrase(p); err == nil {
			t.Errorf("a passphrase containing a control character was accepted: %q", p)
		}
	}
	cfg := testAP()
	cfg.SSID = "Caspian\nap_isolate=0"
	if _, err := RenderHostapd(cfg); err == nil {
		t.Error("an SSID containing a newline was accepted")
	}
	cfg = testAP()
	cfg.Interface = "wlan0\nssid=evil"
	if _, err := RenderHostapd(cfg); err == nil {
		t.Error("an interface name containing a newline was accepted")
	}
}

func TestGeneratedPassphrase(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		p, err := GeneratePassphrase()
		if err != nil {
			t.Fatalf("GeneratePassphrase: %v", err)
		}
		if len(p) != GeneratedPassphraseLen {
			t.Fatalf("generated passphrase is %d characters, want %d", len(p), GeneratedPassphraseLen)
		}
		if err := ValidatePassphrase(p); err != nil {
			t.Fatalf("a generated passphrase failed validation: %v", err)
		}
		if seen[p] {
			t.Fatalf("GeneratePassphrase repeated %q within 200 calls", p)
		}
		seen[p] = true
		for _, c := range p {
			if !strings.ContainsRune(passphraseAlphabet, c) {
				t.Fatalf("generated passphrase contains %q, which is outside the alphabet", c)
			}
		}
	}
}

func TestEnsurePassphraseGeneratesWhenAbsent(t *testing.T) {
	cfg := testAP()
	cfg.Passphrase = ""

	out, generated, err := EnsurePassphrase(cfg)
	if err != nil {
		t.Fatalf("EnsurePassphrase: %v", err)
	}
	if !generated {
		t.Error("EnsurePassphrase did not report that it generated a passphrase")
	}
	if out.Passphrase == "" {
		t.Fatal("EnsurePassphrase returned an empty passphrase")
	}
	// The caller has to be able to show it to the user, since nobody else
	// knows it.
	if err := ValidatePassphrase(out.Passphrase); err != nil {
		t.Errorf("the generated passphrase is not valid: %v", err)
	}

	cfg.Passphrase = "a-supplied-passphrase"
	out, generated, err = EnsurePassphrase(cfg)
	if err != nil {
		t.Fatalf("EnsurePassphrase: %v", err)
	}
	if generated {
		t.Error("EnsurePassphrase replaced a passphrase the caller supplied")
	}
	if out.Passphrase != "a-supplied-passphrase" {
		t.Errorf("supplied passphrase changed to %q", out.Passphrase)
	}
}

func TestCountryCodeIsRequired(t *testing.T) {
	cfg := testAP()
	cfg.CountryCode = ""
	if _, err := RenderHostapd(cfg); err == nil {
		t.Fatal("a configuration with no country code was accepted")
	}
	for _, bad := range []string{"G", "GBR", "gb", "G1"} {
		cfg.CountryCode = bad
		if _, err := RenderHostapd(cfg); err == nil {
			t.Errorf("country code %q was accepted", bad)
		}
	}
}

func TestChannelValidation(t *testing.T) {
	tests := []struct {
		band Band
		ch   int
		ok   bool
	}{
		{Band2GHz, 1, true},
		{Band2GHz, 10, true},
		{Band2GHz, 13, true},
		{Band2GHz, 0, false},
		{Band2GHz, 14, false}, // Japan only, 802.11b only
		{Band2GHz, 36, false},
		{Band5GHz, 36, true},
		{Band5GHz, 149, true},
		{Band5GHz, 52, false}, // radar detection channel, excluded on purpose
		{Band5GHz, 10, false},
	}
	for _, tc := range tests {
		cfg := testAP()
		cfg.Band = tc.band
		cfg.Channel = tc.ch
		_, err := RenderHostapd(cfg)
		if tc.ok && err != nil {
			t.Errorf("channel %d on %s was refused: %v", tc.ch, tc.band, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("channel %d on %s was accepted", tc.ch, tc.band)
		}
	}
}

func TestSSIDLengthIsInOctets(t *testing.T) {
	cfg := testAP()
	cfg.SSID = strings.Repeat("a", 32)
	if _, err := RenderHostapd(cfg); err != nil {
		t.Errorf("a 32 octet SSID was refused: %v", err)
	}
	cfg.SSID = strings.Repeat("a", 33)
	if _, err := RenderHostapd(cfg); err == nil {
		t.Error("a 33 octet SSID was accepted")
	}
	// 17 Persian characters are 34 octets, so a name that looks short is not.
	cfg.SSID = strings.Repeat("ش", 17)
	if _, err := RenderHostapd(cfg); err == nil {
		t.Error("a 34 octet SSID was accepted")
	}
	cfg.SSID = strings.Repeat("ش", 16)
	if _, err := RenderHostapd(cfg); err != nil {
		t.Errorf("a 32 octet non-Latin SSID was refused: %v", err)
	}
}
