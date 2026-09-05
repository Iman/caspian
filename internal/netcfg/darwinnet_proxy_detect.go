// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"context"
	"strconv"
	"strings"
)

// ParseNetworkServices returns the enabled services printed by
// networksetup -listallnetworkservices. A leading asterisk is Apple's marker
// for a disabled service, not part of its name.
func ParseNetworkServices(out string) []string {
	var services []string
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "*") || strings.Contains(strings.ToLower(line), "asterisk") {
			continue
		}
		services = append(services, line)
	}
	return services
}

// ParseSystemSOCKSState reads networksetup -getsocksfirewallproxy. All four
// fields are required, including empty Server and zero Port when it is off;
// partial output is not enough evidence for a reversible change.
func ParseSystemSOCKSState(service, out string) (SystemSOCKSState, bool) {
	state := SystemSOCKSState{Service: service}
	seen := map[string]bool{}
	for _, raw := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(raw, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "enabled":
			var valid bool
			state.Enabled, valid = parseNetworksetupBool(value)
			if !valid {
				return SystemSOCKSState{}, false
			}
			seen[key] = true
		case "server":
			state.Server = value
			seen[key] = true
		case "port":
			port, err := strconv.ParseUint(value, 10, 16)
			if err != nil {
				return SystemSOCKSState{}, false
			}
			state.Port = uint16(port)
			seen[key] = true
		case "authenticated proxy enabled":
			var valid bool
			state.Authenticated, valid = parseNetworksetupBool(value)
			if !valid {
				return SystemSOCKSState{}, false
			}
			seen[key] = true
		}
	}
	return state, seen["enabled"] && seen["server"] && seen["port"] && seen["authenticated proxy enabled"]
}

func parseNetworksetupBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "on", "1":
		return true, true
	case "no", "off", "0":
		return false, true
	default:
		return false, false
	}
}

// ParseProxyBypassDomains reads networksetup -getproxybypassdomains. Apple
// prints one domain per line, or a sentence saying none are set.
func ParseProxyBypassDomains(out string) ([]string, bool) {
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return nil, false
	}
	if strings.Contains(strings.ToLower(trimmed), "aren't any bypass domains") {
		return nil, true
	}
	var domains []string
	for _, raw := range strings.Split(trimmed, "\n") {
		if domain := strings.TrimSpace(raw); domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains, len(domains) > 0
}

// darwinDetectSystemSOCKS is best effort for callers that do not intend to
// change the proxy. A plan that does intend to change it checks the Known bit
// and refuses if any read failed.
func darwinDetectSystemSOCKS(ctx context.Context, r Runner, f *Facts) {
	res, err := r.Run(ctx, Command{
		Path: BinNetworksetup, Args: []string{"-listallnetworkservices"},
		Why: "the enabled macOS network services whose proxy settings must be preserved",
	})
	if err != nil {
		return
	}
	services := ParseNetworkServices(res.Stdout)
	states := make([]SystemSOCKSState, 0, len(services))
	for _, service := range services {
		res, err := r.Run(ctx, Command{
			Path: BinNetworksetup, Args: []string{"-getsocksfirewallproxy", service},
			Why: "read the SOCKS proxy endpoint and state to restore on teardown",
		})
		if err != nil {
			return
		}
		state, ok := ParseSystemSOCKSState(service, res.Stdout)
		if !ok {
			return
		}
		res, err = r.Run(ctx, Command{
			Path: BinNetworksetup, Args: []string{"-getproxybypassdomains", service},
			Why: "read the proxy bypass domains to restore on teardown",
		})
		if err != nil {
			return
		}
		state.BypassDomains, ok = ParseProxyBypassDomains(res.Stdout)
		if !ok {
			return
		}
		states = append(states, state)
	}
	f.SystemSOCKS = states
	f.SystemSOCKSKnown = true
}
