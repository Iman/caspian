// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package netcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// windowsBackend is Windows 11.
//
// Untagged for the same reason darwinnet.go is: the commands a Windows plan
// turns into are generated and tested on every development machine. Only
// exec_windows.go, the runner that carries them out, is built on Windows.
//
// There is no shell and no program on Windows that this backend runs. Its
// commands name three PSEUDO-BINARIES that the Windows runner implements in
// process: "iphlpapi" (the IP Helper API: adapters, addresses, routes,
// per-interface forwarding and metric), "wfp" (the Windows Filtering Platform:
// the fail-closed filters) and "wintun" (legacy adapter recovery; the engine
// now owns adapter creation). They are Commands all the same,
// so the journal of inverses, the idempotence rules, the Applier's two-pass
// replay and RecordingRunner all work on them unchanged. That is the point of
// keeping Command as the unit rather than adding a second kind of step.
//
// Decisions recorded on 2026-09-03 with the owner:
//   - The access point is Mobile Hotspot (see internal/hotspot). Its subnet is
//     192.168.137.0/24 and cannot be chosen, so the plan is pinned to it.
//   - Windows has no per-source routing, so the whole host is tunnelled: the
//     default route goes through the tunnel adapter and the engine's own
//     connection to the server is kept off it by a pinned host route (and, in
//     the engine document, by binding its outbounds to the uplink adapter).
//   - Fail closed is enforced by WFP filters at the IP forwarding layer, not by
//     Windows Firewall rules, which never see forwarded traffic.
type windowsBackend struct{}

func init() { registerBackend(windowsBackend{}) }

func (windowsBackend) Platform() Platform { return PlatformWindows }

// The pseudo-binaries. See the package comment above.
const (
	// BinIPHelper: "adapters" (read everything; stdout is WindowsInventory as
	// JSON), "route add|delete <prefix> dev <alias> [via <gateway>] metric <n>",
	// "addr add|delete <prefix> dev <alias>", "iface set <alias> forwarding
	// on|off", "iface set <alias> metric <n>|auto".
	BinIPHelper = "iphlpapi"
	// BinWFP: "load" with the filter set as JSON on stdin (one transaction),
	// "flush" removes every filter this appliance added.
	BinWFP = "wfp"
	// BinWintun: "create <name>" and "delete <name>".
	BinWintun = "wintun"
)

var windowsAllowedBinaries = map[string]bool{
	BinIPHelper: true,
	BinWFP:      true,
	BinWintun:   true,
}

func (windowsBackend) AllowedBinaries() map[string]bool { return windowsAllowedBinaries }

// WindowsHotspotSubnet is the network Mobile Hotspot serves. Internet
// Connection Sharing fixes the host at 192.168.137.1 and hands out the rest;
// the only override is a registry value that needs a reboot, so the plan is
// pinned to what Windows will do rather than told what it should.
var WindowsHotspotSubnet = netip.MustParsePrefix("192.168.137.0/24")

// FixedHotspotSubnet is read by internal/privsvc, which pins Options to it.
func (windowsBackend) FixedHotspotSubnet() netip.Prefix { return WindowsHotspotSubnet }

// windowsForwardingKnob is the pseudo knob for per-interface IPv4 forwarding,
// recorded per adapter so that the change has an inverse.
func windowsForwardingKnob(alias string) string { return "forwarding." + alias }

func (windowsBackend) BaseSysctlKnobs() []string  { return nil }
func (windowsBackend) SysctlKnobs(*Plan) []string { return nil }

// RegulatoryDomain is not readable through anything this backend runs; the
// service falls back to its configured country (cmd/caspian reads the
// system's region).
func (windowsBackend) RegulatoryDomain(context.Context, Runner) (string, bool) { return "", false }

// ---------------------------------------------------------------------------
// Detection
// ---------------------------------------------------------------------------

// WindowsInventory is what "iphlpapi adapters" prints. The runner fills it
// from GetAdaptersAddresses, GetIpForwardTable2 and GetIpInterfaceEntry; the
// backend reads it into Facts. Keeping the boundary as JSON on stdout means a
// RecordingRunner can play a Windows machine from a fixture.
type WindowsInventory struct {
	Adapters []WindowsAdapter      `json:"adapters"`
	Defaults []WindowsDefaultRoute `json:"defaults"`
}

// GetIfTable2Ex includes remembered interfaces for removed hardware. Only
// adapters also exposed by GetAdaptersAddresses are usable candidates. Keep
// disconnected radios: a radio does not need an address to host Mobile Hotspot.
func (inv *WindowsInventory) retainPresentAdapters(present map[int]bool) {
	kept := inv.Adapters[:0]
	for _, adapter := range inv.Adapters {
		if present[adapter.Index] {
			kept = append(kept, adapter)
		}
	}
	inv.Adapters = kept
}

// WindowsAdapter is one adapter as the IP Helper API describes it.
type WindowsAdapter struct {
	// Alias is the friendly name ("Ethernet", "Wi-Fi", "Local Area Connection* 2").
	// Every command names adapters by alias, which is what Go's net package
	// also uses on Windows.
	Alias string `json:"alias"`
	Index int    `json:"index"`
	// Type is "ethernet", "wifi", "loopback", "tunnel" or "other", from the
	// IANA interface type.
	Type string `json:"type"`
	Up   bool   `json:"up"`
	// USB is best effort from the PnP instance id starting with USB.
	USB bool `json:"usb,omitempty"`
	// WiFiDirect marks the Microsoft Wi-Fi Direct Virtual Adapter that Mobile
	// Hotspot serves clients on. It is not a radio of its own.
	WiFiDirect bool     `json:"wifiDirect,omitempty"`
	Prefixes   []string `json:"prefixes,omitempty"`
	// Forwarding is the adapter's IPv4 forwarding flag.
	Forwarding bool `json:"forwarding"`
}

// WindowsDefaultRoute is one 0.0.0.0/0 or ::/0 row of the forwarding table.
type WindowsDefaultRoute struct {
	Alias   string `json:"alias"`
	Gateway string `json:"gateway,omitempty"`
	Metric  int    `json:"metric"`
	Family  int    `json:"family"`
	Up      bool   `json:"up"`
}

func (windowsBackend) Detect(ctx context.Context, r Runner, knobs []string) (Facts, error) {
	return windowsDetect(ctx, r, knobs)
}

func windowsDetect(ctx context.Context, r Runner, _ []string) (Facts, error) {
	f := Facts{CapturedAt: time.Now().UTC(), Sysctl: map[string]string{}}
	res, err := r.Run(ctx, Command{
		Path: BinIPHelper, Args: []string{"adapters"},
		Why: "adapters, their addresses, the default routes and the forwarding flags, read once",
	})
	if err != nil {
		return f, fmt.Errorf("netcfg: read adapters: %w", err)
	}
	inv, err := ParseWindowsInventory(res.Stdout)
	if err != nil {
		return f, err
	}
	for _, a := range inv.Adapters {
		l := Link{Name: a.Alias, State: "DOWN"}
		if a.Up {
			l.State = "UP"
		}
		if a.USB {
			l.Bus = "usb"
		}
		for _, p := range a.Prefixes {
			if pp, err := netip.ParsePrefix(p); err == nil {
				l.Prefixes = append(l.Prefixes, pp)
			}
		}
		f.Links = append(f.Links, l)
		if a.Forwarding {
			f.Sysctl[windowsForwardingKnob(a.Alias)] = "1"
		} else {
			f.Sysctl[windowsForwardingKnob(a.Alias)] = "0"
		}
		if a.Type != "wifi" || a.WiFiDirect {
			continue
		}
		w := WirelessIface{Name: a.Alias, Phy: windowsPhyName(a.Alias), Type: "managed", Manager: ManagedByNothing}
		f.Wireless = append(f.Wireless, w)
		f.Phys = append(f.Phys, windowsPhy(w.Phy))
	}
	for _, d := range inv.Defaults {
		dr := DefaultRoute{Dev: d.Alias, Metric: d.Metric, Family: d.Family, LinkDown: !d.Up}
		if d.Gateway != "" {
			if g, err := netip.ParseAddr(d.Gateway); err == nil {
				dr.Gateway = g
			}
		}
		if dr.Family == 0 {
			dr.Family = 4
		}
		f.Routes = append(f.Routes, dr)
	}
	return f, nil
}

// ParseWindowsInventory decodes what "iphlpapi adapters" printed.
func ParseWindowsInventory(out string) (WindowsInventory, error) {
	var inv WindowsInventory
	dec := json.NewDecoder(strings.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&inv); err != nil {
		return inv, fmt.Errorf("netcfg: the adapter inventory could not be read: %w", err)
	}
	return inv, nil
}

func windowsPhyName(alias string) string { return "radio-" + alias }

// windowsPhy models a Wi-Fi adapter under Mobile Hotspot: it can be joined to
// a network and host the hotspot at the same time (Windows documents the
// concurrency, and the band interference between the two), so the combination
// lists both. Channel is not selectable; the band is. The channels listed are
// the ones a band choice can land on, so the planner has something to pin.
func windowsPhy(name string) Phy {
	var b24, b5 Band
	b24.Number = 1
	for ch := 1; ch <= 11; ch++ {
		b24.Frequencies = append(b24.Frequencies, Frequency{MHz: 2407 + 5*ch, Channel: ch})
	}
	b5.Number = 2
	for _, ch := range []int{36, 40, 44, 48, 149, 153, 157, 161} {
		b5.Frequencies = append(b5.Frequencies, Frequency{MHz: 5000 + 5*ch, Channel: ch})
	}
	return Phy{
		Name:  name,
		Modes: []string{"managed", "AP"},
		Combinations: []Combination{{
			Raw:      "#{ managed } <= 1, #{ AP } <= 1, total <= 2, #channels <= 1",
			Limits:   []ComboLimit{{Max: 1, Types: []string{"managed"}}, {Max: 1, Types: []string{"AP"}}},
			Total:    2,
			Channels: 1,
		}},
		Bands: []Band{b24, b5},
	}
}

// ---------------------------------------------------------------------------
// Steps
// ---------------------------------------------------------------------------

// OpWFP and OpWintun are the journal ops for the two Windows-only changes.
const (
	OpWFP    = "wfp"
	OpWintun = "wintun"
)

// Block forwarding before the engine creates its adapter. Xray owns the
// Wintun handle; precreating the same GUID makes its CreateAdapter call fail.
func (p *Plan) windowsPreEngineSteps(current map[string]string) []Step {
	steps := []Step{p.windowsCutStep()}
	steps = append(steps, p.windowsServerRouteSteps()...)
	return steps
}

// windowsPostEngineSteps: address, metric and the default route on the tunnel
// adapter, once the engine has opened it.
func (p *Plan) windowsPostEngineSteps(current map[string]string) []Step {
	var steps []Step
	permit := p.windowsFirewallStep()
	// Rollback reinstates the block. Only the original pre-engine step
	// removes it, after all later network changes have been undone.
	permit.Undo = p.windowsCutStep().Do
	steps = append(steps, permit, p.windowsForwardingStep(p.Tun, current))
	addr := netip.PrefixFrom(p.TunAddr, p.TunSubnet.Bits()).String()
	why := "the tunnel adapter needs an address to be routable; the engine on Windows assigns none"
	steps = append(steps, Step{
		Op:   OpAddr,
		Why:  why,
		Do:   Command{Path: BinIPHelper, Args: []string{"addr", "add", addr, "dev", p.Tun}, Why: why},
		Undo: Command{Path: BinIPHelper, Args: []string{"addr", "delete", addr, "dev", p.Tun}, Why: "remove the tunnel address"},
	})
	why = "the tunnel's routes must beat the uplink's: interface metric 0"
	steps = append(steps, Step{
		Op:   OpLink,
		Why:  why,
		Do:   Command{Path: BinIPHelper, Args: []string{"iface", "set", p.Tun, "metric", "0"}, Why: why},
		Undo: Command{Path: BinIPHelper, Args: []string{"iface", "set", p.Tun, "metric", "auto"}, Why: "back to an automatic metric"},
	})
	why = "the default route through the tunnel; the whole host is tunnelled because Windows has no per-source routing"
	steps = append(steps, Step{
		Op:   OpRoute,
		Why:  why,
		Do:   Command{Path: BinIPHelper, Args: []string{"route", "add", "0.0.0.0/0", "dev", p.Tun, "metric", "0"}, Why: why},
		Undo: Command{Path: BinIPHelper, Args: []string{"route", "delete", "0.0.0.0/0", "dev", p.Tun, "metric", "0"}, Why: "remove the tunnel default route"},
	})
	return steps
}

func (p *Plan) windowsForwardingStep(alias string, current map[string]string) Step {
	knob := windowsForwardingKnob(alias)
	why := "a machine that shares its connection is a router, and on Windows forwarding is switched per interface"
	s := Step{
		Op:  OpSysctl,
		Why: why,
		Do:  Command{Path: BinIPHelper, Args: []string{"iface", "set", alias, "forwarding", "on"}, Why: why},
	}
	if prev, ok := current[knob]; ok {
		val := "off"
		if prev == "1" {
			val = "on"
		}
		s.Undo = Command{Path: BinIPHelper, Args: []string{"iface", "set", alias, "forwarding", val}, Why: "restore the value read before the change"}
		return s
	}
	s.Why += " (no previous value was read for " + alias + ", so this change has no recorded inverse)"
	return s
}

func (p *Plan) windowsServerRouteSteps() []Step {
	var steps []Step
	for _, s := range p.ServerAddr {
		if !p.canPin(s) || s.Is6() {
			continue
		}
		prefix := netip.PrefixFrom(s, 32).String()
		why := "the engine's own connection to the server must leave by the uplink, or the tunnel would try to reach itself"
		args := []string{"route", "add", prefix, "dev", p.Uplink}
		if p.UplinkGateway.IsValid() && !p.UplinkOnLink {
			args = append(args, "via", p.UplinkGateway.String())
		}
		args = append(args, "metric", "1")
		undo := append([]string{"route", "delete"}, args[2:]...)
		steps = append(steps, Step{
			Op:   OpRoute,
			Why:  why,
			Do:   Command{Path: BinIPHelper, Args: args, Why: why},
			Undo: Command{Path: BinIPHelper, Args: undo, Why: "remove the pinned server route"},
		})
	}
	return steps
}

// WindowsFilterSet is what "wfp load" reads on stdin: enough for the runner to
// build the provider, sublayer and filters, and for RunnerKey to tell a
// normal set from a cut one.
type WindowsFilterSet struct {
	// Hotspot is the client subnet; Tun the adapter forwarded traffic may leave by.
	Hotspot string `json:"hotspot"`
	Tun     string `json:"tun"`
	// Forward is "normal" or "cut".
	Forward string `json:"forward"`
	// PanelPort is permitted from the hotspot to the host.
	PanelPort int `json:"panelPort"`
}

func (p *Plan) windowsFilterSet(state ForwardState) string {
	fs := WindowsFilterSet{Hotspot: p.HotspotSubnet.String(), Tun: p.Tun, Forward: "normal", PanelPort: p.Opts.PanelPort}
	if state == ForwardCut {
		fs.Forward = "cut"
	}
	b, _ := json.Marshal(fs)
	return string(b) + "\n"
}

func (p *Plan) windowsFirewallStep() Step {
	why := "install the fail-closed filters, which must be in force before anything can be forwarded"
	return Step{
		Op:   OpWFP,
		Why:  why,
		Do:   Command{Path: BinWFP, Args: []string{"load"}, Stdin: p.windowsFilterSet(ForwardNormal), Why: why},
		Undo: Command{Path: BinWFP, Args: []string{"flush"}, Why: "remove the generated filters"},
	}
}

func (p *Plan) windowsCutStep() Step {
	why := "cut forwarded client traffic without taking the hotspot down, so a joined device stays " +
		"joined and can still reach the panel to turn it back on"
	return Step{
		Op:   OpWFP,
		Why:  why,
		Do:   Command{Path: BinWFP, Args: []string{"load"}, Stdin: p.windowsFilterSet(ForwardCut), Why: why},
		Undo: Command{Path: BinWFP, Args: []string{"flush"}, Why: "remove the generated filters"},
	}
}

func (p *Plan) windowsRestoreStep() Step {
	why := "resume forwarding client traffic through the tunnel"
	return Step{
		Op:   OpWFP,
		Why:  why,
		Do:   Command{Path: BinWFP, Args: []string{"load"}, Stdin: p.windowsFilterSet(ForwardNormal), Why: why},
		Undo: Command{Path: BinWFP, Args: []string{"flush"}, Why: "remove the generated filters"},
	}
}

func (windowsBackend) PreEngineSteps(p *Plan, current map[string]string) []Step {
	return p.windowsPreEngineSteps(current)
}
func (windowsBackend) PostEngineSteps(p *Plan, current map[string]string) []Step {
	return p.windowsPostEngineSteps(current)
}
func (windowsBackend) CutStep(p *Plan) Step     { return p.windowsCutStep() }
func (windowsBackend) RestoreStep(p *Plan) Step { return p.windowsRestoreStep() }

// ---------------------------------------------------------------------------
// Readbacks
// ---------------------------------------------------------------------------

// windowsHotspotAdapter finds the Wi-Fi Direct virtual adapter Mobile Hotspot
// serves clients on, from a fresh inventory.
func windowsHotspotAdapter(ctx context.Context, r Runner, gateway netip.Addr) (WindowsAdapter, bool, error) {
	res, err := r.Run(ctx, Command{
		Path: BinIPHelper, Args: []string{"adapters"},
		Why: "read back what the interface actually is, rather than trusting that a command to change it succeeded",
	})
	if err != nil {
		return WindowsAdapter{}, false, fmt.Errorf("netcfg: read adapters: %w", err)
	}
	inv, err := ParseWindowsInventory(res.Stdout)
	if err != nil {
		return WindowsAdapter{}, false, err
	}
	// The active Mobile Hotspot adapter is the interface that owns the
	// configured gateway. Some Windows drivers do not describe this virtual
	// interface as "Wi-Fi Direct", so the address is the stronger readback.
	if gateway.IsValid() {
		for _, a := range inv.Adapters {
			for _, s := range a.Prefixes {
				if p, err := netip.ParsePrefix(s); err == nil && p.Addr() == gateway {
					return a, true, nil
				}
			}
		}
	}
	var first WindowsAdapter
	found := false
	for _, a := range inv.Adapters {
		if !a.WiFiDirect {
			continue
		}
		if !found {
			first, found = a, true
		}
		// Windows keeps stale Wi-Fi Direct virtual adapters. Prefer the one
		// that is up and carries a non-link-local IPv4 address, which is the
		// adapter Mobile Hotspot is serving on now.
		if a.Up {
			for _, s := range a.Prefixes {
				if p, err := netip.ParsePrefix(s); err == nil && p.Addr().Is4() && !p.Addr().IsLinkLocalUnicast() {
					return a, true, nil
				}
			}
		}
	}
	return first, found, nil
}

// AssertHotspotInterfaceReleased on Windows checks the adapter clients will
// join, which Mobile Hotspot creates: absent is released, and present with an
// address outside the hotspot subnet is somebody else's network.
func (windowsBackend) AssertHotspotInterfaceReleased(ctx context.Context, r Runner, p *Plan) error {
	if p == nil || p.Hotspot == "" {
		return errors.New("netcfg: no hotspot interface to check")
	}
	a, present, err := windowsHotspotAdapter(ctx, r, netip.Addr{})
	if err != nil || !present {
		return err
	}
	for _, s := range a.Prefixes {
		pp, err := netip.ParsePrefix(s)
		if err != nil || pp.Addr().IsLinkLocalUnicast() {
			continue
		}
		if p.HotspotSubnet.IsValid() && p.HotspotSubnet.Contains(pp.Addr()) {
			continue
		}
		return fmt.Errorf("%w: the hotspot adapter %q already carries %s, which is not in the hotspot subnet %s",
			ErrHotspotNotReleased, a.Alias, pp, p.HotspotSubnet)
	}
	return nil
}

// AssertHotspotIsAccessPoint on Windows: the Wi-Fi Direct virtual adapter is
// up and carries the gateway. The SSID is read back by the access point
// driver from Mobile Hotspot itself, which reports it directly.
func (windowsBackend) AssertHotspotIsAccessPoint(ctx context.Context, r Runner, p *Plan, _ string) error {
	if p == nil || p.Hotspot == "" {
		return errors.New("netcfg: no hotspot interface to check")
	}
	a, present, err := windowsHotspotAdapter(ctx, r, p.HotspotGateway)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("%w: Mobile Hotspot has not created its adapter", ErrNotAccessPoint)
	}
	if !a.Up {
		return fmt.Errorf("%w: the hotspot adapter %q is not up", ErrNotAccessPoint, a.Alias)
	}
	for _, s := range a.Prefixes {
		if pp, err := netip.ParsePrefix(s); err == nil && pp.Addr() == p.HotspotGateway {
			return nil
		}
	}
	return fmt.Errorf("%w: the hotspot adapter %q does not carry the gateway address %s", ErrNotAccessPoint, a.Alias, p.HotspotGateway)
}

// windowsMetric parses the metric argument of a route or iface command; kept
// here beside the commands that use it so the runner and the tests agree.
func windowsMetric(s string) (uint32, bool) {
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}
