// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/netcfg"
)

// Engine is the part of internal/engine this package uses.
//
// It is an interface for one reason that is not testing convenience: the real
// engine's TUN inbound opens /dev/net/tun, which needs Linux and root, so a
// test that drove the real engine could never observe the ordering around the
// moment the tunnel device appears. *engine.Engine satisfies this exactly; see
// TestRealEngineSatisfiesTheInterface.
type Engine interface {
	Start(ctx context.Context, configJSON []byte) error
	Stop() error
	State() engine.State
	Logs() []engine.LogEntry
	LogsDropped() uint64
}

// Resolver turns the server hostname in the user's config into addresses.
//
// It is needed before the tunnel exists, because the pinned host route to the
// server is part of the network plan and a plan with no server address is a
// plan that loops the engine through its own tunnel
// (internal/netcfg/route.go, ServerRouteSteps).
//
// The cost of that ordering is worth stating rather than hiding: this lookup
// leaves the box in the clear, out of the uplink, carrying the name of the
// user's proxy server. A config whose address is already an IP literal skips
// it entirely, which is the shape that discloses nothing.
type Resolver interface {
	Resolve(ctx context.Context, host string) ([]netip.Addr, error)
}

// Reachability answers the third of the design's three failure states
// (docs/2026-08-29-design.md section 8, step 11): the config loaded and the
// server did not answer.
//
// WHAT AN IMPLEMENTATION OF THIS CAN AND CANNOT PROVE. The default is a TCP
// dial to the server's address and port. A success proves a TCP connection was
// accepted at that address. It does NOT prove the proxy handshake succeeded, it
// does not prove the credentials are right, and it captures no exit IP, so it
// is not evidence that the tunnel carries traffic. Nothing in this package is.
type Reachability interface {
	Probe(ctx context.Context, addr netip.Addr, port uint16) error
}

// Config is everything the service needs. Nothing here is discovered from the
// environment: cmd/caspian reads docs/LAYOUT.md and passes the values in, so
// that no package hardcodes a value it does not own.
type Config struct {
	// Runner executes the network commands. Required.
	// Use netcfg.NewSystemRunner on the appliance and netcfg.RecordingRunner
	// in a test.
	Runner netcfg.Runner

	// System executes the hotspot's effects. Required.
	// Use hotspot.NewSystemRunner on the appliance and hotspot.Recorder in a
	// test.
	System hotspot.System

	// HotspotPaths is where hostapd and dnsmasq keep their files. The zero
	// value is refused rather than defaulted, because these paths decide which
	// process the supervisor is allowed to kill.
	HotspotPaths hotspot.Paths

	// JournalPath is the teardown journal. docs/LAYOUT.md fixes
	// /var/lib/caspian/netcfg.journal, which is netcfg.DefaultJournalPath.
	// Required.
	JournalPath string

	// TunName is the tunnel device. It is ONE value given to both
	// internal/netcfg and internal/xcfg, because the two describe the same
	// device and a drift between them is a tunnel the routes do not name.
	// Empty means netcfg.DefaultOptions().TunName.
	TunName string

	// SocksPort is the loopback diagnostics inbound, LAYOUT.md 10808.
	SocksPort uint16

	// LocalDNSPort is the engine's local DNS listener, LAYOUT.md 5354. It is
	// ONE value given to internal/xcfg, which listens on it, and to
	// internal/hotspot, whose dnsmasq forwards to it. docs/LAYOUT.md: "The
	// 5354 pairing is the one that breaks quietly."
	LocalDNSPort uint16

	// DNSPort is where dnsmasq answers on the hotspot interface, LAYOUT.md 53.
	DNSPort int

	// PanelPort is the panel's listener, LAYOUT.md 8088. The firewall opens it
	// on the hotspot interface only.
	PanelPort int

	// Country is the regulatory domain to fall back on when the request
	// carries no override and "iw reg get" reports none. Empty means there is
	// no fallback, and a start with no country anywhere is refused: hostapd
	// with no country_code beacons on nothing in most of the world.
	Country string

	// TUNDisabled leaves the TUN inbound out of the engine document.
	//
	// It exists because a developer machine has no /dev/net/tun and no root.
	// A service running with this set configures the network, starts an
	// engine and CARRIES NO CLIENT TRAFFIC, so it is never right on the
	// appliance. cmd/caspian never sets it.
	TUNDisabled bool

	// Engine is the tunnel. nil means a real engine with the default log ring.
	Engine Engine

	// Resolver turns the server hostname into addresses. nil means the host
	// resolver.
	Resolver Resolver

	// Reach probes the server. nil means a TCP dial. See Reachability for what
	// that proves and what it does not.
	Reach Reachability

	// Logger receives this service's own lines. nil discards them.
	//
	// Nothing written through it carries the pasted config, the config
	// document, the hotspot passphrase or any part of any of them. The values
	// never reach a formatting verb: panel.StartRequest and panel.HotspotSpec
	// redact themselves through String and GoString, and this package logs
	// only fixed words plus a fingerprint.
	Logger *slog.Logger

	// Now is the clock. nil means time.Now.
	Now func() time.Time

	// ClockFloor is the earliest wall-clock time this service will attempt a
	// connection at. See clock.go. Zero means DefaultClockFloor.
	ClockFloor time.Time

	// DetectTTL is how long a detection is reused for a Status poll. Zero
	// means DefaultDetectTTL. The panel polls status every couple of seconds
	// and a full detection runs five commands, so re-running it per poll would
	// spend the box's CPU on a picture that does not change that fast.
	DetectTTL time.Duration
}

// DefaultDetectTTL is how long Status reuses the last detection.
const DefaultDetectTTL = 10 * time.Second

// DefaultEngineLogCapacity is the size of the log ring a service builds when
// the caller supplies no engine.
const DefaultEngineLogCapacity = 256

func (c Config) check() error {
	switch {
	case c.Runner == nil:
		return errors.New("privsvc: Config.Runner is required")
	case c.System == nil:
		return errors.New("privsvc: Config.System is required")
	case c.JournalPath == "":
		return errors.New("privsvc: Config.JournalPath is required; docs/LAYOUT.md fixes /var/lib/caspian/netcfg.journal")
	case c.HotspotPaths.HostapdConf == "" || c.HotspotPaths.DnsmasqConf == "":
		return errors.New("privsvc: Config.HotspotPaths is required; it decides which processes the supervisor may stop")
	case c.SocksPort == 0:
		return errors.New("privsvc: Config.SocksPort is required; docs/LAYOUT.md fixes 10808")
	case c.LocalDNSPort == 0:
		return errors.New("privsvc: Config.LocalDNSPort is required; docs/LAYOUT.md fixes 5354")
	case c.DNSPort == 0:
		return errors.New("privsvc: Config.DNSPort is required; docs/LAYOUT.md fixes 53")
	case c.PanelPort == 0:
		return errors.New("privsvc: Config.PanelPort is required; docs/LAYOUT.md fixes 8088")
	}
	return nil
}

func (c Config) withDefaults() Config {
	if c.TunName == "" {
		c.TunName = netcfg.DefaultOptions().TunName
	}
	if c.Engine == nil {
		c.Engine = engine.NewWithLogCapacity(DefaultEngineLogCapacity)
	}
	if c.Resolver == nil {
		c.Resolver = systemResolver{}
	}
	if c.Reach == nil {
		c.Reach = tcpReachability{}
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.DiscardHandler)
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.ClockFloor.IsZero() {
		c.ClockFloor = DefaultClockFloor
	}
	if c.DetectTTL == 0 {
		c.DetectTTL = DefaultDetectTTL
	}
	return c
}

// netOptions builds the netcfg options from the ports this service was given,
// so that no value is written twice.
func (c Config) netOptions() netcfg.Options {
	o := netcfg.DefaultOptions()
	if c.TunName != "" {
		// Guarded rather than assigned. netcfg.PlanNetwork replaces the WHOLE
		// options value when TunName is empty, which would silently discard
		// the ports this appliance was told to use and put the panel back on
		// the default port. New fills TunName in, so this is defence against a
		// Config built by hand.
		o.TunName = c.TunName
	}
	o.DNSPort = c.DNSPort
	o.PanelPort = c.PanelPort
	return o
}
