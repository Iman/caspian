package tunnel

import (
	"encoding/json"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// TestXHTTPCarriesHTTP2AndHTTP3ThroughTheALPN pins the mapping the README
// documents under "HTTP/2 and HTTP/3 are carried, under a different name".
//
// The transports spelled quic, h2, h3 and http were removed from the engine and
// are refused. What replaced them is XHTTP, which chooses its HTTP version from
// the TLS ALPN rather than from the transport name: exactly one ALPN entry of
// h3 selects HTTP/3 over QUIC, and anything else selects HTTP/2.
//
// That mapping is documented, so it needs a guard. It lives one engine upgrade
// away from silently changing, and the failure would be quiet: the link would
// still parse, still validate and still connect, over HTTP/2, while the README
// said HTTP/3 and the user believed it.
//
// WHAT THIS PROVES AND WHAT IT DOES NOT. It proves the keys survive Caspian's
// own path and that the engine accepts the result. It dials nothing, so it is
// not evidence that either version carries a byte. The distinction is the one
// this repository keeps between accepting a document and carrying traffic.
func TestXHTTPCarriesHTTP2AndHTTP3ThroughTheALPN(t *testing.T) {
	const (
		uuid = "b7f8c2a1-4d3e-4f5a-9b8c-1d2e3f4a5b6c"
		host = "front.invalid"
	)

	for _, tc := range []struct {
		name     string
		query    string
		wantALPN string
	}{
		{
			name:     "alpn h3 asks for HTTP/3, which is QUIC",
			query:    "type=xhttp&security=tls&alpn=h3&mode=stream-one&sni=" + host + "&host=" + host + "&path=%2Fx",
			wantALPN: "h3",
		},
		{
			name:     "no alpn leaves the engine on HTTP/2",
			query:    "type=xhttp&security=tls&mode=stream-one&sni=" + host + "&host=" + host + "&path=%2Fx",
			wantALPN: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := "vless://" + uuid + "@" + host + ":443?" + tc.query + "#box"

			l, err := link.Parse(raw)
			if err != nil {
				t.Fatalf("the link did not parse: %v", err)
			}

			// The engine's canonical name for XHTTP is splithttp, its former
			// name. Both spellings parse; this is the one it reports back.
			if l.Network != "splithttp" {
				t.Errorf("network is %q, want \"splithttp\". If the engine renamed the "+
					"transport, the README section documenting this mapping needs the "+
					"new name too.", l.Network)
			}
			if l.Security != "tls" {
				t.Errorf("security is %q, want \"tls\". Without TLS there is no ALPN, and "+
					"with no ALPN there is no way to ask for HTTP/3.", l.Security)
			}

			o := xcfg.Defaults()
			o.Link = l
			o.TUN.Disabled = true
			doc, err := xcfg.Build(o)
			if err != nil {
				t.Fatalf("building the engine document: %v", err)
			}
			if err := engine.Validate(doc); err != nil {
				t.Fatalf("the engine refused the document: %v", err)
			}

			// The ALPN has to reach the engine intact. internal/xcfg carries
			// the outbound as opaque JSON precisely so that keys it has never
			// heard of survive, and this is the assertion that it does.
			var seen struct {
				Outbounds []struct {
					StreamSettings struct {
						Network     string `json:"network"`
						Security    string `json:"security"`
						TLSSettings struct {
							ALPN []string `json:"alpn"`
						} `json:"tlsSettings"`
					} `json:"streamSettings"`
				} `json:"outbounds"`
			}
			if err := json.Unmarshal(doc, &seen); err != nil {
				t.Fatalf("reading back the built document: %v", err)
			}
			if len(seen.Outbounds) == 0 {
				t.Fatal("the built document has no outbound")
			}
			got := seen.Outbounds[0].StreamSettings.TLSSettings.ALPN

			switch tc.wantALPN {
			case "":
				if len(got) != 0 {
					t.Errorf("no alpn was asked for but the document carries %v", got)
				}
			default:
				if len(got) != 1 || got[0] != tc.wantALPN {
					t.Errorf("the document carries alpn %v, want exactly [%q].\n"+
						"Exactly one entry matters: the engine reads any other length as a "+
						"request for HTTP/2, so an extra entry silently downgrades the "+
						"connection rather than failing.", got, tc.wantALPN)
				}
			}

			// mode has to survive too. The default resolves to packet-up, not
			// the stream-one shape the engine names as the QUIC replacement.
			if !strings.Contains(string(doc), "stream-one") {
				t.Error("the built document lost mode=stream-one, which is the shape the " +
					"engine names as the replacement for the removed QUIC transport")
			}
		})
	}

	// The trap, pinned deliberately. A user who writes two ALPN entries hoping
	// to negotiate either version gets HTTP/2, silently: the engine reads any
	// length other than one as a request for version 2. Nothing rejects it,
	// nothing warns, and the connection works, so the only way anybody learns
	// this is by being told. The README says so, and this holds the document
	// carrying both entries so the claim stays checkable.
	t.Run("two alpn entries are carried, and mean HTTP/2 to the engine", func(t *testing.T) {
		raw := "vless://" + uuid + "@" + host + ":443?type=xhttp&security=tls&alpn=h3,h2" +
			"&mode=stream-one&sni=" + host + "&host=" + host + "&path=%2Fx#box"

		l, err := link.Parse(raw)
		if err != nil {
			t.Fatalf("the link did not parse: %v", err)
		}
		o := xcfg.Defaults()
		o.Link = l
		o.TUN.Disabled = true
		doc, err := xcfg.Build(o)
		if err != nil {
			t.Fatalf("building the engine document: %v", err)
		}
		if err := engine.Validate(doc); err != nil {
			t.Fatalf("the engine refused a two-ALPN document, which would make this a "+
				"loud failure rather than the silent one documented: %v", err)
		}

		var seen struct {
			Outbounds []struct {
				StreamSettings struct {
					TLSSettings struct {
						ALPN []string `json:"alpn"`
					} `json:"tlsSettings"`
				} `json:"streamSettings"`
			} `json:"outbounds"`
		}
		if err := json.Unmarshal(doc, &seen); err != nil {
			t.Fatalf("reading back the built document: %v", err)
		}
		if len(seen.Outbounds) == 0 {
			t.Fatal("the built document has no outbound")
		}
		if got := seen.Outbounds[0].StreamSettings.TLSSettings.ALPN; len(got) != 2 {
			t.Errorf("the document carries alpn %v, want both entries. If the count has "+
				"changed, the engine's rule for choosing the HTTP version may have changed "+
				"with it, and the README section on this needs rereading against the "+
				"engine rather than edited to match this test.", got)
		}
	})
}
