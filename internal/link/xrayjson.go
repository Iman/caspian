// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"encoding/json"
	"strings"

	"github.com/xtls/xray-core/infra/conf"
)

// A full xray-core JSON config is the third shape a pasted text can take, after
// share links and Clash documents. It arrives two ways (GitHub issue 7):
//
//   - one object, as v2rayN and v2rayNG export a "Custom" config: log, policy,
//     inbounds, outbounds, dns, routing, and a "remarks" display name;
//   - an array of such objects, as a BPB panel subscription serves with
//     ?app=xray. Measured 2026-09-23 on a 208-entry sample: every object has
//     remarks, a version, dns, inbounds, routing and four or more outbounds.
//
// Each object is one entry, the same model a subscription list has, and from
// each object exactly one outbound is taken: the proxy. Everything else in it
// (inbounds, routing, dns, policy, the other outbounds) is ignored, because
// this box decides all of those itself (internal/xcfg) and none of them may be
// taken from pasted text. The outbound is decoded into the engine's own
// conf.OutboundDetourConfig and re-serialised from it, as every other path in
// this package does; nothing from the pasted text is interpolated.
//
// Before this file, a JSON object went to the vendored parser, which decoded
// the whole document into conf.Config and returned every outbound as an entry
// (third_party/libxray-share/parse_share.go:63-72), and fill read only the flat
// settings form. Measured 2026-09-23: a v2rayN custom config was refused with
// ErrBadAddress because its server was under settings.vnext, and a BPB array
// was refused with ErrNoLink because the vendored parser has no array branch.

// proxyProtocols are the outbound protocols this box can use as its proxy: the
// ones the vendored parser emits and fill knows how to check. The names are the
// engine's, infra/conf/xray.go outboundConfigLoader.
var proxyProtocols = map[string]bool{
	"vless":       true,
	"vmess":       true,
	"trojan":      true,
	"shadowsocks": true,
	"socks":       true,
	"hysteria":    true,
}

// isXrayJSON reports whether text is a JSON object or array, which is how the
// JSON path is told apart from share links, base64 blobs and YAML: none of
// those can start with either character.
func isXrayJSON(text string) bool {
	return strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[")
}

// xrayDocument is the part of a full config this package reads. Everything
// else in the document is left undecoded on purpose, so a field this package
// does not use cannot make a config fail to parse.
type xrayDocument struct {
	Remarks   string            `json:"remarks"`
	Outbounds []json.RawMessage `json:"outbounds"`
}

// parseXrayJSON reads one object or an array of objects. It fails only when
// the text is not JSON of either shape; an object that holds no usable proxy
// is kept as a refused entry, so that the entries after it keep their index
// and ParseAll can count it in Dropped.
//
// The decoder's error is never returned: it quotes the text it choked on.
func parseXrayJSON(text string) (*parsed, error) {
	var objects []json.RawMessage
	if strings.HasPrefix(text, "[") {
		if err := json.Unmarshal([]byte(text), &objects); err != nil {
			return nil, ErrNoLink
		}
	} else {
		objects = []json.RawMessage{json.RawMessage(text)}
		if !json.Valid(objects[0]) {
			return nil, ErrNoLink
		}
	}
	if len(objects) == 0 {
		return nil, ErrNoLink
	}
	p := &parsed{
		outbounds: make([]conf.OutboundDetourConfig, len(objects)),
		names:     make([]string, len(objects)),
		refused:   make([]error, len(objects)),
	}
	for i, raw := range objects {
		var doc xrayDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			// Not an object, or an object whose outbounds is not a list.
			p.refused[i] = ErrNoLink
			continue
		}
		p.names[i] = doc.Remarks
		ob, err := proxyOutboundOf(doc.Outbounds)
		if err != nil {
			p.refused[i] = err
			continue
		}
		p.outbounds[i] = *ob
	}
	return p, nil
}

// proxyOutboundOf picks the proxy out of one config's outbounds.
//
// The outbound tagged "proxy" is preferred when its protocol is a proxy
// protocol: that is the tag v2rayN, v2rayNG and BPB give the server, and the
// one their routing sends traffic to. Otherwise the first outbound in a proxy
// protocol is taken. A "proxy" in another protocol is passed over rather than
// refused: in the BPB sample the two objects tagged that way are freedom
// outbounds, which are not servers, and the result for them is
// ErrNoProxyOutbound either way because nothing else in them is a proxy.
//
// Only the chosen outbound is decoded into the engine's struct, so an
// unrelated outbound this engine version cannot decode does not sink the
// entry.
func proxyOutboundOf(outbounds []json.RawMessage) (*conf.OutboundDetourConfig, error) {
	chosen := -1
	for i, raw := range outbounds {
		var head struct {
			Tag      string `json:"tag"`
			Protocol string `json:"protocol"`
		}
		if json.Unmarshal(raw, &head) != nil || !proxyProtocols[head.Protocol] {
			continue
		}
		if head.Tag == OutboundTag {
			chosen = i
			break
		}
		if chosen < 0 {
			chosen = i
		}
	}
	if chosen < 0 {
		return nil, ErrNoProxyOutbound
	}
	var ob conf.OutboundDetourConfig
	if err := json.Unmarshal(outbounds[chosen], &ob); err != nil {
		return nil, ErrNoLink
	}
	return &ob, nil
}

// checkChain refuses an outbound that dials through another outbound.
//
// Two ways to say it exist: streamSettings.sockopt.dialerProxy and the older
// proxySettings.tag. Both name another outbound by tag, and the engine looks
// that tag up only when it dials: transport/internet/dialer.go:271-279 in the
// pinned xray-core returns "there is no outbound handler for dialerProxy" per
// connection. So a dangling reference builds, starts, and carries nothing.
//
// Carrying the chained outbound as well was considered and not done, on what
// the samples actually contain. Measured 2026-09-23 across the 209 documents in
// the issue 7 samples (one v2rayN custom config, a 208-entry BPB array): zero
// use dialerProxy or proxySettings. BPB expresses its TLS ClientHello fragment
// as streamSettings.finalmask.tcp on the proxy outbound itself, which the
// engine builds as a transport mask (infra/conf/transport_internet.go:2092-2098,
// FragmentMask at :1406-1448) and which this package carries untouched. The
// chained shape is the older form, where the fragment lived in a freedom
// outbound's settings; the pinned engine still accepts that
// (infra/conf/freedom.go:78), but it needs a second outbound, and this box
// emits exactly one from what was pasted.
//
// There is a second reason, independent of the first. The tag is resolved
// against the engine's whole outbound list, which in a running box is this
// box's own document: "direct", "block" and "dns-out" (internal/xcfg/build.go,
// direct, blackhole and dnsOut). A pasted "dialerProxy": "direct" would bind the
// user's proxy to an outbound this box owns. No pasted text gets to name one.
//
// It runs from fill, so it holds for every input path, not only JSON. The
// vendored share-link parser never sets either field today.
func checkChain(ob *conf.OutboundDetourConfig) error {
	if ob.ProxySettings != nil && ob.ProxySettings.Tag != "" {
		return ErrChainedOutbound
	}
	if ss := ob.StreamSetting; ss != nil && ss.SocketSettings != nil && ss.SocketSettings.DialerProxy != "" {
		return ErrChainedOutbound
	}
	return nil
}

// flattenSettings rewrites the vnext or servers form of an outbound's
// settings into the flat form, using the engine's own structs for both.
//
// # Why rewrite rather than only read
//
// The engine accepts both forms: every client config struct has a flat
// address/port and a one-member list, and Build copies the flat form into the
// list when the flat address is set (infra/conf/vless.go:251-259, vmess.go
// :121-129, trojan.go:44-55, shadowsocks.go:187-199, socks.go:84-94). A reader
// that understood both forms would be enough for fill's checks. It would not be
// enough for the rest of this program, which reads the emitted document in the
// flat form only: ThroughLoopback (forward.go) redirects the proxy to a
// loopback forwarder by overwriting settings.address and settings.port, and on
// a vnext-form VLESS outbound that would set a flat address with no flat id,
// which the engine then fails to parse as a UUID (vless.go:285-310). One form
// in the document means one place the server can be read from, so the address
// fill reports, the address privsvc pins a route to, and the address the
// engine dials are the same bytes.
//
// # What is carried
//
// The user object inside vnext is decoded by the engine as protocol.User plus
// the protocol's account (vless.go:280-302, vmess.go:144-163, socks.go
// :106-124), so the keys read here are exactly the JSON keys of those types
// and the flat struct has a field for each. A key the engine would not read
// from the user object is not carried, which is what the engine does with it.
// VLESS reverse is the one exception: the engine refuses it in the vnext form
// (vless.go:303-305) and this box owns every inbound a reverse tunnel would
// need, so it is not carried either.
//
// More than one server, or more than one user, is ErrManyServers: the engine
// refuses both, and choosing one of several would pick a server for the user.
func flattenSettings(ob *conf.OutboundDetourConfig) error {
	if ob.Settings == nil {
		return nil
	}
	raw := []byte(*ob.Settings)
	var out any
	switch ob.Protocol {
	case "vless":
		var c conf.VLessOutboundConfig
		if json.Unmarshal(raw, &c) != nil {
			return ErrNoLink
		}
		if c.Address != nil || len(c.Vnext) == 0 {
			return nil
		}
		if len(c.Vnext) != 1 || len(c.Vnext[0].Users) != 1 {
			return ErrManyServers
		}
		var u struct {
			Level      uint32   `json:"level"`
			Email      string   `json:"email"`
			ID         string   `json:"id"`
			Flow       string   `json:"flow"`
			Encryption string   `json:"encryption"`
			Testpre    uint32   `json:"testpre"`
			Testseed   []uint32 `json:"testseed"`
		}
		if json.Unmarshal(c.Vnext[0].Users[0], &u) != nil {
			return ErrNoLink
		}
		out = &conf.VLessOutboundConfig{
			Address: c.Vnext[0].Address, Port: c.Vnext[0].Port,
			Level: u.Level, Email: u.Email, Id: u.ID, Flow: u.Flow,
			Encryption: u.Encryption, Testpre: u.Testpre, Testseed: u.Testseed,
		}
	case "vmess":
		var c conf.VMessOutboundConfig
		if json.Unmarshal(raw, &c) != nil {
			return ErrNoLink
		}
		if c.Address != nil || len(c.Receivers) == 0 {
			return nil
		}
		if len(c.Receivers) != 1 || len(c.Receivers[0].Users) != 1 {
			return ErrManyServers
		}
		var u struct {
			Level uint32 `json:"level"`
			Email string `json:"email"`
			conf.VMessAccount
		}
		if json.Unmarshal(c.Receivers[0].Users[0], &u) != nil {
			return ErrNoLink
		}
		out = &conf.VMessOutboundConfig{
			Address: c.Receivers[0].Address, Port: c.Receivers[0].Port,
			Level: u.Level, Email: u.Email,
			ID: u.ID, Security: u.Security, Experiments: u.Experiments,
		}
	case "trojan":
		var c conf.TrojanClientConfig
		if json.Unmarshal(raw, &c) != nil {
			return ErrNoLink
		}
		if c.Address != nil || len(c.Servers) == 0 {
			return nil
		}
		if len(c.Servers) != 1 {
			return ErrManyServers
		}
		s := c.Servers[0]
		out = &conf.TrojanClientConfig{
			Address: s.Address, Port: s.Port, Level: s.Level, Email: s.Email,
			Password: s.Password, Flow: s.Flow,
		}
	case "shadowsocks":
		var c conf.ShadowsocksClientConfig
		if json.Unmarshal(raw, &c) != nil {
			return ErrNoLink
		}
		if c.Address != nil || len(c.Servers) == 0 {
			return nil
		}
		if len(c.Servers) != 1 {
			return ErrManyServers
		}
		s := c.Servers[0]
		out = &conf.ShadowsocksClientConfig{
			Address: s.Address, Port: s.Port, Level: s.Level, Email: s.Email,
			Cipher: s.Cipher, Password: s.Password, UoT: s.UoT, UoTVersion: s.UoTVersion,
		}
	case "socks":
		var c conf.SocksClientConfig
		if json.Unmarshal(raw, &c) != nil {
			return ErrNoLink
		}
		if c.Address != nil || len(c.Servers) == 0 {
			return nil
		}
		if len(c.Servers) != 1 || len(c.Servers[0].Users) > 1 {
			return ErrManyServers
		}
		flat := &conf.SocksClientConfig{Address: c.Servers[0].Address, Port: c.Servers[0].Port}
		if len(c.Servers[0].Users) == 1 {
			var u struct {
				Level uint32 `json:"level"`
				Email string `json:"email"`
				conf.SocksAccount
			}
			if json.Unmarshal(c.Servers[0].Users[0], &u) != nil {
				return ErrNoLink
			}
			flat.Level, flat.Email = u.Level, u.Email
			flat.Username, flat.Password = u.Username, u.Password
		}
		out = flat
	default:
		// hysteria has only the flat form (infra/conf/hysteria.go:12-16).
		return nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return errSerialise
	}
	msg := json.RawMessage(b)
	ob.Settings = &msg
	return nil
}
