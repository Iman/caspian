// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"net/netip"
	"sort"
	"strings"
	"time"
)

// Facts is everything detection learned about the machine. It is a value with
// no behaviour that touches the system, so a Facts read from a fixture and a
// Facts read from a Raspberry Pi are the same thing to the planner.
type Facts struct {
	Links    []Link
	Routes   []DefaultRoute
	Wireless []WirelessIface
	Phys     []Phy

	// NetworkManagerPresent records that the nmcli device listing ANSWERED
	// and named at least one device.
	//
	// It is not the same question as "who owns this interface". An interface
	// can be reported unmanaged while NetworkManager is very much running,
	// and NetworkManager takes over devices that appear AFTER detection ran,
	// including the access point interface this package creates. So the
	// per-interface Manager cannot answer "will something claim a device that
	// does not exist yet"; this can.
	//
	// False means the listing failed, was empty, or nmcli is not installed.
	// Nothing is inferred from that beyond "do not run nmcli".
	NetworkManagerPresent bool

	// Sysctl holds the values read during detection, keyed by the full knob
	// name. The applier needs them to record the inverse of every change it
	// makes; a knob that could not be read is absent from the map.
	Sysctl map[string]string

	CapturedAt time.Time
}

// Link is one network interface as "ip -br addr" and "ip -d link" describe it.
type Link struct {
	Name     string
	State    string // UP, DOWN, UNKNOWN, as printed
	Prefixes []netip.Prefix

	// Bus is best effort. iproute2 prints "parentbus usb" for a USB adapter
	// and "parentbus platform" for the radio soldered to the board, but only
	// on kernels and iproute2 versions that expose it, so an empty string
	// means "not reported" and never "not USB".
	Bus string
}

// IsLoopback reports whether the link is the loopback device. The name is not
// hardcoded anywhere else in this package; this is the one place it is
// recognised, and it is recognised by its address rather than by being called
// "lo" where possible.
func (l Link) IsLoopback() bool {
	if l.Name == "lo" {
		return true
	}
	for _, p := range l.Prefixes {
		if p.Addr().IsLoopback() {
			return true
		}
	}
	return false
}

// DefaultRoute is one line of "ip route show default" or "ip -6 route show
// default".
type DefaultRoute struct {
	Family   int // 4 or 6
	Gateway  netip.Addr
	Dev      string
	Src      netip.Addr
	Proto    string
	Metric   int
	LinkDown bool

	// OnLink is true for a default route with no gateway, which happens on
	// point-to-point uplinks. The pinned host route to the user's server has
	// to be expressed without a "via" in that case.
	OnLink bool
}

// InterfaceManager is what owns a network interface, and therefore what has
// to let go of it before this package can use it for an access point.
type InterfaceManager string

const (
	// ManagedByUnknown means detection could not tell. It is the zero value
	// on purpose: an undetected manager must never be read as "nothing owns
	// this", which is the assumption that put a DHCP server on a stranger's
	// LAN.
	ManagedByUnknown InterfaceManager = ""
	// ManagedByNetworkManager means NetworkManager reports the device as
	// something other than unmanaged.
	ManagedByNetworkManager InterfaceManager = "NetworkManager"
	// ManagedByNothing means NetworkManager is present and reports the device
	// unmanaged, or NetworkManager is not present at all.
	ManagedByNothing InterfaceManager = "none"
)

// WirelessIface is one entry of "iw dev".
type WirelessIface struct {
	Name    string
	Phy     string // "phy0"
	Type    string // "managed", "AP", "monitor", ...
	SSID    string
	Channel int
	FreqMHz int
	MAC     string

	// Manager is what owns this interface. Detected, never assumed.
	Manager InterfaceManager

	// Associated is the answer "iw dev <if> link" gave, and LinkKnown says
	// whether it was asked and understood. Both are set by Detect.
	//
	// They exist because the alternatives INFER an association and this
	// MEASURES it. A channel is reported by an interface that is joined to
	// nothing, and an SSID can be absent from one that is mid-association, so
	// neither settles the question that matters before an access point is put
	// on a radio: is somebody's connection about to be broken.
	//
	// LinkKnown false means the probe failed or was not run, not that the
	// interface is free. StationLink falls back to the SSID there.
	Associated bool
	LinkKnown  bool
}

// The three states a wireless interface can be in here, all MEASURED on the
// target on 2026-08-30, kernel 6.18.34, brcmfmac:
//
//	station joined to a network      type managed, ssid HomeNet,         channel 10
//	access point with hostapd        type AP,      ssid Caspian-Probe,  channel 6
//	access point, nothing serving    type AP,      NO ssid,             channel 10
//	station joined to NOTHING        type managed, NO ssid,             channel 36
//
// The fourth was measured on 2026-08-30 and is the one that broke channel
// selection: an interface left over from a previous hotspot, typed back to
// managed, associated with nothing, still reporting the channel it last used.
//
// The third is the one this package creates: released from NetworkManager,
// stripped, typed with "iw dev X set type __ap", and not yet served. Note its
// channel. It is 10, the channel the STATION link was using before the
// release, and the driver keeps reporting it. It is stale and it means
// nothing: no access point is on that channel because no access point is
// running. Nothing may read Channel off an interface in AP mode to decide
// anything.
//
// An earlier predicate here asked "SSID is set OR a channel is reported" and
// called that Associated. It answered a different question depending on the
// interface type without saying so, and for the third state it answered TRUE:
// an interface this package had just freed and typed itself read as still in
// use. The readback that proves the release worked would have refused a
// correctly released interface on this hardware every time, and the appliance
// would never have started.
//
// The lesson is narrower than "the channel clause was wrong". Association is a
// property of a STATION. Asking it of an access point is a category error, and
// the fix is to make each predicate say which question it answers.

// IsAccessPoint reports whether the interface is in access point mode.
func (w WirelessIface) IsAccessPoint() bool {
	return strings.EqualFold(w.Type, "AP")
}

// StationLink reports whether this interface is a station joined to a network.
//
// A CHANNEL IS NOT EVIDENCE OF ONE, and this is the third place in this
// package that had to learn it. The readback learned it, the plan note learned
// it, and this predicate is where both of them read it from.
//
// The comment that stood here said the channel clause "guards against a real
// harm while the cost is nil". MEASURED on the target on 2026-08-30, the cost
// was not nil and the harm was the clause itself. wlan0, down, joined to
// nothing by all three sources:
//
//	iw dev wlan0 link            Not connected.
//	iw dev wlan0 info            type managed, channel 36, no ssid line
//	nmcli -t -f DEVICE,STATE     wlan0:disconnected
//
// reported channel 36 left over from the last time it hosted the hotspot. The
// clause read that as a live connection, so the planner pinned the access
// point to channel 36 and said so in a note asserting a connection that did
// not exist. The user had asked for 2.4GHz; 36 is 5GHz; hostapd was handed the
// contradiction and the start failed. When the same pin succeeded earlier it
// put the hotspot on a band the test handset cannot see at all.
//
// So the question is now asked of the machine. Associated comes from
// "iw dev <if> link", the same command the release readback uses. Where that
// was not asked or not understood the SSID is the fallback, which is what an
// association reports and what a freed interface does not.
func (w WirelessIface) StationLink() bool {
	if w.IsAccessPoint() {
		return false
	}
	if w.LinkKnown {
		return w.Associated
	}
	return w.SSID != ""
}

// InUse reports whether anything is currently using this interface, so that
// taking it over would disrupt somebody.
//
// For a station that is a link to a network. For an access point it is
// BROADCASTING, which means an SSID: an interface typed AP with no SSID is one
// nothing is serving, and on this hardware that is exactly the state a correct
// release leaves behind.
func (w WirelessIface) InUse() bool {
	if w.IsAccessPoint() {
		return w.SSID != ""
	}
	return w.StationLink()
}

// Phy is one radio, as "iw list" describes it.
type Phy struct {
	Name         string // "phy0"
	Index        int
	Modes        []string // Supported interface modes
	Commands     []string // Supported commands
	Combinations []Combination
	Bands        []Band
}

// Band is one frequency band of a radio.
type Band struct {
	Number      int
	Frequencies []Frequency
}

// Frequency is one channel the radio knows about, with the flags that decide
// whether an access point may actually use it.
type Frequency struct {
	MHz      int
	Channel  int
	MaxDBm   float64
	NoIR     bool // no initiating radiation: cannot start an AP here
	Disabled bool
	Radar    bool // radar detection required (DFS)
}

// Usable reports whether an access point may be started on this channel
// without radar detection.
func (f Frequency) Usable() bool { return !f.Disabled && !f.NoIR && !f.Radar }

// RadioBand names a frequency band an access point can run on.
//
// The two values are the strings the rest of the appliance already uses for
// this, so a band chosen in the panel travels to hostapd's hw_mode without
// being translated on the way and without a third spelling appearing here.
type RadioBand string

const (
	// BandAuto is the zero value: the user expressed no preference and this
	// package chooses. See chooseChannel for what it chooses and why.
	BandAuto RadioBand = ""
	// Band2GHz is 2.4 GHz, which hostapd calls hw_mode=g.
	Band2GHz RadioBand = "2.4GHz"
	// Band5GHz is 5 GHz, which hostapd calls hw_mode=a.
	Band5GHz RadioBand = "5GHz"
)

// BandOf classifies a frequency in MHz.
//
// It reads the FREQUENCY rather than the channel number, because the channel
// numbering has seams a range check gets wrong: channel 14 is 2484 MHz, and
// 6GHz numbering restarts at 1. Anything outside the two bands this appliance
// offers returns BandAuto, which means "neither", and such a channel is never
// selected for an explicit band.
func BandOf(mhz int) RadioBand {
	switch {
	case mhz >= 2400 && mhz <= 2500:
		return Band2GHz
	case mhz >= 4900 && mhz <= 5895:
		return Band5GHz
	}
	return BandAuto
}

// BandOfChannel says which band a channel number is on for THIS radio, by
// looking up the frequency the radio itself reported for it.
//
// The channel number alone is not enough: 6GHz numbering restarts at 1, so the
// same number means different things on different radios. Returns BandAuto
// when the radio does not list the channel at all.
func (p Phy) BandOfChannel(ch int) RadioBand {
	for _, b := range p.Bands {
		for _, f := range b.Frequencies {
			if f.Channel == ch {
				return BandOf(f.MHz)
			}
		}
	}
	return BandAuto
}

// UsableChannelsIn returns the usable channels of one band, lowest first.
func (p Phy) UsableChannelsIn(band RadioBand) []int {
	var out []int
	for _, b := range p.Bands {
		for _, f := range b.Frequencies {
			if !f.Usable() || f.Channel <= 0 {
				continue
			}
			if band != BandAuto && BandOf(f.MHz) != band {
				continue
			}
			out = append(out, f.Channel)
		}
	}
	sort.Ints(out)
	return out
}

// ComboLimit is one "#{ a, b } <= n" group of an interface combination.
type ComboLimit struct {
	Max   int
	Types []string
}

// Has reports whether the group covers the given interface type.
func (c ComboLimit) Has(t string) bool {
	for _, have := range c.Types {
		if strings.EqualFold(have, t) {
			return true
		}
	}
	return false
}

// Combination is one line of the "valid interface combinations" block. On the
// Raspberry Pi 5 built-in radio the interesting one reads
//
//	#{ managed } <= 1, #{ AP } <= 1, #{ P2P-client } <= 1, #{ P2P-device } <= 1,
//	total <= 4, #channels <= 1
//
// which says an access point may run beside the existing client link but is
// pinned to that link's channel. This type exists so the planner reads that
// from the radio rather than assuming it.
type Combination struct {
	Raw      string
	Limits   []ComboLimit
	Total    int
	Channels int
	Notes    []string
}

// Allows reports whether this combination permits one interface of each of the
// given types to exist at the same time.
//
// The check is an assignment problem rather than a membership test, because a
// group covers several types and its Max is the total across them:
// "#{ AP, mesh point } <= 8" allows eight access points and no station at all,
// so a naive "does any group mention managed, does any group mention AP" test
// answers yes and is wrong.
func (c Combination) Allows(types ...string) bool {
	if len(types) == 0 {
		return true
	}
	if c.Total > 0 && len(types) > c.Total {
		return false
	}
	remaining := make([]int, len(c.Limits))
	for i, l := range c.Limits {
		remaining[i] = l.Max
	}
	var assign func(i int) bool
	assign = func(i int) bool {
		if i == len(types) {
			return true
		}
		for g, l := range c.Limits {
			if remaining[g] <= 0 || !l.Has(types[i]) {
				continue
			}
			remaining[g]--
			if assign(i + 1) {
				return true
			}
			remaining[g]++
		}
		return false
	}
	return assign(0)
}

// SupportsAP reports whether the radio lists AP among its interface modes.
func (p Phy) SupportsAP() bool {
	for _, m := range p.Modes {
		if strings.EqualFold(m, "AP") {
			return true
		}
	}
	return false
}

// APWithStation reports whether an access point may run at the same time as a
// station on this radio, and returns the combination that permits it. The
// second return is the combination's #channels limit: 1 means the access point
// is pinned to the station's channel.
func (p Phy) APWithStation() (ok bool, combo Combination) {
	for _, c := range p.Combinations {
		if c.Allows("managed", "AP") {
			return true, c
		}
	}
	return false, Combination{}
}

// DeclaresAPWithStation is APWithStation's question without the combination,
// for the places that only need the yes or no.
//
// It is a DECLARATION, never a promise. Measured both ways on this project's
// hardware: the Pi 5's brcmfmac declares the combination and then refuses to
// create the interface while its station is associated, and the RTL8192EU
// dongle declares no combinations at all while declaring AP among its modes,
// which is a radio that can be an access point but not beside a station.
func (p Phy) DeclaresAPWithStation() bool {
	ok, _ := p.APWithStation()
	return ok
}

// UsableChannels returns the channels on which an access point may be started,
// lowest first. Channels needing radar detection are excluded: DFS makes an
// access point wait and then possibly move, which is not a sane default for an
// appliance whose whole job is to be reachable.
func (p Phy) UsableChannels() []int { return p.UsableChannelsIn(BandAuto) }

// LinkByName returns the link with the given name.
func (f Facts) LinkByName(name string) (Link, bool) {
	for _, l := range f.Links {
		if l.Name == name {
			return l, true
		}
	}
	return Link{}, false
}

// WirelessByName returns the wireless interface with the given name.
func (f Facts) WirelessByName(name string) (WirelessIface, bool) {
	for _, w := range f.Wireless {
		if w.Name == name {
			return w, true
		}
	}
	return WirelessIface{}, false
}

// PhyByName returns the radio with the given name.
func (f Facts) PhyByName(name string) (Phy, bool) {
	for _, p := range f.Phys {
		if p.Name == name {
			return p, true
		}
	}
	return Phy{}, false
}

// IsWireless reports whether the named interface is a wireless one.
func (f Facts) IsWireless(name string) bool {
	_, ok := f.WirelessByName(name)
	return ok
}

// PrimaryDefault returns the IPv4 default route the kernel will actually use:
// the lowest metric among routes whose device is not reported link-down.
//
// The interface carrying the default route is the uplink. It is discovered
// this way and never by name, because a Pi may present the wired port as eth0,
// end0 or enxMACADDR depending on the image and the predictable-names setting.
func (f Facts) PrimaryDefault() (DefaultRoute, bool) {
	best := DefaultRoute{}
	found := false
	for _, r := range f.Routes {
		if r.Family != 4 || r.LinkDown || r.Dev == "" {
			continue
		}
		if !found || r.Metric < best.Metric {
			best, found = r, true
		}
	}
	return best, found
}

// DefaultV6 returns the IPv6 default route with the lowest metric, if any. It
// is needed only to pin a host route to a server given as an IPv6 address.
func (f Facts) DefaultV6() (DefaultRoute, bool) {
	best := DefaultRoute{}
	found := false
	for _, r := range f.Routes {
		if r.Family != 6 || r.LinkDown || r.Dev == "" {
			continue
		}
		if !found || r.Metric < best.Metric {
			best, found = r, true
		}
	}
	return best, found
}

// OccupiedPrefixes returns every network the box is already on, which is what
// a chosen hotspot subnet must not collide with. Loopback and link-local are
// included on purpose: a hotspot subnet inside 169.254.0.0/16 or 127.0.0.0/8
// is broken for reasons that have nothing to do with collision.
func (f Facts) OccupiedPrefixes() []netip.Prefix {
	var out []netip.Prefix
	for _, l := range f.Links {
		for _, p := range l.Prefixes {
			out = append(out, p.Masked())
		}
	}
	for _, r := range f.Routes {
		if r.Gateway.IsValid() {
			out = append(out, netip.PrefixFrom(r.Gateway, r.Gateway.BitLen()))
		}
	}
	return out
}
