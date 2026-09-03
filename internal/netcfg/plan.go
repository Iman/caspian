// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// defaultAPIfaceName is the interface created for the access point when the
// radio already carries a station link. It is not the hotspot interface in
// general: where the radio is free, the interface already on it is used and
// this name never appears.
const defaultAPIfaceName = "ap0"

// Mode is which interface does what.
type Mode int

const (
	// ModeUnset is the zero value and never appears in a returned Plan.
	ModeUnset Mode = iota
	// ModeWiredUplink is mode A of the design: the internet arrives on a
	// non-wireless interface and the hotspot runs on a radio.
	ModeWiredUplink
	// ModeWirelessUplink is mode B: the internet arrives over WiFi and the
	// hotspot runs on a second radio, normally a USB adapter.
	ModeWirelessUplink
)

// String renders the mode for logs and for the panel's advanced view.
func (m Mode) String() string {
	switch m {
	case ModeWiredUplink:
		return "wired uplink"
	case ModeWirelessUplink:
		return "wireless uplink"
	default:
		return "unset"
	}
}

// RouteStrategy decides how client traffic is put onto the tunnel.
type RouteStrategy int

const (
	// StrategyPolicy is the default: a rule matching the hotspot subnet sends
	// client traffic to a second table whose default route is the tunnel. The
	// box's own traffic keeps using the main table, which is what lets the
	// engine reach the server at all.
	StrategyPolicy RouteStrategy = iota

	// StrategySplitDefault replaces the default route for everything on the
	// box with two half routes through the tunnel. It tunnels the box itself
	// as well as its clients. The pinned host route to the server is not
	// optional under this strategy: without it the engine's own connection
	// matches 0.0.0.0/1 and loops through the tunnel it is trying to build.
	StrategySplitDefault
)

// EgressPolicy decides whether the kill switch covers the box's own outbound
// traffic as well as its clients'.
type EgressPolicy int

const (
	// EgressRestricted is the default. The OUTPUT chain drops by policy and
	// names what may leave: replies to inbound connections, the tunnel, the
	// engine's connection to the server, and the few services the box needs
	// to stay on the network at all. Everything else on the Pi, apt included,
	// cannot reach the internet directly while the appliance is on.
	EgressRestricted EgressPolicy = iota

	// EgressOpen is the previous behaviour: the OUTPUT chain accepts by
	// policy and the promise covers forwarded client traffic only.
	//
	// It exists because a user on a network nobody has thought about needs a
	// way back that is not a rebuild. The failure modes of a restricted
	// egress are delayed rather than immediate, and a box that cannot renew
	// its DHCP lease loses its address hours later.
	EgressOpen
)

// ForwardState is whether client traffic is being forwarded at all.
type ForwardState int

const (
	// ForwardNormal is the ordinary state: client traffic is forwarded to the
	// tunnel and nowhere else.
	ForwardNormal ForwardState = iota

	// ForwardCut drops client traffic while the hotspot, DHCP, DNS and the
	// panel keep working, so a joined device stays joined and can still reach
	// the panel to turn it back on. Switching the appliance off would take
	// the hotspot down with it and disconnect the device the user is holding.
	//
	// It is runtime state and nothing here persists it. The ruleset is
	// regenerated and reloaded on every start, so a cut cannot survive a
	// restart unless a caller deliberately stores it, and netcfg does not.
	ForwardCut
)

// IPv6Policy decides what happens to client IPv6.
type IPv6Policy int

const (
	// IPv6Block is the default. There is no IPv6 tunnel, so a client with a
	// working IPv6 path would prefer it and bypass the tunnel completely.
	// Blocking it is the only fail-closed answer.
	IPv6Block IPv6Policy = iota
	// IPv6Forward carries client IPv6 to the tunnel. Only set this when the
	// tunnel actually carries IPv6, which the engine's TUN inbound has not
	// been shown to do on the target.
	IPv6Forward
)

// Options are the knobs the installer and the panel's advanced view can turn.
// Every automatic choice in this package is overridable, because detection
// that cannot be overridden fails silently on the one machine that does not
// match the assumption (design 5.4).
type Options struct {
	// TunName is the tunnel device the engine creates. The engine's JSON
	// surface defaults it to xray0.
	// Platform is the operating system this plan is for. Empty means Linux.
	// It decides which Backend turns the plan into commands and reads the
	// result back; see platform.go.
	Platform Platform

	TunName string

	HotspotPool []netip.Prefix
	TunnelPool  []netip.Prefix

	// HotspotSubnet, if set, overrides the pool and the collision check is
	// only reported rather than enforced.
	HotspotSubnet netip.Prefix

	// UplinkOverride and HotspotOverride name interfaces explicitly.
	// HotspotOverride accepts either an interface name or a radio name.
	UplinkOverride  string
	HotspotOverride string

	// HotspotBand is the band the user asked the access point to run on, or
	// BandAuto when they expressed no preference.
	//
	// An EXPLICIT band is never silently replaced. If the chosen radio has no
	// usable channel in it, or a station link pins the access point to a
	// channel outside it, the plan REFUSES with ErrBandUnavailable and says
	// which band and which radio. The alternative, which is what happened
	// before this field existed, is a channel from one band carrying the
	// label of the other: internal/privsvc took the channel from the plan and
	// the band from the request without checking that they agree, hostapd was
	// handed hw_mode=g with channel 36, and the user was told "the hotspot
	// failed" with nothing pointing at the band.
	HotspotBand RadioBand

	// APIfaceName is the interface created for the access point when the
	// chosen radio already carries a station link.
	APIfaceName string

	// Table is the routing table id used for tunnelled traffic, and RulePrio
	// is the priority of the ip rules that select it.
	Table    int
	RulePrio int

	// FwMark selects the tunnel table for traffic the box originates on
	// behalf of clients, which is how the resolver on the box sends client
	// queries through the tunnel while the engine's own socket does not.
	FwMark uint32

	// DNSPort is where the resolver on the box listens. Client DNS is
	// redirected to it rather than merely permitted.
	DNSPort int

	// PanelPort is the panel's listener, reachable from the hotspot only.
	PanelPort int

	Strategy RouteStrategy
	IPv6     IPv6Policy

	// Egress decides whether the box's own outbound traffic is restricted as
	// well as its clients'.
	Egress EgressPolicy

	// ClientIsolation stops hotspot clients reaching each other.
	ClientIsolation bool

	// MasqueradeToTunnel adds a source NAT on the way into the tunnel. It is
	// off by default: the engine's TUN inbound is a userspace netstack that
	// terminates TCP and UDP flows and dials out itself, so the client's
	// source address never reaches the wire and there is nothing to translate.
	// Turn it on only if the tunnel device is ever a real layer 3 interface.
	MasqueradeToTunnel bool

	// RPFilter is the value written to the reverse-path filter knobs. 2 is
	// loose mode, which still drops a packet with no route back at all while
	// allowing the asymmetry that a second routing table creates. 0 disables
	// the check entirely and is not the default because it gives up the
	// anti-spoof property for nothing.
	RPFilter int
}

// DefaultOptions returns the settings the appliance uses when the user has
// changed nothing.
func DefaultOptions() Options {
	return Options{
		TunName:     "xray0",
		APIfaceName: defaultAPIfaceName,
		HotspotPool: DefaultHotspotPool(),
		TunnelPool:  DefaultTunnelPool(),
		Table:       8410,
		RulePrio:    8410,
		FwMark:      0x20da,
		DNSPort:     53,
		PanelPort:   8088,
		Strategy:    StrategyPolicy,
		IPv6:        IPv6Block,
		// Restricted is also the zero value, so a caller that builds Options
		// by hand gets the safer posture rather than the open one.
		Egress:          EgressRestricted,
		ClientIsolation: true,
		RPFilter:        2,
	}
}

// PlanError is a refusal with wording fit for the panel beside it.
type PlanError struct {
	Err  error
	User string
}

func (e *PlanError) Error() string { return e.Err.Error() }
func (e *PlanError) Unwrap() error { return e.Err }

// UserMessage returns the plain-words version for the panel.
func (e *PlanError) UserMessage() string { return e.User }

// Refusals. Each is a distinct situation with a distinct remedy, so they are
// distinct errors rather than one "cannot plan".
var (
	ErrNoUplink              = errors.New("netcfg: no usable default route, so no uplink")
	ErrNoAPCapableInterface  = errors.New("netcfg: no wireless interface reports AP support")
	ErrAPConflictsWithUplink = errors.New("netcfg: the only AP-capable radio is carrying the uplink and cannot do both")
	ErrNoServerAddress       = errors.New("netcfg: no server address to pin a host route to")

	// ErrBandUnavailable is returned when the band the user asked for cannot
	// be honoured on the radio that will run the access point. It is a
	// refusal, never a substitution: a hotspot on a band the user did not
	// choose is one their devices may not be able to see, and that failure is
	// invisible from the box.
	ErrBandUnavailable = errors.New("netcfg: the radio has no usable channel in the band that was asked for")

	// ErrServerFamilyUnreachable is returned when every one of the user's
	// server addresses is in an address family this machine has no default
	// route for. It is a refusal rather than a note because the engine cannot
	// reach the server at all in that state: a server outside the local
	// network needs a default route, and without one the box would report
	// "connected" and carry nothing.
	ErrServerFamilyUnreachable = errors.New("netcfg: every server address is in a family with no default route on this machine")
)

// Plan is the decision. It holds no behaviour that touches the machine: it is
// turned into commands by PreEngineSteps and PostEngineSteps and into firewall
// text by Ruleset.
type Plan struct {
	// Platform is copied from Options at planning time, so a Plan carries the
	// backend that made its commands wherever it is handed.
	Platform Platform

	Mode Mode

	Uplink        string
	UplinkGateway netip.Addr
	UplinkOnLink  bool
	UplinkV6Gw    netip.Addr
	UplinkV6Dev   string

	Hotspot        string
	HotspotPhy     string
	HotspotSubnet  netip.Prefix
	HotspotGateway netip.Addr

	// HotspotIsVirtual reports that Hotspot names an interface this package
	// has to create on the radio, because the radio already carries a station
	// link that must not be disturbed. HotspotParent names that link.
	HotspotIsVirtual bool
	HotspotParent    string

	// HotspotFallback names the existing wireless interface the access point
	// takes over if creating a second one fails, or "" when there is nothing
	// safe to take. It is never the uplink: taking that would cut the box off
	// from the internet it exists to share.
	//
	// It is decided at plan time and acted on at apply time, because whether
	// the driver will create a second interface is not something the planner
	// can know. See Plan.HotspotTakeover.
	HotspotFallback string

	// HotspotTakenOver reports that this plan is the fallback: the access
	// point uses an interface that already existed. That interface has to be
	// released and stripped before anything binds to it.
	HotspotTakenOver bool

	// HotspotManager is what owns the hotspot interface, as detected. Any plan
	// whose value here is ManagedByNetworkManager releases the interface from
	// it before anything binds; see [Plan.HotspotReleaseSteps].
	//
	// It is set on every path that names an interface which ALREADY EXISTS: the
	// takeover, and the free interface a second radio offers. It is deliberately
	// ManagedByUnknown on the two paths where this package CREATES the
	// interface, because detection ran before that interface existed and no
	// value was measured for it. Guessing the parent radio's manager would be
	// attributing a measurement of one device to a different one, which is the
	// mistake that produced the rp_filter fixture recorded above SysctlKnobs.
	//
	// A created interface is NOT left alone by NetworkManager. That used to be
	// recorded here as an open live-machine question; it was MEASURED on the
	// target on 2026-08-30 and the answer is that NetworkManager claims it:
	//
	//	NetworkManager: device (ap0): driver supports Access Point (AP) mode
	//	NetworkManager: manager: (ap0): new 802.11 Wi-Fi device
	//	NetworkManager: device (ap0): state change: unmanaged -> unavailable
	//	                              (reason 'managed', managed-type: 'external')
	//	NetworkManager: device (ap0): state change: unavailable -> disconnected
	//	                              (reason 'supplicant-available', managed-type: 'full')
	//
	// and the address this package had just added went with it, which
	// avahi-daemon witnessed independently ("Withdrawing address record for
	// 10.83.51.1 on ap0"). dnsmasq then died with "failed to create listening
	// socket for 10.83.51.1: Cannot assign requested address".
	//
	// So the created path releases the interface too, gated on
	// NetworkManagerPresent rather than on a per-interface manager that
	// cannot exist for a device that has not been created yet. See
	// Plan.VirtualIfaceSteps.
	HotspotManager InterfaceManager

	// NetworkManagerPresent is copied from the facts. It gates the release of
	// an interface this package CREATES, where no per-interface manager was
	// or could have been measured.
	NetworkManagerPresent bool

	// HotspotStationPrefixes are the addresses the hotspot interface already
	// carries from the network it is joined to. They are removed before the
	// hotspot address is added, and restored on teardown.
	//
	// Leaving them is not cosmetic. On the target the interface kept
	// 10.0.0.222/24 from the house network alongside the hotspot address,
	// which gave the DHCP server a path onto that LAN, where it answered a
	// real device with DHCPNAK.
	HotspotStationPrefixes []netip.Prefix

	// Channel is the channel the access point must use. Pinned reports
	// whether that was forced by the radio rather than chosen: a radio whose
	// combination says "#channels <= 1" makes the access point follow the
	// station link, and a client roam that changes the station's channel takes
	// the access point with it.
	Channel       int
	ChannelPinned bool
	UsableChannel []int

	Tun        string
	TunSubnet  netip.Prefix
	TunAddr    netip.Addr
	TunPeer    netip.Addr
	ServerAddr []netip.Addr

	// UnpinnableServers lists the server addresses that get no pinned host
	// route, because this machine has no default route in their address
	// family and so no gateway to pin them through.
	//
	// It is a field rather than only a line in Notes so that a caller can act
	// on it. A note is prose: the panel can print it and nothing can branch on
	// it. The absence of an IPv6 default route is a normal state on a normal
	// box, not an error, and the code that has to cope with it should be able
	// to ask rather than parse English.
	UnpinnableServers []netip.Addr

	Opts Options

	// Notes carries anything an operator should know that did not stop the
	// plan, such as an overridden subnet that does collide.
	Notes []string
}

// PlanNetwork picks a mode and every address that follows from it.
//
// servers are the addresses of the user's proxy server, already resolved. They
// are needed here and not later because the pinned host route to each of them
// is part of the plan, and a plan with no server address is a plan that loops
// the engine through its own tunnel.
func PlanNetwork(f Facts, servers []netip.Addr, o Options) (*Plan, error) {
	if o.TunName == "" {
		o = DefaultOptions()
	}
	if len(o.HotspotPool) == 0 {
		o.HotspotPool = DefaultHotspotPool()
	}
	if len(o.TunnelPool) == 0 {
		o.TunnelPool = DefaultTunnelPool()
	}
	if !ValidInterfaceNameOn(o.Platform, o.TunName) {
		return nil, fmt.Errorf("netcfg: tunnel device name %q is not a valid interface name", o.TunName)
	}

	p := &Plan{Platform: o.Platform, Tun: o.TunName, Opts: o, NetworkManagerPresent: f.NetworkManagerPresent}

	// 1. The uplink is whichever interface carries the default route. The
	//    name is never assumed: a Pi presents its wired port as eth0, end0 or
	//    enx<mac> depending on the image and on predictable naming.
	def, ok := f.PrimaryDefault()
	if o.UplinkOverride != "" {
		found := false
		for _, r := range f.Routes {
			if r.Dev == o.UplinkOverride && r.Family == 4 {
				def, found, ok = r, true, true
				break
			}
		}
		if !found {
			return nil, &PlanError{
				Err:  fmt.Errorf("%w: override %q carries no default route", ErrNoUplink, o.UplinkOverride),
				User: "The interface chosen for the internet connection has no internet connection on it.",
			}
		}
	}
	if !ok {
		return nil, &PlanError{
			Err:  ErrNoUplink,
			User: "This machine has no internet connection yet. Plug in a network cable, or join a WiFi network, then try again.",
		}
	}
	p.Uplink = def.Dev
	p.UplinkGateway = def.Gateway
	p.UplinkOnLink = def.OnLink
	if v6, ok := f.DefaultV6(); ok {
		p.UplinkV6Gw = v6.Gateway
		p.UplinkV6Dev = v6.Dev
	}

	// 2. The hotspot interface comes from AP support, never from a name.
	if err := p.chooseHotspot(f, o); err != nil {
		return nil, err
	}

	// 3. Addresses that collide with nothing the box is already on.
	taken := f.OccupiedPrefixes()
	if o.HotspotSubnet.IsValid() {
		p.HotspotSubnet = o.HotspotSubnet.Masked()
		for _, t := range taken {
			if Overlaps(p.HotspotSubnet, t) {
				p.Notes = append(p.Notes, fmt.Sprintf(
					"the hotspot subnet %v was set by hand and overlaps %v, which this machine is already on", p.HotspotSubnet, t))
			}
		}
	} else {
		sub, err := ChooseSubnet(o.HotspotPool, taken)
		if err != nil {
			return nil, err
		}
		p.HotspotSubnet = sub
	}
	gw, err := GatewayAddr(p.HotspotSubnet)
	if err != nil {
		return nil, err
	}
	p.HotspotGateway = gw

	// Addresses to take OFF the hotspot interface before anything serves on
	// it, for the path where an interface that already exists is used as it
	// is. The takeover computes its own list; a created interface has nothing
	// on it yet.
	//
	// An interface joined to NOTHING can still be HOLDING an address, and that
	// address is a path onto somebody else's network for whatever binds here.
	// That is the DHCPNAK incident of 2026-08-30 arriving by a different door,
	// and the door only opened when a leftover channel stopped making every
	// such interface read as a station: before that, an interface in this
	// state was never chosen directly.
	//
	// This runs here rather than in acceptHotspot because the hotspot subnet
	// is not chosen yet at that point, and an address already inside it is
	// this appliance's own from a previous run. Removing that one to add it
	// back is churn with an inverse that outlives it.
	if p.Hotspot != "" && !p.HotspotIsVirtual {
		p.HotspotStationPrefixes = foreignPrefixes(f, p.Hotspot, p.HotspotSubnet)
	}

	tunSub, err := ChooseSubnet(o.TunnelPool, append(taken, p.HotspotSubnet))
	if err != nil {
		return nil, err
	}
	p.TunSubnet = tunSub
	if p.TunAddr, err = GatewayAddr(tunSub); err != nil {
		return nil, err
	}
	if p.TunPeer, err = PeerAddr(tunSub); err != nil {
		return nil, err
	}

	// 4. The server addresses the pinned host routes are built from.
	for _, s := range servers {
		if s.IsValid() {
			p.ServerAddr = append(p.ServerAddr, s.Unmap())
		}
	}
	for _, s := range p.ServerAddr {
		if p.canPin(s) {
			continue
		}
		p.UnpinnableServers = append(p.UnpinnableServers, s)
		p.Notes = append(p.Notes, fmt.Sprintf(
			"the server address %s is IPv6 and this machine has no IPv6 default route, so no host route can be "+
				"pinned for it; if a default route through the tunnel is ever installed for IPv6 the engine will loop", s))
	}
	if len(p.ServerAddr) > 0 && len(p.UnpinnableServers) == len(p.ServerAddr) {
		return nil, &PlanError{
			Err: fmt.Errorf("%w (%d address(es), all IPv6)", ErrServerFamilyUnreachable, len(p.ServerAddr)),
			User: "The proxy server can only be reached over IPv6, and this machine has no IPv6 internet " +
				"connection. Ask whoever gave you the configuration for an address that works over IPv4.",
		}
	}
	if len(p.ServerAddr) == 0 {
		return nil, &PlanError{
			Err: ErrNoServerAddress,
			User: "The proxy server's address could not be worked out from the configuration, " +
				"so the connection cannot be set up safely.",
		}
	}

	return p, nil
}

// chooseHotspot picks the radio that will run the access point, and decides
// the mode from whether the uplink is on a radio too.
//
// The rule it applies, and the reason for it:
//
// An access point never disturbs an existing station link. If the chosen radio
// already has an interface operating on a channel, the access point is added
// as a second virtual interface rather than by reconfiguring the one that is
// there, because reconfiguring it drops whatever that link is connected to,
// and on the target that link may be the uplink itself.
//
// Adding a second interface is only possible if the radio's own interface
// combinations permit a station and an access point at the same time, and a
// radio that permits it while limiting "#channels" to 1 pins the access point
// to the channel the station is already on. Both facts are read from the
// radio. On the target the built-in radio reports exactly that pair, so the
// access point follows the existing WiFi connection's channel, and a roam that
// moves that connection moves the hotspot with it.
func (p *Plan) chooseHotspot(f Facts, o Options) error {
	if _, uplinkIsWireless := f.WirelessByName(p.Uplink); uplinkIsWireless {
		p.Mode = ModeWirelessUplink
	} else {
		p.Mode = ModeWiredUplink
	}

	cands := apCandidates(f, p.Uplink)
	if len(cands) == 0 {
		return &PlanError{
			Err:  ErrNoAPCapableInterface,
			User: "No adapter on this machine can create a hotspot. Plug in a USB WiFi adapter.",
		}
	}

	if o.HotspotOverride != "" {
		for _, c := range cands {
			if c.iface.Name == o.HotspotOverride || c.phy.Name == o.HotspotOverride {
				return p.acceptHotspot(f, c, o)
			}
		}
		return &PlanError{
			Err:  fmt.Errorf("%w: %q was chosen by hand", ErrNoAPCapableInterface, o.HotspotOverride),
			User: "The WiFi adapter chosen for the hotspot cannot create one. Choose another, or plug in a USB WiFi adapter.",
		}
	}

	// Preference order, best first:
	//   1. a radio with no station link at all, because the access point then
	//      owns the radio and can use any channel it likes
	//   2. among those, a USB adapter, which is what mode B describes
	//   3. anything else that can hold a station and an access point together
	var free, shared, takeoverable, blocked []apCandidate
	for _, c := range cands {
		if c.station.Name == "" {
			free = append(free, c)
			continue
		}
		// A radio carrying a link that must COEXIST with the access point
		// needs a declared combination of managed and AP.
		if c.phy.DeclaresAPWithStation() {
			shared = append(shared, c)
			continue
		}
		// No declaration. The link can still be ENDED, and then there is
		// nothing to coexist with: the access point takes the interface over.
		// That is worse than sharing, because somebody's connection stops, so
		// it is preferred only after every sharing option.
		//
		// Never the uplink. Ending the connection the box exists to share is
		// the one outcome this package must not plan, and acceptHotspot
		// refuses it by name.
		if c.station.Name != p.Uplink {
			takeoverable = append(takeoverable, c)
			continue
		}
		blocked = append(blocked, c)
	}
	if len(free) > 0 {
		best := free[0]
		for _, c := range free {
			if c.bus == "usb" && best.bus != "usb" {
				best = c
			}
		}
		return p.acceptHotspot(f, best, o)
	}
	// Among radios that already carry a link, one whose link is NOT the
	// internet connection comes first, whether it will be shared or taken
	// over. Sharing beats taking over, because sharing keeps the connection.
	var sharedNotUplink, sharedUplink []apCandidate
	for _, c := range shared {
		if c.station.Name == p.Uplink {
			sharedUplink = append(sharedUplink, c)
			continue
		}
		sharedNotUplink = append(sharedNotUplink, c)
	}
	if len(sharedNotUplink) > 0 {
		return p.acceptHotspot(f, preferUSB(sharedNotUplink), o)
	}
	if len(takeoverable) > 0 {
		return p.acceptHotspot(f, preferUSB(takeoverable), o)
	}
	if len(shared) > 0 {
		// Among radios that already carry a link, prefer one whose link is
		// NOT the internet connection.
		//
		// Putting the access point beside the uplink is the worst of the
		// available options even when the radio declares it can: the channel
		// is pinned to the uplink's, so a roam moves the hotspot and drops
		// every joined device, and the fallback is unavailable because taking
		// that interface over would end the connection being shared. A radio
		// whose link is something else has neither problem.
		//
		// Without this the choice fell to whichever radio "iw list" happened
		// to print first, which is not a decision.
		best := shared[0]
		for _, c := range shared {
			bestIsUplink := best.station.Name == p.Uplink
			cIsUplink := c.station.Name == p.Uplink
			if bestIsUplink && !cIsUplink {
				best = c
				continue
			}
			if cIsUplink && !bestIsUplink {
				continue
			}
			if c.bus == "usb" && best.bus != "usb" {
				best = c
			}
		}
		// The last resort is only a resort when nothing better was BLOCKED.
		//
		// If the best remaining choice is the radio carrying the internet
		// connection, and another adapter could have hosted the hotspot but
		// for a capability this package does not have, then falling through
		// to the uplink's own radio silently trades a missing feature for the
		// worst arrangement there is: the hotspot pinned to the uplink's
		// channel, roaming with it, with no fallback because taking that
		// interface over would end the connection being shared. Refusing and
		// naming the adapter is the honest answer, and it is one the user can
		// act on.
		if best.station.Name == p.Uplink {
			for _, c := range blocked {
				if c.station.Name != p.Uplink {
					return blockedRefusal(c)
				}
			}
		}
		return p.acceptHotspot(f, best, o)
	}

	// No radio can hold an access point beside the link it carries. If one of
	// them is blocked only by the missing release, say which and why.
	for _, c := range blocked {
		if c.station.Name != p.Uplink {
			return blockedRefusal(c)
		}
	}

	// Every AP-capable radio already carries a station link and none of them
	// permits an access point beside it.
	carriesUplink := false
	for _, c := range cands {
		if c.station.Name == p.Uplink {
			carriesUplink = true
		}
	}
	if carriesUplink {
		return &PlanError{
			Err: fmt.Errorf("%w: no combination on any AP-capable radio allows managed and AP together", ErrAPConflictsWithUplink),
			User: "The only WiFi adapter on this machine is being used for the internet connection, " +
				"and it cannot run a hotspot at the same time. Plug in a USB WiFi adapter, or connect the internet with a cable.",
		}
	}
	return &PlanError{
		Err: fmt.Errorf("%w: every AP-capable radio is already connected to a network and cannot do both", ErrAPConflictsWithUplink),
		User: "The WiFi adapter that could create a hotspot is already connected to a network and cannot do both. " +
			"Disconnect it in advanced settings, or plug in a USB WiFi adapter.",
	}
}

// foreignPrefixes lists the addresses on an interface that belong to some
// other network, so they can be removed before anything serves on it.
//
// Link-local and loopback are left: the kernel regenerates the first and the
// second reaches nobody. An address inside the hotspot subnet is this
// appliance's own and stays.
func foreignPrefixes(f Facts, name string, hotspot netip.Prefix) []netip.Prefix {
	l, ok := f.LinkByName(name)
	if !ok {
		return nil
	}
	var out []netip.Prefix
	for _, pfx := range l.Prefixes {
		if pfx.Addr().IsLinkLocalUnicast() || pfx.Addr().IsLoopback() {
			continue
		}
		if hotspot.IsValid() && hotspot.Contains(pfx.Addr()) {
			continue
		}
		out = append(out, pfx)
	}
	return out
}

// chooseChannel picks the channel the access point will run on, and refuses
// rather than substituting when the user asked for a band this radio cannot
// give them.
//
// With no band asked for it prefers 2.4GHz. That is a REACH decision, not a
// speed one, and it is written down here because it used to be a side effect
// of sorting channel numbers ascending, which happens to put 2.4GHz first.
// The product exists to give somebody an internet connection: a 5GHz hotspot
// is faster and is invisible to devices that only scan 2.4GHz. MEASURED on
// 2026-08-30: a hotspot on channel 36 did not appear in the test handset's
// scan at all, whose results covered 2412 to 2462 MHz only, while the panel
// correctly reported it up and broadcasting.
func (p *Plan) chooseChannel(phy Phy, band RadioBand) error {
	if band == BandAuto {
		if in := phy.UsableChannelsIn(Band2GHz); len(in) > 0 {
			p.Channel = in[0]
			return nil
		}
		if len(p.UsableChannel) > 0 {
			p.Channel = p.UsableChannel[0]
		}
		return nil
	}
	in := phy.UsableChannelsIn(band)
	if len(in) == 0 {
		return &PlanError{
			Err: fmt.Errorf("%w: %s reports no usable channel in %s", ErrBandUnavailable, phy.Name, band),
			User: "This WiFi adapter cannot run a hotspot on " + string(band) + ". Choose the other band, " +
				"or plug in an adapter that supports this one.",
		}
	}
	p.Channel = in[0]
	return nil
}

// refuseBandAgainstPin is the other half of the same promise. A radio limited
// to one channel puts the access point on the channel its station link is
// already using, and that channel may be in the band the user did not ask for.
// Substituting either one is what produced hw_mode=g with channel 36.
func (p *Plan) refuseBandAgainstPin(phy Phy, station WirelessIface, band RadioBand) error {
	if band == BandAuto {
		return nil
	}
	pinned := phy.BandOfChannel(p.Channel)
	if pinned == BandAuto && station.FreqMHz > 0 {
		pinned = BandOf(station.FreqMHz)
	}
	if pinned == band {
		return nil
	}
	network := station.Name
	if station.SSID != "" {
		network = fmt.Sprintf("%q on %s", station.SSID, station.Name)
	}
	return &PlanError{
		Err: fmt.Errorf("%w: %s is pinned to channel %d, which is %s, because %s is joined to a network there",
			ErrBandUnavailable, phy.Name, p.Channel, pinned, station.Name),
		User: "This WiFi adapter can only be on one channel at a time, and it is already on " + string(pinned) +
			" for the connection to " + network + ", so the hotspot cannot run on " + string(band) +
			". Choose that band instead, disconnect that network, or plug in a USB WiFi adapter.",
	}
}

// takeOver plans the access point onto the interface a radio already has, by
// ending the station link on it first.
//
// It is the same arrangement Plan.HotspotTakeover produces as a runtime
// fallback, chosen at PLAN time for a radio that has told us it cannot hold
// both at once. The steps come out of HotspotReleaseSteps and HotspotAddrSteps
// and are exactly the sequence proved by hand on the target:
//
//	nmcli device set <if> managed no
//	ip address del <station address> dev <if>
//	ip link set dev <if> down
//	iw dev <if> set type __ap
//	ip link set dev <if> up
//	ip address add <hotspot address> dev <if>
//
// The cost is real and is recorded in Notes rather than hidden: the connection
// that interface holds ends. It is never the uplink; the caller checks that
// before calling, and TestPlan_TheHotspotIsNeverTheUplink checks the caller.
func (p *Plan) takeOver(f Facts, c apCandidate, o Options) error {
	w := c.station
	if !ValidInterfaceName(w.Name) {
		return fmt.Errorf("netcfg: hotspot interface name %q is not usable", w.Name)
	}
	p.Hotspot = w.Name
	p.HotspotIsVirtual = false
	p.HotspotTakenOver = true
	p.HotspotParent = ""
	p.HotspotFallback = ""
	// MEASURED for this exact interface. It is the user's own adapter and the
	// release has to be undone on teardown, or their box never rejoins the
	// network it was on.
	p.HotspotManager = w.Manager

	// Not pinned. "#channels <= 1" constrains interfaces that COEXIST on one
	// radio; with the station link ended the access point is the only
	// interface on it and may use any channel the radio allows.
	p.ChannelPinned = false
	if err := p.chooseChannel(c.phy, o.HotspotBand); err != nil {
		return err
	}

	what := w.Name
	if w.SSID != "" {
		what = fmt.Sprintf("%q on %s", w.SSID, w.Name)
	}
	p.Notes = append(p.Notes, fmt.Sprintf(
		"this WiFi adapter (%s) cannot run a hotspot and stay connected to a network at the same time, "+
			"so the connection to %s ends while the hotspot is on; it is restored when the hotspot is switched off",
		c.phy.Name, what))
	return nil
}

// preferUSB picks the USB adapter when the choice is otherwise equal, which is
// what mode B describes: the built-in radio keeps the box's own connection and
// the adapter the user plugged in runs the hotspot.
func preferUSB(cands []apCandidate) apCandidate {
	// An empty slice returns the zero candidate rather than panicking. Every
	// call site guards the length today; a panic here would abort the whole
	// test binary, which this package has been bitten by twice, and in
	// production it would take the appliance down instead of refusing.
	if len(cands) == 0 {
		return apCandidate{}
	}
	best := cands[0]
	for _, c := range cands {
		if c.bus == "usb" && best.bus != "usb" {
			best = c
		}
	}
	return best
}

// blockedRefusal is the one refusal for a radio that declares no combination
// of managed and AP while carrying a link this package cannot end.
//
// The only way to use such a radio is to release its station first, which is
// the takeover-first path and does not exist yet. Until it does, this is a
// refusal BEFORE anything is applied rather than a note followed by a start
// that fails two steps in.
func blockedRefusal(c apCandidate) error {
	// ErrHotspotNotReleased, not ErrAPConflictsWithUplink. The adapter is not
	// the uplink and there may be no uplink involved at all: it is an adapter
	// that CAN host a hotspot and is busy holding a network. That is what this
	// error says, and internal/privsvc already maps it to the panel word for
	// exactly that situation, so the refusal reaches the user as "the adapter
	// is busy" rather than "no adapter can do this" or "restart the machine".
	return &PlanError{
		Err: fmt.Errorf("%w: %s declares no combination of managed and AP, and %s is joined to a network "+
			"that this package cannot yet end first", ErrHotspotNotReleased, c.phy.Name, c.station.Name),
		User: "This WiFi adapter cannot run a hotspot while it is connected to a WiFi network, and Caspian " +
			"cannot disconnect it for you yet. Use the other WiFi adapter for the hotspot, or disconnect " +
			"this one from its network first.",
	}
}

// apCandidate is one radio that could run the access point, together with the
// interface already on it and the station link, if any, that must be preserved.
type apCandidate struct {
	phy     Phy
	iface   WirelessIface // an existing interface on the radio, if there is one
	station WirelessIface // an interface already operating on a channel
	bus     string
}

// apCandidates lists the radios that report AP support. It is driven by the
// radio list rather than the interface list so that a radio present with no
// interface on it is still a candidate: an access point can be created on it.
//
// uplink is the interface carrying the internet connection. It counts as a
// link that must not be disturbed whatever type it reports, which is stricter
// than StationLink alone: an interface can be carrying the default route while
// reporting a type that is not a station, and taking that one over would cut
// the box off from the internet it exists to share. MEASURED: planning against
// a two-radio capture where the built-in radio was type AP and carried the
// default route produced hotspot == uplink == wlan0, with nothing to stop it.
func apCandidates(f Facts, uplink string) []apCandidate {
	var out []apCandidate
	for _, phy := range f.Phys {
		if !phy.SupportsAP() {
			continue
		}
		c := apCandidate{phy: phy}
		for _, w := range f.Wireless {
			if w.Phy != phy.Name {
				continue
			}
			if c.iface.Name == "" {
				c.iface = w
				if l, ok := f.LinkByName(w.Name); ok {
					c.bus = l.Bus
				}
			}
			// The link an access point must not disturb, and whose channel
			// pins it on a radio limited to one channel.
			//
			// StationLink, not "reports a channel". An interface already in
			// AP mode reports the channel its station link was using before
			// the release, and that number is stale: reading it here would
			// pin a new hotspot to the old network's channel on any box where
			// a previous run left the interface typed.
			//
			// The uplink counts whatever its type says, and it wins over any
			// other link on the same radio, because losing it is the one
			// outcome this package must never plan.
			if w.Name == uplink {
				c.station = w
				continue
			}
			if w.StationLink() && c.station.Name == "" {
				c.station = w
			}
		}
		out = append(out, c)
	}
	return out
}

// acceptHotspot records the chosen radio, decides whether the access point
// needs an interface of its own, and works out the channel.
func (p *Plan) acceptHotspot(f Facts, c apCandidate, o Options) error {
	p.HotspotPhy = c.phy.Name
	p.UsableChannel = c.phy.UsableChannels()

	switch {
	case c.station.Name != "" && !c.phy.DeclaresAPWithStation() && c.station.Name != p.Uplink:
		// TAKEOVER-FIRST. The radio can be an access point and cannot be one
		// beside a station, so the station goes first and the access point
		// runs on the interface that is already there.
		//
		// MEASURED on the target on 2026-08-30, on the TP-Link TL-WN823N
		// (RTL8192EU, rtl8xxxu, firmware rtl8192eu_nic.bin rev 35.7):
		//
		//	supported interface modes:      managed, AP, AP/VLAN, monitor
		//	valid interface combinations:   none declared
		//	iw phy <dongle> interface add captest type __ap   rc=0, CREATED
		//	ip link set dev captest up                        refused
		//
		// The declaration and the refusal agree: it can be an access point,
		// and not while it is also a station. Adding a second interface is
		// therefore the wrong shape for this radio, and it fails in the worst
		// possible place, two steps after the create, on a step no fallback
		// watches.
		//
		// What is NOT yet measured is whether this radio BEACONS once it is
		// an access point. Nothing here can establish that, and nothing here
		// pretends to: AssertHotspotIsAccessPoint is the readback that has to
		// catch a radio which accepts AP mode and serves nothing, and it is
		// the only thing between that state and a panel claiming a working
		// hotspot.
		if err := p.takeOver(f, c, o); err != nil {
			return err
		}

	case c.station.Name != "":
		// A station link is already up on this radio and the radio declares it
		// can hold an access point beside it. Add an interface rather than
		// take that one over: the connection on it is somebody's, and keeping
		// it is better than ending it.
		//
		// This is the DEFAULT ARRANGEMENT's branch on a box whose built-in
		// radio is joined to a network, and it is deliberately untouched by
		// the takeover-first change above.
		name := o.APIfaceName
		if name == "" {
			name = defaultAPIfaceName
		}
		if !ValidInterfaceName(name) {
			return fmt.Errorf("netcfg: access point interface name %q is not a valid interface name", name)
		}
		p.Hotspot = name
		p.HotspotIsVirtual = true
		p.HotspotParent = c.station.Name
		// The interface this names does not exist yet, so nothing has measured
		// who manages it and this stays unknown. See the note on HotspotManager.
		p.HotspotManager = ManagedByUnknown

		// The fallback, decided now and used only if creation fails. Never
		// the uplink: an access point on the interface carrying the internet
		// connection ends that connection, and a box that cannot reach the
		// internet has nothing to share.
		if c.station.Name != p.Uplink {
			p.HotspotFallback = c.station.Name
		}

		ok, combo := c.phy.APWithStation()
		if !ok {
			// Only the uplink can reach here: every other no-combination
			// radio took the takeover branch above.
			// The link on this radio is the internet connection, so it has to
			// survive, so the access point has to coexist with it, so the
			// radio must declare that it can. Without the declaration there
			// is nothing else to try: taking the interface over would end the
			// connection the box is sharing.
			return &PlanError{
				Err: fmt.Errorf("%w: %s permits no combination of managed and AP", ErrAPConflictsWithUplink, c.phy.Name),
				User: "The WiFi adapter that could create a hotspot is the one being used for the internet connection, " +
					"and it cannot do both. Connect the internet with a cable, or plug in a USB WiFi adapter.",
			}
		}
		// The pin, and the sentence that goes with it, are allowed ONLY when
		// there is a connection to match.
		//
		// c.station.StationLink() is asked again here rather than assumed
		// from c.station being set, because the uplink is put there whatever
		// its state: it is the link that must not be disturbed, and that is a
		// different question from whether it is joined to anything.
		//
		// The note used to be emitted from the same branch as the pin with no
		// such condition, and on 2026-08-30 it told the operator the hotspot
		// was being pinned to "the channel an existing WiFi connection is
		// using on wlan0" while wlan0 was joined to nothing. A confident
		// sentence asserting a connection that does not exist is worse than
		// no sentence: it explains away the wrong channel and stops anyone
		// looking for the real cause.
		switch {
		case combo.Channels == 1 && c.station.StationLink():
			p.Channel = c.station.Channel
			p.ChannelPinned = true
			if err := p.refuseBandAgainstPin(c.phy, c.station, o.HotspotBand); err != nil {
				return err
			}
			what := "the WiFi connection"
			if c.station.Name == p.Uplink {
				what = "the internet connection"
			}
			if c.station.SSID != "" {
				what += fmt.Sprintf(" to %q", c.station.SSID)
			}
			p.Notes = append(p.Notes, fmt.Sprintf(
				"%s reports #channels <= 1, so the hotspot is pinned to channel %d, the channel %s on %s is using; "+
					"if that connection roams to another channel the hotspot follows it and every joined device is dropped while it does",
				c.phy.Name, c.station.Channel, what, c.station.Name))
		default:
			if err := p.chooseChannel(c.phy, o.HotspotBand); err != nil {
				return err
			}
		}

	case c.iface.Name != "":
		// The radio has an interface and nothing is using it.
		if !ValidInterfaceName(c.iface.Name) {
			return fmt.Errorf("netcfg: hotspot interface name %q is not usable", c.iface.Name)
		}
		p.Hotspot = c.iface.Name
		// MEASURED for this exact interface, and carried rather than dropped.
		// "Nothing is using it" is not "nothing manages it": NetworkManager can
		// hold a disconnected adapter and connect it to a remembered network at
		// any moment, including while hostapd is beaconing on it. This is the
		// value HotspotReleaseSteps acts on.
		p.HotspotManager = c.iface.Manager

		if err := p.chooseChannel(c.phy, o.HotspotBand); err != nil {
			return err
		}

	default:
		// The radio exists with no interface on it, so one is created.
		name := o.APIfaceName
		if name == "" {
			name = defaultAPIfaceName
		}
		if !ValidInterfaceName(name) {
			return fmt.Errorf("netcfg: access point interface name %q is not a valid interface name", name)
		}
		p.Hotspot = name
		p.HotspotIsVirtual = true
		// Created by this package, so unmeasurable for the same reason as above.
		p.HotspotManager = ManagedByUnknown
		if err := p.chooseChannel(c.phy, o.HotspotBand); err != nil {
			return err
		}
	}

	if p.Channel == 0 && len(p.UsableChannel) == 0 {
		p.Notes = append(p.Notes, fmt.Sprintf(
			"%s reported no usable channel, so the access point will have to be given one by hand", c.phy.Name))
	}
	return nil
}

// canPin reports whether a host route to this server address can be written.
// It needs a gateway, or a device on a point-to-point link, in the address's
// own family.
func (p *Plan) canPin(s netip.Addr) bool {
	if !s.Is6() {
		return p.Uplink != ""
	}
	return p.UplinkV6Gw.IsValid() || p.UplinkV6Dev != ""
}

// ErrNoTakeoverCandidate is returned when creating a second interface failed
// and the only interface that could host the access point is the uplink.
var ErrNoTakeoverCandidate = errors.New("netcfg: the only interface that could host the access point is carrying the uplink")

// ErrInterfaceOwnerUnknown is returned when the hotspot interface cannot be
// shown to be free. Not knowing what holds an interface is the state that put
// a DHCP server on somebody's home network, so it is a refusal and not a
// warning.
var ErrInterfaceOwnerUnknown = errors.New("netcfg: cannot determine what owns the interface the hotspot would take over")

// HotspotTakeover returns the fallback plan: the access point takes over an
// existing wireless interface instead of getting one of its own.
//
// It exists because a radio's interface-combination table is a statement about
// what the hardware could do in principle, not proof that creating the
// interface succeeds. MEASURED on the target on 2026-08-30: phy0 advertises
// "#{ managed } <= 1, #{ AP } <= 1, total <= 4" and
// "iw phy phy0 interface add ap0 type __ap" fails with "Input/output error
// (-5)" while wlan0 is associated. The planner reads the table correctly and
// the driver refuses anyway, so the only way to tell is to try.
//
// The cost is real and is recorded in Notes rather than hidden: taking over an
// interface ends whatever WiFi connection it currently holds. On a box whose
// uplink is wired that is a second route to the same network and losing it is
// harmless. On a box whose only uplink is WiFi it would be fatal, which is
// exactly why this refuses rather than taking the uplink.
func (p *Plan) HotspotTakeover(f Facts) (*Plan, error) {
	if !p.HotspotIsVirtual {
		return nil, fmt.Errorf("netcfg: the hotspot on %s already uses an existing interface, so there is no fallback to take", p.Hotspot)
	}
	if p.HotspotFallback == "" {
		return nil, &PlanError{
			Err: fmt.Errorf("%w: %s", ErrNoTakeoverCandidate, p.Uplink),
			User: "This machine's WiFi adapter cannot run a hotspot alongside the connection it is using for the internet, " +
				"and taking it over would cut off the internet this box is sharing. Connect the internet with a cable, " +
				"or plug in a USB WiFi adapter.",
		}
	}

	// Whether the interface can be taken is a question about the machine, so
	// it is asked of freshly detected facts rather than of the plan that was
	// made before any of this was attempted.
	w, isWireless := f.WirelessByName(p.HotspotFallback)
	if !isWireless {
		return nil, &PlanError{
			Err:  fmt.Errorf("%w: %s is not in the wireless interface list", ErrInterfaceOwnerUnknown, p.HotspotFallback),
			User: "The WiFi adapter that would run the hotspot could not be found. Try switching off and on again.",
		}
	}
	// A DOWN interface is not in use, whatever channel it still reports.
	//
	// MEASURED on the target: wlan0 down, no address, "iw dev" reporting type
	// managed with NO ssid and channel 56. That is the exact shape that broke
	// the predicate this package already split once: a channel with no network
	// name. The channel is left over from an earlier association and means
	// nothing once the link is down, and reading it as "somebody is using
	// this" refuses to take an interface that is plainly free.
	inUse := w.InUse()
	if l, ok := f.LinkByName(w.Name); ok && strings.EqualFold(l.State, "DOWN") {
		inUse = false
	}

	switch {
	case w.Manager == ManagedByNetworkManager:
		// Releasable, and the release is journalled.
	case w.Manager == ManagedByNothing && !inUse:
		// Nothing holds it.
	case w.Manager == ManagedByNothing && inUse:
		return nil, &PlanError{
			Err: fmt.Errorf("%w: %s is joined to %q but no manager reports owning it",
				ErrInterfaceOwnerUnknown, w.Name, w.SSID),
			User: "The WiFi adapter needed for the hotspot is connected to a network and this box cannot work out " +
				"what is keeping it there, so it will not take it over. Disconnect that WiFi network first.",
		}
	default:
		return nil, &PlanError{
			Err: fmt.Errorf("%w: %s reports manager %q", ErrInterfaceOwnerUnknown, w.Name, w.Manager),
			User: "This box cannot work out what is managing the WiFi adapter needed for the hotspot, so it will not " +
				"take it over. Setting up a hotspot on an adapter still joined to another network can disrupt that network.",
		}
	}

	q := *p
	q.Hotspot = p.HotspotFallback
	q.HotspotIsVirtual = false
	q.HotspotParent = ""
	q.HotspotTakenOver = true
	q.HotspotManager = w.Manager

	// The addresses the interface already carries, which must come off before
	// anything binds to it. Link-local is left alone: the kernel regenerates
	// it and it reaches nobody.
	q.HotspotStationPrefixes = nil
	if l, ok := f.LinkByName(w.Name); ok {
		for _, pfx := range l.Prefixes {
			if pfx.Addr().IsLinkLocalUnicast() || pfx.Addr().IsLoopback() {
				continue
			}
			q.HotspotStationPrefixes = append(q.HotspotStationPrefixes, pfx)
		}
	}

	// The channel is no longer pinned. "#channels <= 1" constrains interfaces
	// that coexist on one radio; with the station link gone the access point
	// is the only interface on it and may use any channel the radio allows.
	q.ChannelPinned = false
	q.Channel = 0
	if len(p.UsableChannel) > 0 {
		q.Channel = p.UsableChannel[0]
	}

	// Copy the notes rather than appending to the original's backing array:
	// the caller still holds the first plan and may need to tear it down.
	q.Notes = make([]string, 0, len(p.Notes)+1)
	for _, n := range p.Notes {
		// The pinned-channel note described the plan that failed.
		if strings.Contains(n, "#channels <= 1") {
			continue
		}
		q.Notes = append(q.Notes, n)
	}
	q.Notes = append(q.Notes, fmt.Sprintf(
		"the radio refused to create a second interface, so the hotspot is planned on %s; "+
			"%s must first be released by whatever manages it and stripped of its own addresses, "+
			"and nothing may bind to it until that has been read back from the kernel; "+
			"the internet connection on %s is not touched",
		q.Hotspot, q.Hotspot, q.Uplink))
	return &q, nil
}

// Explain is the one line the panel shows in basic mode (design 5.2).
//
// It says "wired" rather than "Ethernet" and never says "built-in", because
// neither is something this package measured: what it read was that the
// interface carrying the default route is not in the wireless list, and which
// radio the access point landed on.
// backend is the Backend for this plan's platform.
func (p *Plan) backend() Backend { return BackendFor(p.Platform) }

func (p *Plan) Explain() string {
	var b strings.Builder
	if p.Mode == ModeWiredUplink {
		fmt.Fprintf(&b, "Internet: wired connection on %s.", p.Uplink)
	} else {
		fmt.Fprintf(&b, "Internet: WiFi on %s.", p.Uplink)
	}
	switch {
	case p.HotspotTakenOver:
		fmt.Fprintf(&b, " Hotspot: WiFi on %s (%s)", p.Hotspot, p.HotspotPhy)
	case p.HotspotIsVirtual && p.HotspotParent != "":
		fmt.Fprintf(&b, " Hotspot: WiFi on %s, a second connection on the same radio as %s (%s)", p.Hotspot, p.HotspotParent, p.HotspotPhy)
	default:
		fmt.Fprintf(&b, " Hotspot: WiFi on %s (%s)", p.Hotspot, p.HotspotPhy)
	}
	if p.Channel > 0 {
		switch {
		case p.ChannelPinned && p.HotspotParent == p.Uplink:
			// Saying "the internet connection" is only true when the link
			// that fixes the channel is the uplink. In mode A the radio is
			// often associated to something that is not the uplink at all,
			// and naming it wrongly sends the user to the wrong setting.
			fmt.Fprintf(&b, ", channel %d, fixed by the radio to match the internet connection", p.Channel)
		case p.ChannelPinned:
			fmt.Fprintf(&b, ", channel %d, fixed by the radio to match the WiFi connection already on %s", p.Channel, p.HotspotParent)
		default:
			fmt.Fprintf(&b, ", channel %d", p.Channel)
		}
	}
	b.WriteString(".")
	// The cost of the fallback belongs in the sentence a person reads, not
	// only in the notes an operator might open. Starting the hotspot on an
	// interface that already had a connection ends that connection.
	if p.HotspotTakenOver {
		// This says what will be done, not what has happened. The previous
		// wording asserted the connection was already ended, and it was read
		// as a description of the outcome by everyone who saw it, including
		// in a journal an hour after the interface had in fact stayed on the
		// house network with our DHCP server answering on it.
		fmt.Fprintf(&b, " To do that, %s has to be disconnected from the WiFi network it is on now.", p.Hotspot)
	}
	// The firewall posture belongs in the sentence a person reads. The
	// previous posture closed inbound access to the machine and said so
	// nowhere, which on a headless box meant the owner discovered it by
	// losing the ability to reach their own Pi.
	fmt.Fprintf(&b, " Devices on the hotspot can reach only the services this box offers them;"+
		" how you reach this machine from %s is not changed.", p.Uplink)
	if p.Opts.Egress == EgressRestricted {
		// The person who hits this will be reading this sentence or the
		// ruleset header, so it says what breaks rather than describing a
		// policy.
		b.WriteString(" While the hotspot is on, programs on this box cannot reach the internet" +
			" directly, so updating its software will not work until you switch it off.")
	}
	return b.String()
}

// UplinkState is the part of the plan that a DHCP renewal, an unplugged cable
// or a move to another network can invalidate.
func (p *Plan) UplinkState() UplinkState {
	return UplinkState{
		Interface: p.Uplink,
		Gateway:   p.UplinkGateway,
		OnLink:    p.UplinkOnLink,
	}
}
