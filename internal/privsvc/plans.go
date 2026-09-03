// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"errors"
	"net/netip"
	"time"

	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// DHCP pool bounds, as offsets from the base of the hotspot subnet.
//
// The gateway is the first address in the subnet (netcfg.GatewayAddr), so the
// pool starts well above it: internal/hotspot refuses a pool containing the
// gateway, because handing the box's own address to a client makes two machines
// answer for the router and takes the hotspot down in a way that looks
// intermittent. The gap below the pool is where a static address can be set by
// hand without colliding with a lease.
const (
	dhcpFirstOffset = 50
	dhcpLastOffset  = 200

	// dhcpLeaseTime is long enough that a phone in a pocket keeps its address
	// across a nap and short enough that a device that has left frees it the
	// same day, which is what makes the panel's device count mean something.
	dhcpLeaseTime = 12 * time.Hour

	// dnsCacheSize is dnsmasq's answer cache. Every miss is a query that
	// crosses the tunnel, so a cache is latency the user feels; it is small
	// because the box serves a room, not a site.
	dnsCacheSize = 150
)

// netOptionsFor applies the request's overrides to this service's options.
//
// Every override is one internal/netcfg already accepts, and each is passed
// through rather than acted on here: the planner is what decides whether an
// override can be honoured, and it has the wording for the refusal.
func (s *Service) netOptionsFor(req panel.StartRequest) (netcfg.Options, error) {
	o := s.cfg.netOptions()
	o.UplinkOverride = req.Network.InternetInterface
	if h := req.Hotspot.Interface; h != "" && !s.isVirtualAPName(h) {
		o.HotspotOverride = h
	}
	// The band goes to the planner, because the planner is what picks the
	// channel and only it can tell whether the radio has one in that band.
	//
	// This used to be applied AFTER the plan, below, by overwriting the band
	// that the chosen channel implied. The two never had to agree, and when
	// they disagreed hostapd was handed a contradiction: a 2.4GHz channel
	// labelled 5GHz, which cannot work and surfaced as "the hotspot failed"
	// with nothing pointing at the band. The user's explicit choice was
	// silently replaced by a number picked for a different reason.
	//
	// The string values match hotspot.Band2GHz and hotspot.Band5GHz exactly,
	// so this is a conversion and not a mapping table that could drift.
	o.HotspotBand = netcfg.RadioBand(req.Hotspot.Band)
	if req.Hotspot.Subnet != "" {
		p, err := netip.ParsePrefix(req.Hotspot.Subnet)
		if err != nil {
			// validateStatic already refused this, so reaching it means the
			// two disagree, which is a bug in this package rather than a bad
			// request.
			return o, fail("hotspot subnet", panel.FaultUnknown, err)
		}
		o.HotspotSubnet = p
	}
	return o, nil
}

// engineDocument composes the complete configuration the engine is started
// with.
//
// What crossed the socket carries outbounds and nothing else. The inbound that
// client traffic arrives on, the loopback diagnostics inbound, the local DNS
// listener, the resolver policy and the routing rules are all decided HERE and
// never taken from anything the caller sent: internal/xcfg owns their shape and
// the order of the rules, which its own comments record as load bearing.
//
// The tunnel device name is the one value handed to both internal/netcfg and
// internal/xcfg. They describe the same device: netcfg writes an address on it
// and routes through it by name, and xcfg tells the engine to create it. A
// drift between them is a tunnel the routes do not name, which presents as a
// box that connects and carries nothing.
func (s *Service) engineDocument(l *link.Link, req panel.StartRequest, netOpts netcfg.Options) ([]byte, error) {
	o := xcfg.Defaults()
	o.Link = l
	o.TUN.Disabled = s.cfg.TUNDisabled
	o.TUN.Name = netOpts.TunName
	o.SOCKS.Port = s.cfg.SocksPort

	// The listener internal/hotspot's dnsmasq forwards to. Enabling it is what
	// makes client DNS resolvable at all: with it off, dnsmasq forwards to a
	// port nothing answers on and every joined device stops resolving while the
	// hotspot and the tunnel both look healthy. docs/LAYOUT.md calls this "the
	// pairing that breaks quietly", and hotspotPlanFor below is given the same
	// port from the same field.
	o.LocalDNS.Enabled = true
	o.LocalDNS.Port = s.cfg.LocalDNSPort

	if req.EngineLogLevel != "" {
		o.LogLevel = xcfg.LogLevel(req.EngineLogLevel)
	}

	doc, err := xcfg.Build(o)
	if err != nil {
		return nil, fail("compose engine configuration", panel.FaultEngineRejectedConfig, err)
	}
	return doc, nil
}

// hotspotPlanFor turns the network plan into a fully rendered access point.
//
// Nothing here is invented. The interface, the channel and the subnet come from
// the plan; the radio's limits are read back out of the facts the plan was made
// from, rather than assumed; the passphrase and the name come from the request.
func (s *Service) hotspotPlanFor(p *netcfg.Plan, f netcfg.Facts, req panel.StartRequest, country string) (hotspot.Plan, error) {
	channel := p.Channel
	if req.Hotspot.Channel != 0 && !p.ChannelPinned {
		// validateAgainstPlan has already refused a channel the radio will not
		// accept, and refused any override at all when the radio pins one.
		channel = req.Hotspot.Channel
	}
	// The band comes from the channel, unconditionally. netcfg has already
	// guaranteed the channel is in the requested band, or refused with a
	// reason naming the band and the radio. Overriding it here is what created
	// the contradiction this comment used to describe.
	band := bandForChannel(channel)
	if country == "" {
		// hostapd with no country_code falls back to the world regulatory
		// domain, where most channels are passive-scan only and beaconing is
		// not permitted, so the access point starts and no network ever
		// appears. internal/hotspot refuses it for that reason and this
		// refusal happens earlier, before anything has been applied.
		//
		// There is no Fault in the panel's closed vocabulary for "no
		// regulatory domain", so this is reported as unclassified rather than
		// forced into the nearest word. See the report accompanying this
		// package.
		return hotspot.Plan{}, fail("country", panel.FaultUnknown,
			errors.New("no country is set and the radio did not report one, so the hotspot cannot legally pick a channel"))
	}

	ap := hotspot.APConfig{
		Interface:   p.Hotspot,
		Uplink:      p.Uplink,
		SSID:        req.Hotspot.SSID,
		Passphrase:  req.Hotspot.Passphrase,
		CountryCode: country,
		Channel:     channel,
		Band:        band,
		ControlDir:  s.cfg.HotspotPaths.HostapdControlDir,
	}

	rc := radioConstraintFor(f, p, channel)

	first, err := nthAddress(p.HotspotSubnet, dhcpFirstOffset)
	if err != nil {
		return hotspot.Plan{}, fail("dhcp range", panel.FaultUnknown, err)
	}
	last, err := nthAddress(p.HotspotSubnet, dhcpLastOffset)
	if err != nil {
		return hotspot.Plan{}, fail("dhcp range", panel.FaultUnknown, err)
	}

	dns := hotspot.DNSConfig{
		Interface:  p.Hotspot,
		Subnet:     p.HotspotSubnet,
		Gateway:    p.HotspotGateway,
		RangeStart: first,
		RangeEnd:   last,
		LeaseTime:  dhcpLeaseTime,
		LeaseFile:  s.cfg.HotspotPaths.LeaseFile,
		// The same port engineDocument gave the engine's listener. One field,
		// two consumers, so the pairing cannot drift.
		Upstream:  netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), s.cfg.LocalDNSPort),
		CacheSize: dnsCacheSize,
	}

	plan, err := hotspot.NewPlan(ap, dns, rc)
	if err != nil {
		return hotspot.Plan{}, fail("hotspot configuration", hotspotFault(unitAP, err.Error(), err), err)
	}
	return plan, nil
}

// radioConstraintFor reads the radio's limits back out of the facts.
//
// internal/hotspot never asks the radio anything: it is given the answer and
// its job is to refuse a configuration the radio cannot run, before hostapd is
// started and fails in a way the user cannot read. This is where that answer
// comes from.
func radioConstraintFor(f netcfg.Facts, p *netcfg.Plan, channel int) hotspot.RadioConstraint {
	phy, ok := f.PhyByName(p.HotspotPhy)
	if !ok {
		// The planner chose this radio out of this same Facts value, so a
		// miss here means the two disagree. Reporting no AP support makes
		// internal/hotspot refuse with the sentence for the user rather than
		// letting an empty constraint pass everything.
		return hotspot.RadioConstraint{}
	}
	_, combo := phy.APWithStation()
	maxAPs := 0
	for _, lim := range combo.Limits {
		if lim.Has("AP") && lim.Max > maxAPs {
			maxAPs = lim.Max
		}
	}
	if maxAPs == 0 && phy.SupportsAP() {
		// The radio lists AP among its modes but no combination names it,
		// which is the shape of a radio that can be an access point on its own
		// and not beside anything.
		maxAPs = 1
	}
	rc := hotspot.RadioConstraint{
		SupportsAP:      phy.SupportsAP(),
		MaxAPs:          maxAPs,
		MaxChannels:     combo.Channels,
		AllowedChannels: phy.UsableChannels(),
	}
	if p.ChannelPinned {
		rc.ClientChannel = channel
	}
	return rc
}

// nthAddress returns the nth host address inside a prefix.
func nthAddress(p netip.Prefix, n int) (netip.Addr, error) {
	if !p.IsValid() {
		return netip.Addr{}, errors.New("privsvc: the hotspot subnet is not a network")
	}
	a := p.Masked().Addr()
	for i := 0; i < n; i++ {
		a = a.Next()
		if !a.IsValid() || !p.Contains(a) {
			return netip.Addr{}, errors.New("privsvc: the hotspot subnet is too small for a DHCP pool")
		}
	}
	return a, nil
}
