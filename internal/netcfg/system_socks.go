// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// SystemSOCKSOptions is the loopback SOCKS endpoint exposed through the
// macOS system proxy settings. Enabled is opt-in so existing Linux, Windows,
// and pure netcfg callers retain their previous behaviour.
type SystemSOCKSOptions struct {
	Enabled bool
	Listen  string
	Port    uint16
}

// SystemSOCKSState is one enabled macOS network service's measured SOCKS and
// bypass-domain configuration. networksetup reports whether authentication is
// enabled, but deliberately does not reveal the password.
type SystemSOCKSState struct {
	Service       string
	Enabled       bool
	Server        string
	Port          uint16
	Authenticated bool
	BypassDomains []string
}

func systemSOCKSForPlan(f Facts, o Options) ([]SystemSOCKSState, error) {
	if o.Platform != PlatformDarwin {
		return nil, fmt.Errorf("netcfg: the macOS system SOCKS proxy was requested for %s", o.Platform)
	}
	listen, err := netip.ParseAddr(o.SystemSOCKS.Listen)
	if err != nil || !listen.IsLoopback() {
		return nil, fmt.Errorf("netcfg: system SOCKS listener %q must be a loopback IP literal", o.SystemSOCKS.Listen)
	}
	if o.SystemSOCKS.Port == 0 {
		return nil, fmt.Errorf("netcfg: the system SOCKS port is zero")
	}
	if !f.SystemSOCKSKnown {
		return nil, &PlanError{
			Err:  ErrSystemSOCKSStateUnknown,
			User: "Caspian could not read this Mac's current proxy settings, so it left them unchanged instead of making a change it could not undo.",
		}
	}
	if len(f.SystemSOCKS) == 0 {
		return nil, &PlanError{
			Err:  fmt.Errorf("%w: no enabled network services", ErrSystemSOCKSStateUnknown),
			User: "This Mac has no enabled network service whose proxy setting Caspian can use.",
		}
	}

	states := make([]SystemSOCKSState, len(f.SystemSOCKS))
	for i, state := range f.SystemSOCKS {
		if state.Service == "" {
			return nil, &PlanError{
				Err:  fmt.Errorf("%w: an enabled network service has no name", ErrSystemSOCKSStateUnknown),
				User: "Caspian could not identify one of this Mac's network services, so it left the proxy settings unchanged.",
			}
		}
		if state.Authenticated {
			return nil, &PlanError{
				Err:  fmt.Errorf("%w on service %q", ErrAuthenticatedSystemSOCKS, state.Service),
				User: "An existing authenticated SOCKS proxy is configured on this Mac. Caspian left it unchanged because macOS does not reveal the password needed to restore it.",
			}
		}
		hasServer := state.Server != ""
		hasPort := state.Port != 0
		if hasServer != hasPort || (state.Enabled && !hasServer) {
			return nil, &PlanError{
				Err:  fmt.Errorf("%w: inconsistent endpoint on service %q", ErrSystemSOCKSStateUnknown, state.Service),
				User: "One of this Mac's existing SOCKS proxy settings is incomplete, so Caspian left it unchanged instead of guessing how to restore it.",
			}
		}
		states[i] = cloneSystemSOCKSState(state)
	}
	return states, nil
}

func cloneSystemSOCKSState(in SystemSOCKSState) SystemSOCKSState {
	out := in
	out.BypassDomains = append([]string(nil), in.BypassDomains...)
	return out
}

var caspianSystemSOCKSBypass = []string{
	"localhost",
	"127.0.0.1",
	"::1",
	"*.local",
	"169.254/16",
}

func mergedSystemSOCKSBypass(existing []string) []string {
	out := append([]string(nil), existing...)
	for _, required := range caspianSystemSOCKSBypass {
		found := false
		for _, have := range out {
			if strings.EqualFold(have, required) {
				found = true
				break
			}
		}
		if !found {
			out = append(out, required)
		}
	}
	return out
}

func systemSOCKSStateCommand(service string, enabled bool, why string) Command {
	state := "off"
	if enabled {
		state = "on"
	}
	return Command{
		Path: BinNetworksetup,
		Args: []string{"-setsocksfirewallproxystate", service, state},
		Why:  why,
	}
}

func systemSOCKSEndpointCommand(service, server string, port uint16, why string) Command {
	return Command{
		Path: BinNetworksetup,
		Args: []string{"-setsocksfirewallproxy", service, server, strconv.Itoa(int(port)), "off"},
		Why:  why,
	}
}

func systemSOCKSBypassCommand(service string, domains []string, why string) Command {
	args := []string{"-setproxybypassdomains", service}
	if len(domains) == 0 {
		args = append(args, "Empty")
	} else {
		args = append(args, domains...)
	}
	return Command{Path: BinNetworksetup, Args: args, Why: why}
}

// darwinSystemSOCKSSteps points each enabled network service at Caspian only
// after the engine has started. The apparently redundant state changes are
// load-bearing: -setsocksfirewallproxy turns a proxy on as a side effect. By
// disabling first and restoring the original state last during reverse-order
// teardown, both an originally-on and an originally-off proxy round-trip.
func (p *Plan) darwinSystemSOCKSSteps() []Step {
	if !p.Opts.SystemSOCKS.Enabled {
		return nil
	}
	var steps []Step
	for _, previous := range p.SystemSOCKS {
		service := previous.Service
		disableWhy := "pause the previous SOCKS proxy before replacing its endpoint with Caspian's listener"
		steps = append(steps, Step{
			Op:   OpProxy,
			Why:  disableWhy,
			Do:   systemSOCKSStateCommand(service, false, disableWhy),
			Undo: systemSOCKSStateCommand(service, previous.Enabled, "restore the SOCKS proxy state read before Caspian started"),
		})

		bypassWhy := "keep loopback and link-local destinations local while macOS applications use Caspian's SOCKS proxy"
		steps = append(steps, Step{
			Op:   OpProxy,
			Why:  bypassWhy,
			Do:   systemSOCKSBypassCommand(service, mergedSystemSOCKSBypass(previous.BypassDomains), bypassWhy),
			Undo: systemSOCKSBypassCommand(service, previous.BypassDomains, "restore the proxy bypass domains read before Caspian started"),
		})

		endpointWhy := "point this macOS network service at the SOCKS listener created by Caspian's running engine"
		undoEndpoint := systemSOCKSStateCommand(service, false, "the service previously had no SOCKS endpoint; leave it disabled")
		if previous.Server != "" && previous.Port != 0 {
			undoEndpoint = systemSOCKSEndpointCommand(service, previous.Server, previous.Port, "restore the SOCKS endpoint read before Caspian started")
		}
		steps = append(steps, Step{
			Op:   OpProxy,
			Why:  endpointWhy,
			Do:   systemSOCKSEndpointCommand(service, p.Opts.SystemSOCKS.Listen, p.Opts.SystemSOCKS.Port, endpointWhy),
			Undo: undoEndpoint,
		})

		enableWhy := "let proxy-aware macOS applications use the configured Caspian tunnel"
		steps = append(steps, Step{
			Op:   OpProxy,
			Why:  enableWhy,
			Do:   systemSOCKSStateCommand(service, true, enableWhy),
			Undo: systemSOCKSStateCommand(service, false, "disable Caspian's system SOCKS proxy before restoring the previous endpoint"),
		})
	}
	return steps
}
