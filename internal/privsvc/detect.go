// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"strings"
	"time"

	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// detectionFrom turns what internal/netcfg measured into the vocabulary the
// panel shows a person. plan may be nil, which is the machine on which no
// workable arrangement exists; the interface list is still filled in, because
// "no adapter here can create a hotspot" is more useful next to the list of
// adapters that are here.
func detectionFrom(f netcfg.Facts, plan *netcfg.Plan, country string, fault panel.Fault, now time.Time) panel.Detection {
	d := panel.Detection{
		Country: country,
		Fault:   fault,
		At:      now,
	}

	seen := map[string]bool{}
	add := func(i panel.InterfaceInfo) {
		if i.Name == "" || seen[i.Name] {
			return
		}
		seen[i.Name] = true
		d.Interfaces = append(d.Interfaces, i)
	}

	def, haveDefault := f.PrimaryDefault()
	for _, l := range f.Links {
		if l.IsLoopback() {
			continue
		}
		info := panel.InterfaceInfo{Name: l.Name, Kind: panel.KindEthernet}
		if w, ok := f.WirelessByName(l.Name); ok {
			info.Kind = wifiKind(l.Bus)
			if phy, ok := f.PhyByName(w.Phy); ok {
				info.CanHostAP = phy.SupportsAP()
			}
		}
		info.HasDefaultRoute = haveDefault && def.Dev == l.Name
		add(info)
	}

	if haveDefault {
		d.InternetInterface = def.Dev
		d.LocalNetworkAddress = firstIPv4(f, def.Dev)
	}
	if plan == nil {
		return d
	}

	d.InternetInterface = plan.Uplink
	d.LocalNetworkAddress = firstIPv4(f, plan.Uplink)
	d.HotspotInterface = plan.Hotspot
	d.Channel = plan.Channel
	d.Band = string(bandForChannel(plan.Channel))
	d.UsableChannels = plan.UsableChannel
	d.ChannelPinned = plan.ChannelPinned
	if plan.HotspotSubnet.IsValid() {
		d.Subnet = plan.HotspotSubnet.String()
	}

	// The access point's own interface does not exist until it is created, so
	// it is not in the link list, and the panel still has to be able to name
	// it: panel.DetectedLine looks the current hotspot interface up by name
	// and falls back to printing the kernel name when it finds nothing.
	//
	// It is added with CanHostAP set, which makes it one of the choices
	// panel.Detection.APCandidates offers. validateAgainstFacts accepts that
	// name back and reads it as "no override", because internal/netcfg's
	// HotspotOverride matches an existing interface or a radio and would refuse
	// a name for an interface that does not exist yet.
	if plan.HotspotIsVirtual {
		kind := panel.KindWiFi
		if parent, ok := f.LinkByName(plan.HotspotParent); ok {
			kind = wifiKind(parent.Bus)
		}
		add(panel.InterfaceInfo{Name: plan.Hotspot, Kind: kind, CanHostAP: true})
	}

	// The hotspot address is REPORTED, not intended: it is whatever address is
	// actually on the hotspot interface right now. internal/panel/priv.go says
	// it is empty until the access point has been brought up, and reading it
	// back from the interface is what makes that true by measurement rather
	// than by this package remembering to blank it.
	d.HotspotAddress = firstIPv4(f, plan.Hotspot)
	return d
}

// firstIPv4 returns the first IPv4 address on the named interface, or "".
func firstIPv4(f netcfg.Facts, name string) string {
	if name == "" {
		return ""
	}
	l, ok := f.LinkByName(name)
	if !ok {
		return ""
	}
	for _, p := range l.Prefixes {
		a := p.Addr().Unmap()
		if a.Is4() && !a.IsLoopback() && !a.IsLinkLocalUnicast() {
			return a.String()
		}
	}
	return ""
}

// wifiKind reads the parent bus iproute2 reported.
//
// An empty string means "not reported" and never "not USB": internal/netcfg's
// Link.Bus comment records that only some kernels and iproute2 versions expose
// it. So the fallback is the generic word rather than a guess at which radio
// this is.
func wifiKind(bus string) panel.InterfaceKind {
	switch bus {
	case "usb":
		return panel.KindUSBWiFi
	case "":
		return panel.KindWiFi
	default:
		return panel.KindBuiltinWiFi
	}
}

// bandForChannel maps a channel onto the band hostapd needs.
//
// internal/netcfg plans a channel and has no concept of a band;
// internal/hotspot needs one to choose hw_mode. Channel 14 is the last 2.4 GHz
// channel (Japan only, 802.11b only, and internal/hotspot refuses it), and
// everything above it is 5 GHz, so the boundary is unambiguous.
func bandForChannel(ch int) hotspot.Band {
	if ch > 14 {
		return hotspot.Band5GHz
	}
	return hotspot.Band2GHz
}

// parseRegDomain reads the country out of "iw reg get".
//
// The output looks like:
//
//	global
//	country GB: DFS-ETSI
//		(2402 - 2482 @ 40), (N/A, 20), (N/A)
//
// and on a box where nothing has set a domain the country line reads
// "country 00: DFS-UNSET". A machine with several phys prints a block per phy;
// the first country line that is not the world domain is taken, because a
// hotspot that is legal under one domain and not another is a situation this
// program cannot resolve and should not pretend to.
func parseRegDomain(out string) (string, bool) {
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		rest, ok := strings.CutPrefix(line, "country ")
		if !ok {
			continue
		}
		cc, _, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		cc = strings.TrimSpace(cc)
		if len(cc) != 2 || cc == "00" {
			continue
		}
		if !isUpperAlpha2(cc) {
			continue
		}
		return cc, true
	}
	return "", false
}

func isUpperAlpha2(s string) bool {
	if len(s) != 2 {
		return false
	}
	for i := 0; i < 2; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}
