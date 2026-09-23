// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package xcfg

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"strings"
)

// # Why the engine must never look the server's name up itself
//
// GitHub issue 7, reported on Windows 11 against v0.2.11: a share link whose
// server is a DOMAIN turns on, and then all internet dies for the Windows host
// and for every hotspot client. The same subscription's entries with an IPv4
// literal work.
//
// The mechanism, read from the pinned engine
// (v1.260327.1-0.20260415235634-c5edc122b70e):
//
//   - Every transport dials the server through internet.DialSystem with
//     streamSettings.sockopt (transport/internet/tcp/dialer.go:22,
//     websocket/dialer.go:51 and :82, grpc/dial.go:127, httpupgrade/dialer.go:49,
//     splithttp/dialer.go:120 and :227, kcp/dialer.go:54,
//     hysteria/dialer.go:173).
//   - DialSystem consults the engine's DNS app ONLY when sockopt.domainStrategy
//     is set (transport/internet/dialer.go:252). Otherwise the destination
//     reaches DefaultSystemDialer.Dial still carrying the domain, and
//     net.Dialer.DialContext resolves it with the operating system's resolver
//     (transport/internet/system_dialer.go:145; :63 for UDP), at EVERY dial.
//   - On Windows the whole host is tunnelled: the default route goes through
//     the tunnel adapter, the adapter is given the host's DNS server, and a WFP
//     filter blocks port 53 on every other interface
//     (internal/netcfg/winnet.go, windowsPostEngineSteps). So once the engine
//     is up, the system resolver's query for the server's name goes into the
//     tunnel, whose outbound is the connection waiting for that answer.
//
// Reproduced on loopback by test/tunnel/servername_test.go with a name no
// resolver answers: without this mapping no protocol carries a byte, and the
// engine logs "lookup <name>: no such host".
//
// # What this does instead
//
// internal/privsvc already resolves the name BEFORE the tunnel exists
// (start.go, step 5) and the network plan pins a host route to what it got.
// Those addresses are handed in as Options.PinnedServer and written into the
// document twice:
//
//   - dns.hosts maps the server's name to them. StaticHosts is consulted
//     first and returns without querying any nameserver
//     (app/dns/dns.go:235-256), so the DNS app's own upstream queries, which
//     are routed into the tunnel, are never made for this name.
//   - The proxy outbound's streamSettings.sockopt.domainStrategy is set to
//     "ForceIP". That is what sends DialSystem to the DNS app at all, and
//     "Force" is what stops it falling back to the system resolver when the
//     lookup fails (dialer.go:257-262): the dial fails instead, which is a
//     connection error rather than a resolver deadlock.
//
// Only the address the socket connects to changes. The transport dialers take
// their TLS server name, HTTP Host and gRPC authority from the destination
// BEFORE they call DialSystem, and DialSystem rewrites only its own local copy
// (dialer.go:264), so every name on the wire is still the domain. The TLS
// server name is in any case already fixed in the document:
// link.fillMissingServerName writes the domain in when the link carried none.
//
// # Why not the outbound's own "targetStrategy"
//
// It looks like the right knob and it is not. It resolves ob.Target, the
// destination a CLIENT asked for, not the proxy server
// (app/proxyman/outbound/handler.go:184).
//
// # What changes for the user, stated so it is not discovered later
//
// With several addresses, the engine now picks one at random per dial
// (dialer.go:264), where Go's dialer would have tried each in turn. A dead
// address among live ones therefore fails some dials instead of costing a
// timeout. The set is the one the network plan pins, so every address in it
// is one this box can route to outside the tunnel.
func pinServerName(o Options, proxy json.RawMessage) (json.RawMessage, map[string][]string, error) {
	name := serverNameOf(o)
	if name == "" || len(o.PinnedServer) == 0 {
		// An IP literal needs nothing: the dial is already to an address, and
		// the document stays byte-identical to the one built before this
		// existed. A domain with no pinned addresses is a caller that did not
		// resolve. internal/privsvc/plans.go is the only caller outside a test,
		// and its guard is
		// TestTheEngineDocumentPinsTheServerNameToThePlannedAddresses.
		return proxy, nil, nil
	}

	var addrs []string
	seen := map[netip.Addr]bool{}
	usable := false
	for _, a := range o.PinnedServer {
		if !a.IsValid() {
			return nil, nil, ErrPinnedServerAddress
		}
		a = a.Unmap()
		if seen[a] {
			continue
		}
		seen[a] = true
		addrs = append(addrs, a.String())
		if (a.Is4() && o.DNS.Strategy != QueryUseIPv6) || (a.Is6() && o.DNS.Strategy != QueryUseIPv4) {
			usable = true
		}
	}
	if !usable {
		// The DNS app filters static hosts by its own query strategy
		// (app/dns/dns.go:227-233 and hosts.go:119), so every pinned address
		// would be dropped and every ForceIP dial would fail. Refusing here
		// names the cause; the engine would only have said "empty response".
		return nil, nil, ErrPinnedServerFamily
	}

	forced, err := forceIPDial(proxy)
	if err != nil {
		return nil, nil, err
	}
	// "full:" is the engine's default for a hosts key (infra/conf/dns.go:258,
	// ParseDomainRule with Domain_Full) and is written out so that a name can
	// never be read as a "domain:", "regexp:" or "geosite:" rule. Lower case
	// and no trailing dot because the lookup side does both to the query
	// (app/dns/dns.go:217, hosts.go:98) and the matcher lowercases the rule
	// (common/geodata/domain_matcher.go:168); normalising here keeps the key
	// readable as the name that will actually match.
	key := "full:" + name
	return forced, map[string][]string{key: addrs}, nil
}

// serverNameOf returns the link's server name in the form the engine's static
// hosts will match, or "" when the link names its server by an IP literal.
func serverNameOf(o Options) string {
	if o.Link == nil {
		return ""
	}
	host := strings.TrimSuffix(o.Link.Address, ".")
	if _, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil || host == "" {
		return ""
	}
	return strings.ToLower(host)
}

// forceIPDial sets streamSettings.sockopt.domainStrategy to "ForceIP" on the
// proxy outbound and leaves every other byte of it as internal/link emitted it.
//
// The outbound is decoded into generic JSON values rather than a struct of this
// package's own, for the reason outboundFromDocument gives: a struct would drop
// every field it did not name. Numbers are kept as written. Key order is not a
// change: internal/link's dropNulls already produced these bytes by marshalling
// a map, so the keys were sorted before this saw them.
//
// Any domainStrategy already there is overwritten, not respected. The parser
// never sets sockopt from a share link, so only a pasted raw JSON document can
// carry one, and every other value either consults the system resolver on
// failure (UseIP and its variants) or skips the lookup entirely (AsIs).
func forceIPDial(proxy json.RawMessage) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(proxy))
	dec.UseNumber()
	var ob map[string]any
	if err := dec.Decode(&ob); err != nil || ob == nil {
		return nil, errSerialise
	}
	stream, _ := ob["streamSettings"].(map[string]any)
	if stream == nil {
		// No stream settings means the engine's default, a plain TCP stream
		// with no security. An object carrying only sockopt builds to the same
		// thing: StreamConfig.Build starts from ProtocolName "tcp" and only
		// replaces it when "network" is present, and treats an absent
		// "security" as none (infra/conf/transport_internet.go:1958-1975).
		stream = map[string]any{}
		ob["streamSettings"] = stream
	}
	sockopt, _ := stream["sockopt"].(map[string]any)
	if sockopt == nil {
		sockopt = map[string]any{}
		stream["sockopt"] = sockopt
	}
	sockopt["domainStrategy"] = "ForceIP"
	out, err := json.Marshal(ob)
	if err != nil {
		return nil, errSerialise
	}
	return out, nil
}
