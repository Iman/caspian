// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// The tests in this file were written BEFORE ProxyConfig.SubscriptionURL,
// Quota, Store.SetSubscriptionURL and Store.RecordRefresh existed, on
// 2026-09-09. Their first run was a compile failure naming all four; that is
// the "red" recorded in the report.

// fakeSubscriptionURL is a documentation-reserved name with a token in the
// path, which is the shape providers use. Nothing here resolves.
const fakeSubscriptionURL = "https://sub.example.com/api/v1/client/subscribe?token=fake-token-not-real"

// TestSetSubscriptionURLPersistsAndTouchesNothingElse: the address survives a
// reload and the config beside it is untouched byte for byte.
func TestSetSubscriptionURLPersistsAndTouchesNothingElse(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
		t.Fatalf("SetProxyConfig: %v", err)
	}
	if err := st.SelectProxyEntry(1); err != nil {
		t.Fatalf("SelectProxyEntry: %v", err)
	}
	before := st.Proxy()

	if err := st.SetSubscriptionURL(fakeSubscriptionURL); err != nil {
		t.Fatalf("SetSubscriptionURL: %v", err)
	}
	after := st.Proxy()
	if after.SubscriptionURL.Reveal() != fakeSubscriptionURL {
		t.Error("the address was not stored")
	}
	if !after.HasSubscription() {
		t.Error("HasSubscription is false after the address was stored")
	}
	if after.Raw.Reveal() != before.Raw.Reveal() || after.Selected != before.Selected ||
		after.Scheme != before.Scheme || after.Label != before.Label || !after.AddedAt.Equal(before.AddedAt) {
		t.Error("SetSubscriptionURL changed a field other than SubscriptionURL")
	}

	again, err := Load(dir)
	if err != nil {
		t.Fatalf("Load again: %v", err)
	}
	if again.Proxy().SubscriptionURL.Reveal() != fakeSubscriptionURL {
		t.Error("the address did not survive the reload")
	}

	// Storing the address with no config is allowed: it is the case where a
	// person has the address and nothing else yet.
	fresh, err := Load(tempStateDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := fresh.SetSubscriptionURL(fakeSubscriptionURL); err != nil {
		t.Errorf("SetSubscriptionURL on a box with no config: %v", err)
	}
	if fresh.Proxy().IsConfigured() {
		t.Error("storing an address made the box look configured")
	}
}

// TestSetSubscriptionURLRefusesEachRule: every rule refuses with its own
// sentinel, the sentence names the rule, and the value never appears in it.
// The value is a credential, so an error that quoted it would be a leak on
// the one path where somebody is most likely to print the error.
func TestSetSubscriptionURLRefusesEachRule(t *testing.T) {
	long := "https://sub.example.com/" + strings.Repeat("a", maxSubscriptionURL)
	cases := []struct {
		name  string
		value string
		want  error
	}{
		{"plain http", "http://sub.example.com/sub", ErrSubscriptionNotHTTPS},
		{"no scheme", "sub.example.com/sub", ErrSubscriptionNotHTTPS},
		{"another scheme", "ftp://sub.example.com/sub", ErrSubscriptionNotHTTPS},
		{"uppercase scheme is still https", "", nil}, // handled below
		{"empty", "", ErrSubscriptionNotHTTPS},
		{"no host", "https:///sub", ErrSubscriptionNoHost},
		{"ipv4 literal", "https://192.0.2.10/sub", ErrSubscriptionIPLiteral},
		{"ipv4 literal with port", "https://192.0.2.10:8443/sub", ErrSubscriptionIPLiteral},
		{"ipv6 literal", "https://[2001:db8::1]/sub", ErrSubscriptionIPLiteral},
		{"ipv6 literal with port", "https://[2001:db8::1]:8443/sub", ErrSubscriptionIPLiteral},
		{"ipv6 literal with zone", "https://[fe80::1%25eth0]/sub", ErrSubscriptionIPLiteral},
		{"ipv4-mapped ipv6 literal", "https://[::ffff:192.0.2.10]/sub", ErrSubscriptionIPLiteral},
		{"userinfo", "https://user:pass@sub.example.com/sub", ErrSubscriptionUserinfo},
		{"user only", "https://token@sub.example.com/sub", ErrSubscriptionUserinfo},
		{"too long", long, ErrSubscriptionTooLong},
		{"unparseable", "https://sub.example.com/%zz", ErrSubscriptionMalformed},
		{"control character", "https://sub.example.com/sub\n", ErrSubscriptionMalformed},
	}
	for _, tc := range cases {
		if tc.want == nil {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			st, err := Load(tempStateDir(t))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			err = st.SetSubscriptionURL(tc.value)
			if err == nil {
				t.Fatal("the address was accepted")
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("error is %v, want %v", err, tc.want)
			}
			if tc.value != "" && strings.Contains(err.Error(), tc.value) {
				t.Errorf("the error quotes the address: %v", err)
			}
			for _, part := range []string{"example.com", "192.0.2.10", "2001:db8", "user:pass", "token@"} {
				if strings.Contains(err.Error(), part) {
					t.Errorf("the error carries part of the address (%q): %v", part, err)
				}
			}
			if st.Proxy().HasSubscription() {
				t.Error("a refused address was stored")
			}
		})
	}

	t.Run("scheme comparison is case-insensitive", func(t *testing.T) {
		st, err := Load(tempStateDir(t))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := st.SetSubscriptionURL("HTTPS://sub.example.com/sub"); err != nil {
			t.Errorf("an uppercase https scheme was refused: %v", err)
		}
	})

	t.Run("exactly the cap is accepted", func(t *testing.T) {
		st, err := Load(tempStateDir(t))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		exact := "https://sub.example.com/" + strings.Repeat("a", maxSubscriptionURL-len("https://sub.example.com/"))
		if len(exact) != maxSubscriptionURL {
			t.Fatalf("setup: built %d bytes, want %d", len(exact), maxSubscriptionURL)
		}
		if err := st.SetSubscriptionURL(exact); err != nil {
			t.Errorf("an address of exactly %d bytes was refused: %v", maxSubscriptionURL, err)
		}
	})
}

// TestRecordRefreshReplacesTheConfigAndKeepsTheAddress: a refresh has the
// semantics of a paste (new list, selection back to the first entry) plus the
// figures the provider sent, and it never touches the address it came from.
func TestRecordRefreshReplacesTheConfigAndKeepsTheAddress(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
		t.Fatalf("SetProxyConfig: %v", err)
	}
	if err := st.SelectProxyEntry(3); err != nil {
		t.Fatalf("SelectProxyEntry: %v", err)
	}
	if err := st.SetSubscriptionURL(fakeSubscriptionURL); err != nil {
		t.Fatalf("SetSubscriptionURL: %v", err)
	}

	fetched := fakeProxyLink + "\n" + fakeProxyLink
	at := time.Date(2026, 9, 9, 10, 30, 0, 0, time.UTC)
	q := Quota{Upload: 1_000, Download: 12_400_000_000, Total: 100_000_000_000, Expire: 1_800_000_000}
	if err := st.RecordRefresh(fetched, "vless", "Provider", q, at); err != nil {
		t.Fatalf("RecordRefresh: %v", err)
	}
	p := st.Proxy()
	if p.Raw.Reveal() != fetched {
		t.Error("the fetched text was not stored")
	}
	if p.Scheme != "vless" || p.Label != "Provider" {
		t.Errorf("scheme %q label %q after a refresh", p.Scheme, p.Label)
	}
	if p.Selected != 0 {
		t.Errorf("Selected = %d after a refresh, want 0: a new list has no old position", p.Selected)
	}
	if p.Quota != q {
		t.Errorf("Quota = %+v, want %+v", p.Quota, q)
	}
	if !p.RefreshedAt.Equal(at) {
		t.Errorf("RefreshedAt = %v, want %v", p.RefreshedAt, at)
	}
	if !p.AddedAt.Equal(at) {
		t.Errorf("AddedAt = %v, want the refresh time %v: the stored config is new", p.AddedAt, at)
	}
	if p.SubscriptionURL.Reveal() != fakeSubscriptionURL {
		t.Error("the address was lost by the refresh")
	}

	again, err := Load(dir)
	if err != nil {
		t.Fatalf("Load again: %v", err)
	}
	if got := again.Proxy(); got.Quota != q || !got.RefreshedAt.Equal(at) {
		t.Errorf("the quota or the refresh time did not survive the reload: %+v %v", got.Quota, got.RefreshedAt)
	}
}

// TestRecordRefreshRefusesWhatItCannotRecord: no address means nothing was
// refreshed from, and an empty body is not a configuration. Neither refusal
// changes what readers see.
func TestRecordRefreshRefusesWhatItCannotRecord(t *testing.T) {
	at := time.Date(2026, 9, 9, 10, 30, 0, 0, time.UTC)

	t.Run("no address stored", func(t *testing.T) {
		st, err := Load(tempStateDir(t))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
			t.Fatalf("SetProxyConfig: %v", err)
		}
		before := st.Proxy()
		err = st.RecordRefresh(fakeProxyLink, "vless", "", Quota{}, at)
		if err == nil {
			t.Fatal("a refresh was recorded on a box with no subscription address")
		}
		if !strings.Contains(err.Error(), "subscription address") {
			t.Errorf("the error does not name what is missing: %v", err)
		}
		if got := st.Proxy(); got.Raw != before.Raw || !got.RefreshedAt.IsZero() {
			t.Error("a refused refresh changed the published state")
		}
	})

	t.Run("empty body", func(t *testing.T) {
		st, err := Load(tempStateDir(t))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := st.SetSubscriptionURL(fakeSubscriptionURL); err != nil {
			t.Fatalf("SetSubscriptionURL: %v", err)
		}
		err = st.RecordRefresh("", "vless", "", Quota{}, at)
		if err == nil {
			t.Fatal("an empty body was recorded as a config")
		}
		if !strings.Contains(err.Error(), "must not be empty") {
			t.Errorf("the error does not say the body is empty: %v", err)
		}
		if st.Proxy().IsConfigured() {
			t.Error("a refused refresh stored a config")
		}
	})

	t.Run("negative figures are refused", func(t *testing.T) {
		st, err := Load(tempStateDir(t))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := st.SetSubscriptionURL(fakeSubscriptionURL); err != nil {
			t.Fatalf("SetSubscriptionURL: %v", err)
		}
		for _, q := range []Quota{{Upload: -1}, {Download: -1}, {Total: -1}, {Expire: -1}} {
			if err := st.RecordRefresh(fakeProxyLink, "vless", "", q, at); err == nil {
				t.Errorf("RecordRefresh accepted %+v", q)
			}
		}
	})
}

// TestSetProxyConfigKeepsTheAddressAndClearsTheFigures: a paste replaces the
// config, so the usage figures and the refresh time belong to a config that
// is gone; the address stays, because it is where the next refresh comes from.
func TestSetProxyConfigKeepsTheAddressAndClearsTheFigures(t *testing.T) {
	st, err := Load(tempStateDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetSubscriptionURL(fakeSubscriptionURL); err != nil {
		t.Fatalf("SetSubscriptionURL: %v", err)
	}
	at := time.Date(2026, 9, 9, 10, 30, 0, 0, time.UTC)
	if err := st.RecordRefresh(fakeProxyLink, "vless", "", Quota{Total: 5}, at); err != nil {
		t.Fatalf("RecordRefresh: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
		t.Fatalf("SetProxyConfig: %v", err)
	}
	p := st.Proxy()
	if p.SubscriptionURL.Reveal() != fakeSubscriptionURL {
		t.Error("a paste lost the subscription address")
	}
	if p.Quota != (Quota{}) || !p.RefreshedAt.IsZero() {
		t.Errorf("a paste kept figures that describe a config that is gone: %+v %v", p.Quota, p.RefreshedAt)
	}
}

// TestAbsentSubscriptionFieldsLoadAsZeroFromAnOlderFile: a file written before
// the fields existed has no address, no figures and no refresh time, which is
// exactly what the zero values mean. The file is carried to the current schema
// version so that a later downgrade refuses it instead of dropping the address
// on its next save.
func TestAbsentSubscriptionFieldsLoadAsZeroFromAnOlderFile(t *testing.T) {
	dir := writeStateFile(t, `{"version":2,
		"proxy":{"raw":"`+fakeProxyLink+`","scheme":"vless","selected":1},
		"advanced":{"dns_mode":"tunnel","on_tunnel_down":"block","client_ipv6":"block"}}`)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := st.Proxy()
	if p.HasSubscription() || !p.SubscriptionURL.IsZero() {
		t.Error("a v2 file loaded with a subscription address")
	}
	if p.Quota != (Quota{}) || !p.RefreshedAt.IsZero() {
		t.Errorf("a v2 file loaded with figures: %+v %v", p.Quota, p.RefreshedAt)
	}
	if p.Raw.Reveal() != fakeProxyLink || p.Selected != 1 {
		t.Error("the migration touched a field it should not have")
	}
	if got := st.Snapshot().Version; got != CurrentVersion {
		t.Errorf("version after load = %d, want %d", got, CurrentVersion)
	}
	if err := st.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	again, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if got := again.Snapshot().Version; got != CurrentVersion {
		t.Errorf("version on disk after save = %d, want %d", got, CurrentVersion)
	}
}

// TestQuotaAndRefreshTimeRoundTrip pins the JSON keys the panel and the
// Flutter client both read. A renamed key here is a client reading zeros.
func TestQuotaAndRefreshTimeRoundTrip(t *testing.T) {
	st := fullState(t)
	if st.Proxy.SubscriptionURL.IsZero() || st.Proxy.RefreshedAt.IsZero() || st.Proxy.Quota == (Quota{}) {
		t.Fatal("fullState does not set the subscription fields, so the round-trip test cannot cover them")
	}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := fmt.Sprintf("%s", b)
	for _, key := range []string{`"subscriptionUrl"`, `"refreshed_at"`, `"quota"`, `"upload"`, `"download"`, `"total"`, `"expire"`} {
		if !strings.Contains(body, key) {
			t.Errorf("the encoded state has no %s key", key)
		}
	}
}
