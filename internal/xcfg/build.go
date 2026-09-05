// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"bytes"
	"encoding/json"

	"caspianbyoc.org/caspian/internal/link"
)

// document is the top-level shape handed to the engine.
//
// Field order here is the key order in the emitted JSON, because
// encoding/json marshals struct fields in declaration order. That determinism
// is what makes the golden files in testdata a diff rather than a coin toss.
//
// The keys are the ones infra/conf.Config declares at infra/conf/xray.go:346-365.
type document struct {
	Log       logSection        `json:"log"`
	DNS       dnsSection        `json:"dns"`
	Inbounds  []json.RawMessage `json:"inbounds"`
	Outbounds []json.RawMessage `json:"outbounds"`
	Routing   routingSection    `json:"routing"`
}

// logSection is infra/conf.LogConfig, infra/conf/log.go:18-24.
//
// "access": "none" and dnsLog false are emitted rather than left out, because
// they are the privacy decision and a document that does not state it reads as
// if nobody made it. internal/engine/logring.go:225-227 forces both again on
// the way in; this is the same decision written where a reader of the config
// will see it.
type logSection struct {
	LogLevel string `json:"loglevel"`
	Access   string `json:"access"`
	DNSLog   bool   `json:"dnsLog"`
}

// dnsSection is the subset of infra/conf.DNSConfig this appliance sets,
// infra/conf/dns.go:198-211.
//
// Servers are plain strings: NameServerConfig.UnmarshalJSON at
// infra/conf/dns.go:37-42 takes a bare address before it tries the object
// form, and the object form buys nothing this appliance uses.
type dnsSection struct {
	Servers       []string `json:"servers"`
	QueryStrategy string   `json:"queryStrategy"`
	Tag           string   `json:"tag"`
}

// routingSection is infra/conf.RouterConfig, infra/conf/router.go:77-81.
type routingSection struct {
	DomainStrategy string `json:"domainStrategy"`
	Rules          []rule `json:"rules"`
}

// rule is the subset of the anonymous RawFieldRule struct at
// infra/conf/router.go:532-551 that this appliance uses.
//
// There is no "type": "field" key. Every rule is a field rule now: parseRule
// at router.go:678-690 unmarshals into RouterRule, which carries only
// ruleTag, outboundTag and balancerTag (router.go:126-130), and then calls
// parseFieldRule unconditionally. Emitting "type" would be a dead key that
// the loader silently drops, and this package does not write keys the engine
// does not read.
type rule struct {
	RuleTag     string   `json:"ruleTag"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	IP          []string `json:"ip,omitempty"`
	Port        string   `json:"port,omitempty"`
	Network     string   `json:"network,omitempty"`
	OutboundTag string   `json:"outboundTag"`
}

type tunInbound struct {
	Tag      string      `json:"tag"`
	Protocol string      `json:"protocol"`
	Settings tunSettings `json:"settings"`
}

// tunSettings is infra/conf.TunConfig, infra/conf/tun.go:8-12. All three
// fields, and there are no others. Note the capitalised MTU key: that is the
// engine's, not a typo here.
type tunSettings struct {
	Name      string `json:"name"`
	MTU       uint32 `json:"MTU"`
	UserLevel uint32 `json:"userLevel"`
}

type socksInbound struct {
	Tag      string        `json:"tag"`
	Protocol string        `json:"protocol"`
	Listen   string        `json:"listen"`
	Port     uint16        `json:"port"`
	Settings socksSettings `json:"settings"`
}

// socksSettings is the subset of infra/conf.SocksServerConfig,
// infra/conf/socks.go:30-35, that this appliance sets.
//
// The "ip" field is deliberately absent. It is the address a UDP-associate
// reply tells the client to send datagrams to, and when it is unset the server
// answers with the session's own gateway address instead
// (proxy/socks/protocol.go:195-197). Since this inbound binds loopback, the
// gateway IS the loopback address, so setting "ip" could only ever disagree
// with where the inbound actually is.
type socksSettings struct {
	Auth string `json:"auth"`
	UDP  bool   `json:"udp"`
}

type freedomOutbound struct {
	Tag      string          `json:"tag"`
	Protocol string          `json:"protocol"`
	Settings freedomSettings `json:"settings"`
}

// freedomSettings carries domainStrategy explicitly.
//
// "AsIs" is already the engine's default for an empty value
// (infra/conf/freedom.go:49-51), and it is written out because it is the
// second of the two switches that decide whether this document ever performs
// a DNS lookup of its own. See DNS.Intercept for the enumeration.
type freedomSettings struct {
	DomainStrategy string `json:"domainStrategy"`
}

type blackholeOutbound struct {
	Tag      string            `json:"tag"`
	Protocol string            `json:"protocol"`
	Settings blackholeSettings `json:"settings"`
}

// blackholeSettings selects the "none" response, which closes the connection
// rather than writing an HTTP 403 first. infra/conf/blackhole.go:45-50
// registers the two response types under the "type" key.
type blackholeSettings struct {
	Response blackholeResponse `json:"response"`
}

type blackholeResponse struct {
	Type string `json:"type"`
}

type dnsOutbound struct {
	Tag      string         `json:"tag"`
	Protocol string         `json:"protocol"`
	Settings dnsOutSettings `json:"settings"`
}

// dnsOutSettings is the subset of infra/conf.DNSOutboundConfig,
// infra/conf/dns_proxy.go:10-17.
//
// nonIPQuery "reject" is emitted rather than left empty. It is the same value
// the engine falls back to (proxy/dns/dns.go:68-71), and it is written out
// because the alternative, "skip", forwards a non-A/AAAA query on to its
// original destination, which is the one setting on this outbound that could
// let a query out by a path the routing rules did not choose.
type dnsOutSettings struct {
	NonIPQuery string `json:"nonIPQuery"`
}

// Build composes the configuration the box runs with the user's link.
//
// The returned bytes are indented JSON with a trailing newline. Indented
// because the same document is what advanced mode shows and what the golden
// files diff, and a few hundred bytes of whitespace on a config loaded once
// per connect is not a cost worth trading readability for.
//
// Nothing in the returned document comes from user text except the outbound
// object, which internal/link parsed and re-serialised. No string is
// interpolated anywhere in this file.
func Build(o Options) ([]byte, error) {
	if o.Link == nil {
		return nil, ErrNoLink
	}
	o = o.normalise()
	if err := o.check(); err != nil {
		return nil, err
	}

	proxy, err := proxyOutbound(o.Link)
	if err != nil {
		return nil, err
	}

	outbounds := []any{}
	// The proxy outbound is FIRST, and that position is load-bearing.
	// app/proxyman/outbound/outbound.go:109-110 makes the first handler added
	// the manager's defaultHandler, and app/dispatcher/default.go:491-492
	// hands any connection no rule matched to it. Whatever is first is what
	// carries traffic when the rules are wrong, so it is the tunnel.
	outbounds = append(outbounds, proxy)
	outbounds = append(outbounds, direct(), blackhole())
	if o.DNS.Intercept || o.LocalDNS.Enabled {
		outbounds = append(outbounds, dnsOut())
	}

	// Rule ORDER is the substance of this block. The first two rules match on
	// inboundTag only, so neither can ever match ordinary client traffic, and
	// both are above every rule that could answer a DNS query anywhere but
	// through the tunnel.
	rules := []rule{}
	if o.LocalDNS.Enabled {
		// ABOVE the private rule, deliberately.
		//
		// The listener stamps a fixed destination on every query it accepts
		// (see localDNSInbound), and that destination is the operator's first
		// resolver, which may legitimately be a private address when their
		// resolver sits behind their own proxy server. If the private rule
		// came first it would match that destination and hand the query to
		// the freedom outbound, which would resolve it on the local network:
		// the exact leak internal/hotspot refuses to allow on its side
		// (internal/hotspot/dnsmasq.go:141-146). Matching on inboundTag first
		// makes the stamped destination irrelevant to routing.
		rules = append(rules, rule{
			RuleTag:     ruleTagLocalDNS,
			InboundTag:  []string{TagLocalDNSIn},
			OutboundTag: TagDNSOut,
		})
	}
	rules = append(rules,
		// Also above the private rule, for the same reason and one more: a
		// resolver on a private address is a legitimate configuration when it
		// is reachable only through the proxy, and this ordering means
		// resolver queries go through the tunnel whatever their address. It
		// must also precede the port 53 intercept rule below, because the DNS
		// app's own upstream queries are stamped with dns.tag
		// (app/dns/nameserver.go:274); if the port 53 rule came first, those
		// queries would match it and be handed straight back to the DNS app
		// that made them.
		rule{RuleTag: ruleTagResolvers, InboundTag: []string{TagResolverIn}, OutboundTag: TagProxy},
	)
	if o.DNS.Intercept {
		rules = append(rules, rule{
			RuleTag: ruleTagDNS,
			// A JSON string rather than a number. PortList.UnmarshalJSON at
			// infra/conf/common.go:243-273 takes either, and the string form
			// is the one that can also carry a range, so a later change from
			// "53" to "53,5353" is a value change rather than a type change.
			Port:        "53",
			Network:     "tcp,udp",
			OutboundTag: TagDNSOut,
		})
	}
	// A private DNS destination is still DNS, not an exemption from tunnel
	// policy. Intercept it before allowing ordinary local-network access.
	rules = append(rules, privateRule(TagDirect))
	// The catch-all is explicit rather than left to the default handler.
	// Both mechanisms point at the proxy; stating it in the rules means a
	// later edit that reorders the outbounds cannot quietly change where
	// unmatched traffic goes.
	//
	// "tcp,udp" is spelled out because an absent network is NOT "any":
	// NetworkList.Build at infra/conf/common.go:110-112 returns TCP only for
	// a nil list, so a catch-all with no network would leave every UDP flow
	// unmatched.
	rules = append(rules, rule{
		RuleTag:     ruleTagCatchAll,
		Network:     "tcp,udp",
		OutboundTag: TagProxy,
	})

	return assemble(o, outbounds, rules)
}

// BuildFailClosed composes the configuration for when there is no usable
// tunnel: no config pasted yet, a config being swapped, or a link the engine
// rejected.
//
// It carries ONE outbound, the blackhole, so there is nothing in the document
// that could reach the network even if every rule in it were deleted. That is
// the point: the firewall in internal/netcfg is the real fail-closed
// mechanism, and docs/2026-08-29-design.md section 7 is explicit that it has
// to be, because when the TUN device disappears the kernel drops its routes
// and traffic falls back to the uplink. This builder's job is narrower and
// still worth doing: the configuration must not be the thing that opens a
// hole.
//
// o.Link is ignored. A caller holding a link that will not start is exactly
// who needs this document.
func BuildFailClosed(o Options) ([]byte, error) {
	o = o.normalise()
	if err := o.check(); err != nil {
		return nil, err
	}
	outbounds := []any{blackhole()}
	rules := []rule{{
		RuleTag:     ruleTagReject,
		Network:     "tcp,udp",
		OutboundTag: TagBlock,
	}}
	return assemble(o, outbounds, rules)
}

// assemble marshals the parts into the final document.
func assemble(o Options, outbounds []any, rules []rule) ([]byte, error) {
	inbounds := []any{}
	if !o.TUN.Disabled {
		inbounds = append(inbounds, tunInbound{
			Tag:      TagTUNIn,
			Protocol: "tun",
			Settings: tunSettings{
				Name:      o.TUN.Name,
				MTU:       o.TUN.MTU,
				UserLevel: o.TUN.UserLevel,
			},
		})
	}
	inbounds = append(inbounds, socksInbound{
		Tag:      TagSOCKSIn,
		Protocol: "socks",
		Listen:   o.SOCKS.Listen,
		Port:     o.SOCKS.Port,
		Settings: socksSettings{Auth: "noauth", UDP: o.SOCKS.UDP},
	})
	if o.LocalDNS.Enabled {
		inbounds = append(inbounds, localDNSInbound(o))
	}

	rawIn, err := rawAll(inbounds)
	if err != nil {
		return nil, err
	}
	rawOut, err := rawAll(outbounds)
	if err != nil {
		return nil, err
	}

	doc := document{
		Log: logSection{
			LogLevel: string(o.LogLevel),
			Access:   "none",
			DNSLog:   false,
		},
		DNS: dnsSection{
			Servers:       o.DNS.Servers,
			QueryStrategy: string(o.DNS.Strategy),
			// Set explicitly so a routing rule can name the DNS app's own
			// upstream queries. Empty would make app/dns/dns.go:119-121
			// generate a random tag that nothing can match. See TagResolverIn.
			Tag: TagResolverIn,
		},
		Inbounds:  rawIn,
		Outbounds: rawOut,
		Routing: routingSection{
			// AsIs, and this is a decision rather than a default taken by
			// accident. app/router/router.go:253 and :263 show the router
			// consults the DNS client only for IpOnDemand and IpIfNonMatch;
			// under IpIfNonMatch EVERY domain-targeted connection would have
			// its hostname resolved through the DNS app before the rules were
			// re-applied. On this box that would put a resolver lookup in
			// front of every connection a hotspot client makes, whether or
			// not any rule needed it.
			DomainStrategy: "AsIs",
			Rules:          rules,
		},
	}

	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, errSerialise
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return nil, errSerialise
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func rawAll(items []any) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		b, err := json.Marshal(item)
		if err != nil {
			return nil, errSerialise
		}
		out = append(out, b)
	}
	return out, nil
}

// linkDocument is the shape internal/link.XrayConfig returns: outbounds only.
// See internal/link/link.go:454-459.
//
// Line numbers rot, and this pair rotted once already: internal/link grew by
// about 150 lines on 2026-08-30 while this package was being written, and both
// citations here moved. The contract they describe did not, and that is what
// the tests hold rather than the numbers: TestOutboundTagIsCheckedNotAssumed
// asserts the tag internal/link stamps, and the golden files in testdata carry
// the outbound object byte for byte, so a change to what internal/link emits
// shows up as a diff a person reads rather than as a surprise on a box.
type linkDocument struct {
	Outbounds []json.RawMessage `json:"outbounds"`
}

// proxyOutbound takes the outbound object out of the document internal/link
// produced, and checks the one property every routing rule here depends on.
//
// It is split in two so that the checking half can be tested. The two halves
// are "get the bytes", which needs a real parsed Link and can only ever
// produce a correct document, and "decide whether these bytes are usable",
// which is the part with the guards in it. Before the split those guards were
// unreachable from any test: link.XrayConfig stamps the tag itself, so no
// input to THIS function could make the tag wrong, and the branch sat
// uncovered while a test named TestOutboundTagIsCheckedNotAssumed claimed to
// exercise it. See outboundFromDocument.
func proxyOutbound(l *link.Link) (json.RawMessage, error) {
	raw, err := l.XrayConfig()
	if err != nil {
		return nil, err
	}
	return outboundFromDocument(raw)
}

// outboundFromDocument extracts and checks the single outbound in raw.
//
// The bytes are carried as a json.RawMessage rather than decoded and
// re-encoded. Decoding into a struct of this package's own would mean
// enumerating every field of every protocol and transport the parser supports,
// and any field this package did not know about would be silently dropped on
// the way out. The bytes already came out of encoding/json inside
// internal/link, so this is re-serialisation, not interpolation.
//
// Every failure here means internal/link produced something this package
// cannot use. None of them is reachable from a Link that link.Parse returned,
// which is why the checks are worth having: they are what turns a future
// change in internal/link from a box that connects and carries nothing into a
// refusal with a name on it.
func outboundFromDocument(raw []byte) (json.RawMessage, error) {
	var doc linkDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, errSerialise
	}
	if len(doc.Outbounds) == 0 {
		return nil, ErrOutboundMissing
	}
	// Checked rather than trusted. link.XrayConfig sets the tag, and every
	// rule in this file names it. A rule pointing at an outbound that is not
	// there does not fall back: the dispatcher closes the connection at
	// app/dispatcher/default.go:481-484.
	var tagged struct {
		Tag string `json:"tag"`
	}
	if err := json.Unmarshal(doc.Outbounds[0], &tagged); err != nil {
		return nil, errSerialise
	}
	if tagged.Tag != TagProxy {
		return nil, ErrOutboundTag
	}
	return doc.Outbounds[0], nil
}

func direct() freedomOutbound {
	return freedomOutbound{
		Tag:      TagDirect,
		Protocol: "freedom",
		Settings: freedomSettings{DomainStrategy: "AsIs"},
	}
}

func blackhole() blackholeOutbound {
	return blackholeOutbound{
		Tag:      TagBlock,
		Protocol: "blackhole",
		Settings: blackholeSettings{Response: blackholeResponse{Type: "none"}},
	}
}

func dnsOut() dnsOutbound {
	return dnsOutbound{
		Tag:      TagDNSOut,
		Protocol: "dns",
		Settings: dnsOutSettings{NonIPQuery: "reject"},
	}
}

func privateRule(outboundTag string) rule {
	return rule{
		RuleTag:     ruleTagPrivate,
		IP:          PrivateRanges(),
		OutboundTag: outboundTag,
	}
}

// dokodemoInbound is an inbound using the dokodemo-door proxy, which forwards
// everything it accepts to one fixed destination.
//
// infra/conf/dokodemo.go:10-17 is the whole JSON surface. followRedirect is
// deliberately absent: it is the transparent-proxy mode that recovers an
// original destination through SO_ORIGINAL_DST, this listener is an ordinary
// socket that something dialled on purpose, and
// docs/2026-08-29-design.md section 4.3 records why the redirect path is not
// used by this product at all.
type dokodemoInbound struct {
	Tag      string           `json:"tag"`
	Protocol string           `json:"protocol"`
	Listen   string           `json:"listen"`
	Port     uint16           `json:"port"`
	Settings dokodemoSettings `json:"settings"`
}

type dokodemoSettings struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
	Network string `json:"network"`
}

// localDNSInbound builds the loopback DNS listener that internal/hotspot's
// dnsmasq forwards to. See localdns_doc.go for why it exists.
//
// # Why "network" is spelled out
//
// DNS is mostly UDP, and an absent network here means TCP ONLY, not "any":
// DokodemoConfig.Build calls v.Network.Build() at infra/conf/dokodemo.go:31,
// and NetworkList.Build returns []net.Network{net.Network_TCP} for a nil
// receiver (infra/conf/common.go:110-112). A listener without this line would
// accept the TCP retry and silently ignore every ordinary UDP query.
//
// # Why the destination is the first resolver
//
// The dokodemo proxy has to stamp SOME destination, and the routing rule for
// this inbound matches on inboundTag, so the value never decides where the
// query goes. It still matters, for two reasons.
//
// First, it is what the dns outbound falls back to. proxy/dns/dns.go:115-124
// keeps ob.Target as the dial target and overrides it only with fields the
// dns outbound's own settings set; this package sets none of them, so the
// stamped destination survives. That target is reached at :229 for any query
// the built-in resolver did not answer.
//
// Second, and this is what closes it: parseIPQuery at proxy/dns/dns.go:80-102
// returns true only for A and AAAA, and :210-213 answers those from the
// built-in resolver without dialing anything. Everything else hits the
// nonIPQuery branch, and this package emits "reject" (:219-226), which replies
// with an error rather than forwarding. So the dial at :229-231 is
// UNREACHABLE as configured. Using the operator's first resolver rather than
// an arbitrary placeholder means that if a later edit sets nonIPQuery to
// "skip" and opens that path, it opens onto an address the operator chose and
// that checkResolvers has already refused to let be a Google address.
func localDNSInbound(o Options) dokodemoInbound {
	return dokodemoInbound{
		Tag:      TagLocalDNSIn,
		Protocol: "dokodemo-door",
		Listen:   o.LocalDNS.Listen,
		Port:     o.LocalDNS.Port,
		Settings: dokodemoSettings{
			// normalise guarantees at least one resolver, and check has
			// already validated every entry.
			Address: o.DNS.Servers[0],
			Port:    53,
			Network: "tcp,udp",
		},
	}
}
