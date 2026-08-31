// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"encoding/json"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/engine"
	share "caspianbyoc.org/caspian/third_party/libxray-share"
	"github.com/xtls/xray-core/infra/conf"
)

// These tests run the emitted document through the engine's own loader rather
// than through conf.Config.Build directly.
//
// The difference matters. engine.Validate calls infra/conf/serial.LoadJSONConfig
// (internal/engine/engine.go:323-326), which is the exact path a config takes on
// its way into a running engine, and the engine package blank-imports
// main/distro/all so the protocol and transport registrations are the real ones.
// A Build call inside this package proves the structs are consistent; this
// proves the document is one the appliance will actually load.

func TestEmittedConfigPassesEngineValidate(t *testing.T) {
	cases := map[string]string{
		"vless_reality":      vlessRealityLink(),
		"vless_tls_ws":       vlessTLSWebsocketLink(),
		"vmess_base64":       vmessBase64Link(),
		"shadowsocks_sip002": shadowsocksSIP002Link(),
		"trojan":             trojanLink(),
		"trojan_no_query":    "trojan://" + fakePassword + "@" + fakeHost + ":443#Minimal",
		"hysteria2":          hysteria2Link(),
		"hysteria2_no_query": "hysteria2://" + fakeAuth + "@" + fakeHost + ":443#NoSNI",
		// The gRPC-to-an-IP case: the one combination where a single transport
		// fills the server name for some addresses and not others
		// (grpc/dial.go:139-142).
		"vless_grpc_ip_no_sni": "vless://" + fakeUUID + "@203.0.113.9:443?security=tls&type=grpc&serviceName=gun",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			l := mustParse(t, raw)
			b, err := l.XrayConfig()
			if err != nil {
				t.Fatalf("XrayConfig: %v", err)
			}
			if err := engine.Validate(b); err != nil {
				t.Fatalf("engine.Validate rejected the emitted config: %v", err)
			}
		})
	}
}

// TestEngineValidateRejectsTheUnfixedDocument is the other half: it shows the
// engine refusing the same link at each stage this package corrects, so the
// test above is known to be capable of failing.
func TestEngineValidateRejectsTheUnfixedDocument(t *testing.T) {
	cfg, err := share.ConvertShareLinksToXrayJson(vlessRealityLink())
	if err != nil {
		t.Fatalf("vendored parser: %v", err)
	}

	// Stage one: straight out of the parser. SendThrough holds the #fragment.
	raw, err := json.Marshal(xrayConfig{Outbounds: cfg.OutboundConfigs})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	err = engine.Validate(raw)
	if err == nil {
		t.Fatal("premise gone: the engine now accepts the raw parser output")
	}
	if !strings.Contains(err.Error(), "unable to send through") {
		t.Fatalf("expected the sendThrough failure, got: %v", err)
	}

	// Stage two: sendThrough cleared, nulls left in.
	for i := range cfg.OutboundConfigs {
		cfg.OutboundConfigs[i].SendThrough = nil
	}
	raw, err = json.Marshal(xrayConfig{Outbounds: cfg.OutboundConfigs})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	err = engine.Validate(raw)
	if err == nil {
		t.Fatal("premise gone: the engine now accepts a document with its nulls left in")
	}
	if !strings.Contains(err.Error(), `serverNames`) {
		t.Fatalf("expected the null-dest failure, got: %v", err)
	}
}

// TestEngineErrorsAreRedactedBeforeTheyEscape checks the seam between this
// package and internal/engine, which is where a pasted key would reach a log if
// either side stopped doing its job.
//
// Two different guarantees meet here. This package never produces an error
// carrying a value, because it validates the link itself. The engine package
// never lets one out either, because it redacts what xray-core hands it. The
// raw engine underneath does quote values, and that is asserted first so the
// second half is known to be doing something.
func TestEngineErrorsAreRedactedBeforeTheyEscape(t *testing.T) {
	l := mustParse(t, vlessRealityLink())
	b, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("XrayConfig: %v", err)
	}
	// Corrupt the public key in the document itself, past this package's checks.
	broken := []byte(strings.Replace(string(b), fakePublicKey(), "TOOSHORT", 1))

	// The raw engine quotes the value it rejected.
	var cfg conf.Config
	if err := json.Unmarshal(broken, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	_, rawErr := cfg.Build()
	if rawErr == nil {
		t.Fatal("premise gone: xray-core now accepts a malformed REALITY public key")
	}
	if !strings.Contains(rawErr.Error(), "TOOSHORT") {
		t.Fatalf("premise gone: xray-core no longer quotes the value it rejected, "+
			"so this test proves nothing about redaction: %v", rawErr)
	}

	// internal/engine does not.
	validateErr := engine.Validate(broken)
	if validateErr == nil {
		t.Fatal("engine.Validate accepted a config xray-core rejects")
	}
	if strings.Contains(validateErr.Error(), "TOOSHORT") {
		t.Errorf("engine.Validate leaked the rejected value: %v", validateErr)
	}
	if !strings.Contains(validateErr.Error(), "redacted") {
		t.Errorf("engine.Validate neither quoted nor redacted the value, which is "+
			"unexpected enough to check by hand: %v", validateErr)
	}
}
