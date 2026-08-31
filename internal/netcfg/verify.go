// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// Reading the machine back, because starting something is not evidence it
// worked.
//
// MEASURED on the target on 2026-08-30: the service reported running with
// hotspot=wlan0 and the panel showed connected, while wlan0 was type managed,
// associated to the house network, on the station's channel, and broadcasting
// nothing. hostapd was alive; hostapd_cli STATUS timed out. A process being
// alive is the same class of evidence as a connect code: necessary, and not
// sufficient.
//
// These two checks are the readback. They belong to whatever supervises the
// hotspot, which is not this package; netcfg owns interfaces, so it provides
// the primitives and internal/hotspot decides when to call them and what to do
// when they fail. What must not happen is a box reporting itself up on the
// strength of a started process.

// ErrHotspotNotReleased is returned when the hotspot interface is still held
// by something, or still carries an address from another network.
var ErrHotspotNotReleased = errors.New("netcfg: the hotspot interface is not free")

// ErrHotspotInterfaceMissing is returned when the hotspot interface does not
// exist at all.
//
// It is separate from ErrHotspotNotReleased because it means the opposite
// thing about the machine and needs the opposite response: not-released means
// something else holds the interface, missing means the step that was supposed
// to create it did not run. MEASURED: "iw dev nosuchif link" exits 237 with
// "command failed: No such device (-19)" on stderr and nothing on stdout, so a
// parser that only looks for the word "Connected" reads a missing device as
// not-connected, therefore free, and lets the start proceed against an
// interface that is not there.
var ErrHotspotInterfaceMissing = errors.New("netcfg: the hotspot interface does not exist")

// ErrNotAccessPoint is returned when the hotspot interface is not in access
// point mode, or is not carrying the expected SSID.
var ErrNotAccessPoint = errors.New("netcfg: the hotspot interface is not an access point")

// ErrFirewallNotLoaded is returned when the generated table is not in the live
// ruleset at all.
var ErrFirewallNotLoaded = errors.New("netcfg: the generated firewall table is not loaded")

// ErrFirewallUnrecognised is returned when the table IS loaded but the live
// rules do not contain the leak block. It is a separate error from
// ErrFirewallNotLoaded on purpose: one means nothing is protecting the box and
// the other means something is, but not what this package generated.
var ErrFirewallUnrecognised = errors.New("netcfg: the loaded firewall table does not contain the expected leak block")

// AssertHotspotInterfaceReleased proves, against the live machine, that the
// hotspot interface is free before anything is allowed to bind to it.
//
// This is the check that would have stopped a DHCP server answering on a
// stranger's home network. Call it after applying the plan and before starting
// any service on the interface, and treat a failure as fatal: a box that
// cannot prove the interface is its own must not serve on it.
func AssertHotspotInterfaceReleased(ctx context.Context, r Runner, p *Plan) error {
	if p == nil || p.Hotspot == "" {
		return errors.New("netcfg: no hotspot interface to check")
	}
	// Whether this interface is a station is asked of "iw dev <if> link",
	// which answers it directly, and NOT inferred from a channel.
	//
	// MEASURED on the target 2026-08-30, on an interface the plan had just
	// created with "iw phy <phy> interface add ap0 type __ap":
	//
	//	iw dev captest info   ->  type AP
	//	iw dev captest link   ->  Not connected.
	//
	// and this check still called it "a station on channel 36". A freshly
	// created access point interface inherits the parent radio's channel, so
	// a channel line is ALWAYS present, so a test that treats a channel as
	// evidence of a station link always fails and the appliance never starts.
	//
	// That is the third time the same shape has cost this package: a channel
	// with no network name is not an association. It is fixed here by asking
	// the question that has a direct answer instead of adding another
	// heuristic on top of the channel.
	link, err := readLinkState(ctx, r, p.Hotspot)
	if err != nil {
		return err
	}
	if link.Connected {
		return fmt.Errorf("%w: %s is still joined to the network %q",
			ErrHotspotNotReleased, p.Hotspot, link.SSID)
	}

	// Something else already broadcasting on it is a different refusal. An
	// access point that is serving has an SSID; the one this package creates
	// has none until hostapd starts.
	w, err := readWireless(ctx, r, p.Hotspot)
	if err != nil {
		return err
	}
	if w.SSID != "" {
		return fmt.Errorf("%w: %s is already broadcasting %q",
			ErrHotspotNotReleased, p.Hotspot, w.SSID)
	}

	addrs, err := readAddrs(ctx, r, p.Hotspot)
	if err != nil {
		return err
	}
	for _, a := range addrs {
		if a.Addr().IsLinkLocalUnicast() || a.Addr().IsLoopback() {
			continue
		}
		if p.HotspotSubnet.IsValid() && p.HotspotSubnet.Contains(a.Addr()) {
			continue
		}
		return fmt.Errorf("%w: %s still carries %s, which is not in the hotspot subnet %s; "+
			"anything bound to this interface would have a path onto that network",
			ErrHotspotNotReleased, p.Hotspot, a, p.HotspotSubnet)
	}
	return nil
}

// ErrHotspotAddressMissing is returned when the hotspot interface does not
// carry the address the plan gave it.
var ErrHotspotAddressMissing = errors.New("netcfg: the hotspot address is not on the interface")

// AssertHotspotAddressPresent proves the box's own address is still on the
// hotspot interface, and it is meant to be called after the plan is applied
// and before anything is asked to bind to it.
//
// It exists because the failure it catches surfaced as an exit code from
// somebody else's program. MEASURED on the target on 2026-08-30: the address
// was added, NetworkManager claimed the newly created interface and flushed
// it, and the first thing that noticed was dnsmasq dying with "failed to
// create listening socket for 10.83.51.1: Cannot assign requested address",
// which the user was then shown as another program holding the address. It is
// the opposite: EADDRNOTAVAIL means the address is on no interface at all.
//
// What this deliberately does NOT check is the operational state. An access
// point interface reads DOWN until hostapd starts and holds a bindable
// address while it does, so requiring UP here would refuse a machine that is
// working. Same class of mistake as reading a channel as an association.
func AssertHotspotAddressPresent(ctx context.Context, r Runner, p *Plan) error {
	if p == nil || p.Hotspot == "" {
		return errors.New("netcfg: no hotspot interface to check")
	}
	if !p.HotspotGateway.IsValid() {
		return errors.New("netcfg: the plan has no hotspot address to check for")
	}
	addrs, err := readAddrs(ctx, r, p.Hotspot)
	if err != nil {
		return err
	}
	for _, a := range addrs {
		if a.Addr() == p.HotspotGateway {
			return nil
		}
	}
	return fmt.Errorf("%w: %s should hold %s and holds %s; "+
		"anything asked to serve on it would fail to bind",
		ErrHotspotAddressMissing, p.Hotspot, p.HotspotGateway, describePrefixes(addrs))
}

// describePrefixes renders what an interface actually carries, for a refusal
// message. "nothing" is the case that matters and it must not read as an
// empty gap in a sentence.
func describePrefixes(addrs []netip.Prefix) string {
	if len(addrs) == 0 {
		return "nothing"
	}
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		parts = append(parts, a.String())
	}
	return strings.Join(parts, ", ")
}

// AssertHotspotIsAccessPoint proves the interface is actually an access point
// carrying the expected SSID.
//
// ssid is what the hotspot was configured to broadcast. Pass "" to check only
// that the interface is in access point mode. PASS IT WHENEVER YOU HAVE IT:
// with an SSID this catches the failure that matters most on an unproven
// radio, which is hostapd exiting or refusing after the interface was already
// typed. That state reads back as type AP with no SSID, and without the SSID
// argument it reads back as a pass.
//
// WHAT THIS CANNOT PROVE, stated because a takeover on a radio nobody has run
// an access point on before is exactly where it matters: a radio can report
// type AP with the right SSID and put nothing on the air. Nothing the box can
// ask its own radio settles that, because a radio in AP mode does not scan for
// its own beacon. Only a SECOND device seeing the network settles it, which is
// the same shape as this project's rule that a connect is not a result. So a
// green readback here means the interface is configured as intended, and it
// does not mean anybody can see the hotspot.
//
// MEASURED for the built-in brcmfmac radio: it beacons, repeatedly, and a
// handset joins. NOT MEASURED for the RTL8192EU dongle, which declares AP
// among its modes and has never been asked to serve on this box.
func AssertHotspotIsAccessPoint(ctx context.Context, r Runner, p *Plan, ssid string) error {
	if p == nil || p.Hotspot == "" {
		return errors.New("netcfg: no hotspot interface to check")
	}
	w, err := readWireless(ctx, r, p.Hotspot)
	if err != nil {
		return err
	}
	if !strings.EqualFold(w.Type, "AP") {
		return fmt.Errorf("%w: %s reports type %q, not AP", ErrNotAccessPoint, p.Hotspot, w.Type)
	}
	if ssid != "" && w.SSID != ssid {
		return fmt.Errorf("%w: %s is an access point but broadcasts %q, not %q",
			ErrNotAccessPoint, p.Hotspot, w.SSID, ssid)
	}
	return nil
}

// readWireless returns the current state of one wireless interface.
//
// It reads "iw dev", the same command and the same parser detection uses, so
// there is no second output format to get wrong: the only capture of iw output
// this package has is of that command.
func readWireless(ctx context.Context, r Runner, name string) (WirelessIface, error) {
	res, err := r.Run(ctx, Command{
		Path: BinIw, Args: []string{"dev"},
		Why: "read back what the interface actually is, rather than trusting that a command to change it succeeded",
	})
	if err != nil {
		return WirelessIface{}, fmt.Errorf("netcfg: read wireless interfaces: %w", err)
	}
	ifaces, err := ParseIwDev(res.Stdout)
	if err != nil {
		return WirelessIface{}, err
	}
	for _, w := range ifaces {
		if w.Name == name {
			return w, nil
		}
	}
	return WirelessIface{}, fmt.Errorf("%w: %s is not in the wireless interface list at all", ErrNotAccessPoint, name)
}

// linkState is what "iw dev <if> link" reports: whether the interface is
// associated to a network, and which one.
//
// This is the only unambiguous answer to "is this interface a station right
// now". The alternatives all infer it: a channel is present on an access point
// too, and a type can be read wrong if the listing is parsed wrong. This
// command answers the question and nothing else.
type linkState struct {
	Connected bool
	SSID      string
}

// readLinkState asks whether one interface is associated.
//
// MEASURED on the target 2026-08-30, all four cases:
//
//	managed, down, not associated   "Not connected."            rc 0
//	freshly created AP vif          "Not connected."            rc 0
//	associated station              "Connected to <bssid> (on <if>)"
//	                                then a TAB-indented "SSID: <name>"   rc 0
//	device does not exist           nothing on stdout, rc 237,
//	                                "command failed: No such device (-19)"
//
// Three things follow. The first two cases are BYTE-IDENTICAL, which is
// correct: an access point with nothing serving it and an unassociated station
// mean the same thing here, that the interface is free. rc is 0 for "Not
// connected.", so exit status is not how absence of association is detected.
// And the network name comes from the SSID line, not from the first line,
// which carries the BSSID.
func readLinkState(ctx context.Context, r Runner, name string) (linkState, error) {
	res, err := r.Run(ctx, Command{
		Path: BinIw, Args: []string{"dev", name, "link"},
		Why: "whether this interface is joined to a network, asked directly rather than inferred from a channel",
	})
	if err != nil {
		// A missing device is its own answer, and it is not "free".
		if IsNotFound(res, err) {
			return linkState{}, fmt.Errorf("%w: %s", ErrHotspotInterfaceMissing, name)
		}
		return linkState{}, fmt.Errorf("netcfg: read link state of %s: %w", name, err)
	}
	st, err := parseIwLink(res.Stdout)
	if err != nil {
		return linkState{}, fmt.Errorf("netcfg: read link state of %s: %w", name, err)
	}
	return st, nil
}

// ErrLinkStateUnrecognised is returned when "iw dev <if> link" printed
// something with no verdict in it.
//
// It exists because the safe default is a REFUSAL, not "free". Both measured
// answers begin with a fixed word, so anything else means this parser and the
// tool have diverged, and treating silence as "not associated" would let the
// appliance bind to an interface whose state was never established.
var ErrLinkStateUnrecognised = errors.New("netcfg: could not tell from the link output whether the interface is associated")

// parseIwLink reads "iw dev <if> link".
//
// Every measured output starts with either "Not connected." or "Connected to
// <bssid> (on <if>)". Anything else is an error rather than a zero value.
func parseIwLink(out string) (linkState, error) {
	var st linkState
	verdict := false
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "Not connected"):
			return linkState{}, nil
		case strings.HasPrefix(line, "Connected to"):
			st.Connected = true
			verdict = true
		case strings.HasPrefix(line, "SSID:"):
			st.SSID = strings.TrimSpace(strings.TrimPrefix(line, "SSID:"))
		}
	}
	if !verdict {
		return linkState{}, fmt.Errorf("%w: %q", ErrLinkStateUnrecognised, strings.TrimSpace(out))
	}
	return st, nil
}

// readAddrs returns the addresses currently on one interface.
func readAddrs(ctx context.Context, r Runner, name string) ([]netip.Prefix, error) {
	res, err := r.Run(ctx, Command{
		Path: BinIP, Args: []string{"-br", "addr", "show", "dev", name},
		Why: "an address from another network left on this interface is a path onto that network",
	})
	if err != nil {
		return nil, fmt.Errorf("netcfg: read addresses on %s: %w", name, err)
	}
	links, err := ParseBriefAddr(res.Stdout)
	if err != nil {
		return nil, err
	}
	for _, l := range links {
		if l.Name == name {
			return l.Prefixes, nil
		}
	}
	return nil, nil
}

// AssertFirewallLoaded reads the live ruleset back and confirms this package's
// table is in force.
//
// It closes the last of the audit findings. Applier.Apply skips a step whose
// change is already recorded in the journal, and that is a claim about a file
// this package wrote, not about the kernel. A journal that survives while the
// ruleset does not, and there are several ways for that to happen (an "nft
// flush ruleset" from a shell, another tool replacing the ruleset, a partial
// restore), leaves the appliance believing it is protected while nothing is
// filtering. Nothing else in this package reads the firewall back.
//
// The discipline is the one AssertHotspotInterfaceReleased already applies to
// the interface: read the machine, do not trust that a command succeeded.
//
// Cost: one additional command execution per start, against roughly twenty the
// plan already runs, and it reads rather than changes. It is not measured on
// the target; the claim here is about the number of commands, not milliseconds.
//
// Two failures, kept separate because they mean different things:
//
//   - ErrFirewallNotLoaded: the table is absent. Nothing this package
//     generated is filtering anything.
//   - ErrFirewallUnrecognised: a table of that name is loaded but does not
//     carry the leak block. Something is in force and it is not this.
//
// NOT MEASURED: how nft renders these rules when listing them back. The
// existence check keys on the command's exit status, which is robust. The
// leak-block check matches on the interface names and the drop verb in the
// listed output, and that rendering has not been captured from the target. A
// rendering surprise therefore shows up as ErrFirewallUnrecognised with the
// live output attached, which is distinguishable from an absent table.
func AssertFirewallLoaded(ctx context.Context, r Runner, p *Plan) error {
	if p == nil || p.Hotspot == "" || p.Uplink == "" {
		return errors.New("netcfg: no plan to check the firewall against")
	}
	res, err := r.Run(ctx, Command{
		Path: BinNft, Args: []string{"list", "table", "inet", TableName},
		Why: "the journal says the firewall was loaded; this asks the kernel whether it still is",
	})
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrFirewallNotLoaded, TableName, strings.TrimSpace(res.Stderr))
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return fmt.Errorf("%w: %s listed nothing", ErrFirewallNotLoaded, TableName)
	}

	// The leak block is the rule the fail-closed guarantee rests on, and it
	// names only the hotspot and the uplink, so it is present whatever the
	// tunnel is doing.
	for _, line := range strings.Split(res.Stdout, "\n") {
		if !strings.Contains(line, "iifname") || !strings.Contains(line, "oifname") {
			continue
		}
		if strings.Contains(line, p.Hotspot) && strings.Contains(line, p.Uplink) && strings.Contains(line, "drop") {
			return nil
		}
	}
	return fmt.Errorf("%w: no rule dropping %s to %s in the live table; nft listed:\n%s",
		ErrFirewallUnrecognised, p.Hotspot, p.Uplink, strings.TrimSpace(res.Stdout))
}
