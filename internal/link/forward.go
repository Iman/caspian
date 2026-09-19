// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"encoding/json"
	"net"
	"net/netip"
	"strconv"
)

// ThroughLoopback returns an independent outbound for a service-owned forwarder.
// The real TLS/REALITY identity, credentials, and HTTP identity remain unchanged.
func (l *Link) ThroughLoopback(endpoint netip.AddrPort) (*Link, error) {
	if !endpoint.Addr().IsLoopback() || endpoint.Port() == 0 {
		return nil, ErrBadAddress
	}
	raw, err := l.XrayConfig()
	if err != nil {
		return nil, err
	}
	// XrayConfig emits valid JSON with exactly one outbound. Decoding to JSON
	// values also makes the later Marshal infallible: there are no custom values.
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	_ = json.Unmarshal(raw, &doc)
	outbound := doc.Outbounds[0]
	settings, ok := outbound["settings"].(map[string]any)
	if !ok {
		return nil, ErrNoLink
	}
	settings["address"] = endpoint.Addr().String()
	settings["port"] = endpoint.Port()
	if stream, ok := outbound["streamSettings"].(map[string]any); ok {
		// Parse already fills a missing real TLS name from the original address.
		host := l.Address
		if l.Security == SecurityTLS {
			host = l.ServerName
		}
		switch l.Network {
		case "ws", "websocket":
			setDefaultSetting(stream, "wsSettings", "host", host)
		case "httpupgrade":
			setDefaultSetting(stream, "httpupgradeSettings", "host", host)
		case "grpc":
			authority := host
			_, ipErr := netip.ParseAddr(l.Address)
			if l.Security == SecurityReality || (l.Security != SecurityTLS && ipErr == nil) {
				authority = net.JoinHostPort(l.Address, strconv.Itoa(int(l.Port)))
			}
			setDefaultSetting(stream, "grpcSettings", "authority", authority)
		}
	}
	raw, _ = json.Marshal(doc)
	return Parse(string(raw))
}

// setDefaultSetting preserves an explicit transport host or authority.
func setDefaultSetting(stream map[string]any, section, key, value string) {
	settings, ok := stream[section].(map[string]any)
	if !ok {
		settings = map[string]any{}
		stream[section] = settings
	}
	if existing, _ := settings[key].(string); existing == "" {
		settings[key] = value
	}
}
