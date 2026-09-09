// SPDX-License-Identifier: AGPL-3.0-or-later
package link

import (
	"bytes"
	"encoding/json"
	"github.com/xtls/xray-core/infra/conf"
	"net/netip"
	"reflect"
	"testing"
)

func TestThroughLoopbackPreservesIdentityAndCredentials(t *testing.T) {
	for _, protocol := range []string{"vless", "trojan", "ss"} {
		t.Run(protocol, func(t *testing.T) {
			l, err := Parse(hostedLink(protocol, hostA, "test"))
			if err != nil {
				t.Fatal(err)
			}
			before, _ := l.XrayConfig()
			endpoint := netip.MustParseAddrPort("127.0.0.1:12345")
			forwarded, err := l.ThroughLoopback(endpoint)
			if err != nil {
				t.Fatal(err)
			}
			if forwarded.Address != "127.0.0.1" || forwarded.Port != 12345 {
				t.Fatalf("wrong endpoint: %+v", forwarded)
			}
			after, _ := forwarded.XrayConfig()
			var a, b map[string]any
			json.Unmarshal(before, &a)
			json.Unmarshal(after, &b)
			settings := b["outbounds"].([]any)[0].(map[string]any)["settings"].(map[string]any)
			original := a["outbounds"].([]any)[0].(map[string]any)["settings"].(map[string]any)
			settings["address"] = original["address"]
			settings["port"] = original["port"]
			if !reflect.DeepEqual(a, b) {
				t.Fatal("forwarding changed fields besides endpoint")
			}
			unchanged, _ := l.XrayConfig()
			if !bytes.Equal(before, unchanged) {
				t.Fatal("original mutated")
			}
		})
	}
}
func TestThroughLoopbackRejectsExternalEndpoint(t *testing.T) {
	l, _ := Parse(hostedLink("vless", hostA, "test"))
	for _, v := range []string{"192.0.2.1:443", "127.0.0.1:0"} {
		if _, err := l.ThroughLoopback(netip.MustParseAddrPort(v)); err == nil {
			t.Fatal("accepted", v)
		}
	}
}

func TestThroughLoopbackPreservesImplicitTLSAndHTTPNames(t *testing.T) {
	for _, network := range []string{"raw", "ws", "httpupgrade", "grpc"} {
		t.Run(network, func(t *testing.T) {
			l, err := Parse("vless://" + fakeUUID + "@" + hostA + ":443?security=tls&type=" + network)
			if err != nil {
				t.Fatal(err)
			}
			f, err := l.ThroughLoopback(netip.MustParseAddrPort("127.0.0.1:12345"))
			if err != nil {
				t.Fatal(err)
			}
			if f.ServerName != hostA {
				t.Fatal("implicit certificate identity changed")
			}
			raw, _ := f.XrayConfig()
			var doc map[string]any
			json.Unmarshal(raw, &doc)
			stream := doc["outbounds"].([]any)[0].(map[string]any)["streamSettings"].(map[string]any)
			section, key := "", "host"
			switch network {
			case "ws":
				section = "wsSettings"
			case "httpupgrade":
				section = "httpupgradeSettings"
			case "grpc":
				section = "grpcSettings"
				key = "authority"
			}
			if section != "" && stream[section].(map[string]any)[key] != hostA {
				t.Fatal("implicit HTTP identity changed")
			}
		})
	}
}

func TestThroughLoopbackRejectsMissingLink(t *testing.T) {
	var l *Link
	if _, err := l.ThroughLoopback(netip.MustParseAddrPort("127.0.0.1:12345")); err == nil {
		t.Fatal("accepted nil link")
	}
}
func TestThroughLoopbackPreservesExplicitTransportHost(t *testing.T) {
	for _, network := range []string{"ws", "httpupgrade", "grpc"} {
		l, err := Parse("vless://" + fakeUUID + "@" + hostA + ":443?security=tls&type=" + network + "&sni=" + fakeSNI + "&host=front.example.invalid&authority=front.example.invalid")
		if err != nil {
			t.Fatal(err)
		}
		f, err := l.ThroughLoopback(netip.MustParseAddrPort("127.0.0.1:12345"))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := f.XrayConfig()
		if !bytes.Contains(raw, []byte("front.example.invalid")) {
			t.Fatal("explicit transport host lost", network)
		}
	}
}
func TestThroughLoopbackPreservesPlainGRPCIPAddressAuthority(t *testing.T) {
	l, err := Parse("vless://" + fakeUUID + "@192.0.2.1:443?security=none&type=grpc")
	if err != nil {
		t.Fatal(err)
	}
	f, err := l.ThroughLoopback(netip.MustParseAddrPort("127.0.0.1:12345"))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := f.XrayConfig()
	if !bytes.Contains(raw, []byte(`192.0.2.1:443`)) {
		t.Fatal("implicit gRPC authority changed")
	}
}

func TestThroughLoopbackRejectsMissingOutboundSettings(t *testing.T) {
	l := &Link{outbound: &conf.OutboundDetourConfig{Protocol: "vless"}}
	if _, err := l.ThroughLoopback(netip.MustParseAddrPort("127.0.0.1:12345")); err == nil {
		t.Fatal("accepted incomplete outbound")
	}
}
func TestThroughLoopbackCreatesAbsentTransportDefaults(t *testing.T) {
	for _, tc := range []struct{ network, section, key string }{{"ws", "wsSettings", "host"}, {"grpc", "grpcSettings", "authority"}} {
		l, err := Parse("vless://" + fakeUUID + "@" + hostA + ":443?security=tls&type=" + tc.network)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := l.XrayConfig()
		var doc map[string]any
		json.Unmarshal(raw, &doc)
		stream := doc["outbounds"].([]any)[0].(map[string]any)["streamSettings"].(map[string]any)
		delete(stream, tc.section)
		raw, _ = json.Marshal(doc)
		l, err = Parse(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		f, err := l.ThroughLoopback(netip.MustParseAddrPort("127.0.0.1:12345"))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ = f.XrayConfig()
		json.Unmarshal(raw, &doc)
		stream = doc["outbounds"].([]any)[0].(map[string]any)["streamSettings"].(map[string]any)
		if stream[tc.section].(map[string]any)[tc.key] != hostA {
			t.Fatal("missing transport default changed host")
		}
	}
}

func TestThroughLoopbackPreservesRealityGRPCAuthority(t *testing.T) {
	l, err := Parse("vless://" + fakeUUID + "@" + hostA + ":443?security=reality&type=grpc&sni=" + fakeSNI + "&fp=chrome&pbk=" + fakePublicKey() + "&sid=" + fakeShortID)
	if err != nil {
		t.Fatal(err)
	}
	f, err := l.ThroughLoopback(netip.MustParseAddrPort("127.0.0.1:12345"))
	if err != nil {
		t.Fatal(err)
	}
	if f.ServerName != fakeSNI {
		t.Fatal("REALITY identity changed")
	}
	raw, _ := f.XrayConfig()
	if !bytes.Contains(raw, []byte(hostA+":443")) {
		t.Fatal("REALITY gRPC authority changed")
	}
}
