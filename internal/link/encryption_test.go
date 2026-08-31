// SPDX-License-Identifier: AGPL-3.0-or-later

package link_test

import (
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/link"
)

// VLESS Encryption must survive the trip from the share link to the engine
// document, and this is the guard that says so.
//
// It matters more than most guards here. VLESS itself carries no encryption of
// its own; confidentiality normally comes from the layer underneath, REALITY or
// TLS. A link written with "security=none" therefore looks like plaintext, and
// it IS plaintext unless it also carries an "encryption=" parameter, which is
// the ML-KEM-768 and X25519 hybrid that VLESS Encryption adds at the protocol
// layer instead.
//
// This package does not rebuild the outbound field by field. It re-serialises
// the structure the vendored parser produced, and the protocol settings ride
// along as an opaque blob. That is why the parameter survives, and it is also
// why nothing would fail if it stopped surviving: no field would be missing, no
// type would change, and no test would notice. The tunnel would simply carry a
// user's traffic in the clear while every other check stayed green.
//
// MEASURED 2026-08-30 against the real configurations: a websocket link with
// security=none produced a document containing the mlkem768 parameter, and the
// REALITY and TLS links produced "encryption":"none", which is correct for them
// because their transport is already encrypted.
func TestVLESSEncryptionSurvivesIntoTheEngineDocument(t *testing.T) {
	// A websocket link with NO transport security, which is the shape that
	// depends on VLESS Encryption entirely. The key material is invented.
	const withEncryption = "vless://11111111-2222-3333-4444-555555555555@example.org:443" +
		"?type=ws&security=none&path=%2Fws&host=example.org" +
		"&encryption=mlkem768x25519plus.random.1rtt.100-111-1111.75-0-111.50-0-3333." +
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA#sample"

	l, err := link.Parse(withEncryption)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	got := string(doc)

	if !strings.Contains(got, "mlkem768x25519plus") {
		t.Error("the VLESS Encryption parameter did not reach the engine document. " +
			"This link has security=none, so that parameter is the ONLY thing encrypting " +
			"the user's traffic. Dropping it does not fail, does not warn, and does not " +
			"change the shape of the document: it silently sends everything in the clear.")
	}
	if strings.Contains(got, `"encryption":"none"`) {
		t.Error(`the document says "encryption":"none" for a link that asked for ` +
			"VLESS Encryption, so the request was replaced with its opposite")
	}

	// The other direction, so this test cannot pass by always finding the
	// string somewhere: a link that asks for nothing must not gain it.
	const withoutEncryption = "vless://11111111-2222-3333-4444-555555555555@example.org:443" +
		"?type=ws&security=tls&path=%2Fws&host=example.org#sample"

	l2, err := link.Parse(withoutEncryption)
	if err != nil {
		t.Fatalf("parse plain: %v", err)
	}
	doc2, err := l2.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig plain: %v", err)
	}
	if strings.Contains(string(doc2), "mlkem768") {
		t.Error("a link that asked for no VLESS Encryption came out of the composer with some")
	}
}
