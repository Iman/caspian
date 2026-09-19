// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"strings"
	"testing"
)

// TestAnHTMLEscapedLinkParsesLikeThePlainOne covers links copied from a web
// page or a Telegram post, where every "&" between parameters arrives as
// "&amp;". Before 2026-09-13 such a link parsed, because the scheme and the
// first parameter were intact, but every later parameter was lost under a name
// like "amp;sni", and the link then failed against a server that was up. A
// survey of 8,600 public links found 88 in that shape.
func TestAnHTMLEscapedLinkParsesLikeThePlainOne(t *testing.T) {
	for _, raw := range []string{vlessTLSWebsocketLink(), vlessRealityLink(), trojanLink(), hysteria2Link()} {
		if !strings.Contains(raw, "&") {
			t.Fatal("fixture has no parameter separator to escape")
		}
		plain, err := Parse(raw)
		if err != nil {
			t.Fatalf("the plain fixture does not parse: %v", err)
		}
		escaped, err := Parse(strings.ReplaceAll(raw, "&", "&amp;"))
		if err != nil {
			t.Fatalf("the HTML-escaped form does not parse: %v", err)
		}
		if escaped.Protocol != plain.Protocol || escaped.Network != plain.Network || escaped.Security != plain.Security ||
			escaped.Address != plain.Address || escaped.Port != plain.Port || escaped.ServerName != plain.ServerName {
			t.Errorf("the escaped link parsed differently:\n plain   %s/%s/%s %s:%d sni=%s\n escaped %s/%s/%s %s:%d sni=%s",
				plain.Protocol, plain.Network, plain.Security, plain.Address, plain.Port, plain.ServerName,
				escaped.Protocol, escaped.Network, escaped.Security, escaped.Address, escaped.Port, escaped.ServerName)
		}
		a, err := plain.XrayConfig()
		if err != nil {
			t.Fatal(err)
		}
		b, err := escaped.XrayConfig()
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("the escaped link builds a different engine document than the plain one")
		}
	}

	// The same through the list path, which is what a pasted page of links
	// goes through.
	list, err := ParseAll(strings.ReplaceAll(vlessTLSWebsocketLink(), "&", "&amp;") + "\n" + vmessBase64Link())
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	if len(list.Entries) != 2 || list.Dropped != 0 {
		t.Errorf("ParseAll saw %d entries and dropped %d, want 2 and 0", len(list.Entries), list.Dropped)
	}
}
