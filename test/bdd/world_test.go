// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package bdd

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/hotspot"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/state"
)

// ---------------------------------------------------------------------------
// The credentials. Every value here is invented.
//
// These are the same invented values as internal/link/fixtures_test.go, which
// states next to each one the length requirement it satisfies. They are
// repeated rather than imported because that file is a _test.go file and its
// identifiers are unexported, so there is no way to reach them from another
// package. If the values there change, the assertions here still hold: nothing
// below depends on the particular bytes, only on the fact that these bytes are
// the secret and must not turn up anywhere but the two places that need them.
//
// Nothing here is, or has ever been, a working credential.
// ---------------------------------------------------------------------------

const (
	// fakeUUID is a syntactically valid UUID, 36 characters, 8-4-4-4-12.
	fakeUUID = "11111111-2222-4333-8444-555555555555"

	// fakeShortID is ten hexadecimal characters, inside the engine's limit.
	fakeShortID = "0a1b2c3d4e"

	fakeHost = "example.invalid"
	fakeSNI  = "www.fake-front.invalid"
	fakeAuth = "not-a-real-auth-string"

	// fakeHotspotPassphrase is a WPA2 passphrase this suite chooses so that it
	// has a known string to hunt for. It is 24 printable ASCII characters and
	// is not one of internal/hotspot's banned well-known defaults.
	fakeHotspotPassphrase = "qtdw-3ngz-7mkr-2vsp-9xhb"
)

// fakePublicKey is 43 characters of base64url decoding to exactly 32 bytes,
// which is what the engine requires of a REALITY public key.
func fakePublicKey() string {
	return base64.RawURLEncoding.EncodeToString([]byte("CASPIAN-FAKE-REALITY-PUBKEY-3232"))
}

// realityShareLink is what the user pastes. It carries the REALITY parameters
// whose URI names differ from their config keys, which is the shape most likely
// to be got wrong, and a #fragment, which is where the parser puts the display
// name and which has to be cleared before the engine will build the config.
func realityShareLink() string {
	return "vless://" + fakeUUID + "@" + fakeHost + ":443" +
		"?security=reality&type=raw&flow=xtls-rprx-vision" +
		"&sni=" + fakeSNI +
		"&fp=chrome" +
		"&pbk=" + fakePublicKey() +
		"&sid=" + fakeShortID +
		"&spx=%2Fspider" +
		"#Living%20room%20box"
}

// insecureShareLink parses and the engine refuses it.
//
// insecure=1 is the common shape for a self-signed server. The vendored parser
// carries it through to tlsSettings.allowInsecure, and xray-core v1.260327.0
// refuses any config containing it, pointing at pinnedPeerCertSha256 instead.
// That refusal is gated on the WALL CLOCK, not on the version: the same binary
// accepted it before 2026-06-01. See allowInsecureGate below, which turns a
// silently-passing scenario into a loud failure on a box whose clock is behind.
func insecureShareLink() string {
	return "hysteria2://" + fakeAuth + "@" + fakeHost + ":443?insecure=1#Self-signed"
}

// allowInsecureGate is the date xray-core v1.260327.0 starts refusing
// allowInsecure, read from infra/conf/transport_internet.go:709-716 by way of
// internal/link's TestAllowInsecureIsRejectedByTheEngine_KnownTrap.
var allowInsecureGate = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

// secrets is every value that must not appear in anything a user or a log file
// can see. See theCredentialAppearsInNothingTheUserOrALogCanSee.
func secrets() []string {
	return []string{fakeUUID, fakePublicKey(), fakeShortID, fakeAuth, fakeHotspotPassphrase}
}

// ---------------------------------------------------------------------------
// The machine.
//
// These are the bytes "ip", "iw" and "sysctl" print. They describe mode A of
// the design (section 4.7): the internet arrives on a wired interface, and the
// built-in radio already carries a client link, so the access point has to be
// added beside it on the same channel.
//
// PROVENANCE, because it decides what a green test proves. These are AUTHORED
// bytes in the SHAPE of a real capture, not a capture. internal/netcfg holds
// real captures from a Raspberry Pi 5 taken on 2026-08-30 and records their
// provenance in internal/netcfg/testdata/PROVENANCE.md; this suite does not
// read them, because that corpus is that package's to rename and reorganise
// and a behaviour suite that breaks when a fixture is renamed is a behaviour
// suite nobody will keep. What this costs is real and worth stating: if the
// real "iw list" output changes shape, internal/netcfg's own tests catch it and
// these do not. Parser fidelity is that package's claim, not this one's.
//
// Every address is RFC 1918 or RFC 5737 documentation space.
// ---------------------------------------------------------------------------

const (
	fixtureIPBriefAddr = "" +
		"lo               UNKNOWN        127.0.0.1/8 ::1/128 \n" +
		"eth0             UP             192.168.1.42/24 fe80::dea6:32ff:fe11:2233/64 \n" +
		"wlan0            UP             192.168.1.57/24 fe80::dea6:32ff:fe11:2234/64 \n"

	fixtureIPRouteDefault = "" +
		"default via 192.168.1.1 dev eth0 proto dhcp src 192.168.1.42 metric 100 \n" +
		"default via 192.168.1.1 dev wlan0 proto dhcp src 192.168.1.57 metric 600 \n"

	fixtureIPRoute6Default = "default via fe80::1 dev eth0 proto ra metric 1024 pref medium\n"

	fixtureIPDetailLink = "" +
		"1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000\n" +
		"    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00 promiscuity 0 minmtu 0 maxmtu 0 addrgenmode eui64 numtxqueues 1 numrxqueues 1 \n" +
		"2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc fq_codel state UP mode DEFAULT group default qlen 1000\n" +
		"    link/ether 02:00:5e:01:00:10 brd ff:ff:ff:ff:ff:ff promiscuity 0 minmtu 68 maxmtu 9000 addrgenmode none numtxqueues 1 numrxqueues 1 parentbus platform parentdev 1f00c00000.ethernet \n" +
		"3: wlan0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc fq_codel state UP mode DORMANT group default qlen 1000\n" +
		"    link/ether 02:00:5e:01:00:11 brd ff:ff:ff:ff:ff:ff promiscuity 0 minmtu 68 maxmtu 1500 addrgenmode eui64 numtxqueues 1 numrxqueues 1 parentbus sdio parentdev mmc1:0001:1 \n"

	fixtureIWDev = "" +
		"phy#0\n" +
		"\tInterface wlan0\n" +
		"\t\tifindex 3\n" +
		"\t\twdev 0x1\n" +
		"\t\taddr 02:00:5e:01:00:11\n" +
		"\t\tssid HomeNet\n" +
		"\t\ttype managed\n" +
		"\t\tchannel 10 (2457 MHz), width: 20 MHz, center1: 2457 MHz\n" +
		"\t\ttxpower 31.00 dBm\n"

	// fixtureIWList is trimmed to the parts the planner reads: the interface
	// modes, the bands with their per-channel flags, and the interface
	// combinations. The second combination is the one that matters on this
	// hardware: it permits an access point beside a client link and pins both
	// to one channel.
	fixtureIWList = "" +
		"Wiphy phy0\n" +
		"\twiphy index: 0\n" +
		"\tSupported interface modes:\n" +
		"\t\t * IBSS\n" +
		"\t\t * managed\n" +
		"\t\t * AP\n" +
		"\t\t * P2P-client\n" +
		"\t\t * P2P-GO\n" +
		"\t\t * P2P-device\n" +
		"\tBand 1:\n" +
		"\t\tFrequencies:\n" +
		"\t\t\t* 2412.0 MHz [1] (20.0 dBm)\n" +
		"\t\t\t* 2437.0 MHz [6] (20.0 dBm)\n" +
		"\t\t\t* 2457.0 MHz [10] (20.0 dBm)\n" +
		"\t\t\t* 2472.0 MHz [13] (20.0 dBm)\n" +
		"\tBand 2:\n" +
		"\t\tFrequencies:\n" +
		"\t\t\t* 5180.0 MHz [36] (23.0 dBm)\n" +
		"\t\t\t* 5260.0 MHz [52] (23.0 dBm) (radar detection)\n" +
		"\t\t\t* 5745.0 MHz [149] (30.0 dBm) (no IR)\n" +
		"\t\t\t* 5825.0 MHz [165] (disabled)\n" +
		"\tSupported commands:\n" +
		"\t\t * new_interface\n" +
		"\t\t * set_interface\n" +
		"\t\t * start_ap\n" +
		"\tvalid interface combinations:\n" +
		"\t\t * #{ managed } <= 1, #{ P2P-device } <= 1, #{ P2P-client, P2P-GO } <= 1,\n" +
		"\t\t   total <= 3, #channels <= 2\n" +
		"\t\t * #{ managed } <= 1, #{ AP } <= 1, #{ P2P-client } <= 1, #{ P2P-device } <= 1,\n" +
		"\t\t   total <= 4, #channels <= 1\n"

	// fixtureIWListNoAP describes a radio that cannot host an access point.
	// It is the machine the panel has to explain in words rather than in
	// wireless vocabulary.
	fixtureIWListNoAP = "" +
		"Wiphy phy0\n" +
		"\twiphy index: 0\n" +
		"\tSupported interface modes:\n" +
		"\t\t * managed\n" +
		"\t\t * monitor\n" +
		"\tBand 1:\n" +
		"\t\tFrequencies:\n" +
		"\t\t\t* 2412.0 MHz [1] (20.0 dBm)\n" +
		"\tvalid interface combinations:\n" +
		"\t\t * #{ managed } <= 1, total <= 1, #channels <= 1\n"
)

// kernelKnobs is what "sysctl -e -- <names>" reports on the fresh box: nothing
// forwards and reverse-path filtering is strict, which is the state a teardown
// has to be able to put back.
func kernelKnobs() map[string]string {
	return map[string]string{
		"net.ipv4.ip_forward":             "0",
		"net.ipv4.conf.all.rp_filter":     "1",
		"net.ipv4.conf.default.rp_filter": "1",
		"net.ipv6.conf.all.forwarding":    "0",
		"net.ipv4.conf.eth0.rp_filter":    "1",
		"net.ipv4.conf.wlan0.rp_filter":   "1",
		"net.ipv4.conf.ap0.rp_filter":     "1",
		"net.ipv4.conf.xray0.rp_filter":   "1",
		"net.ipv6.conf.ap0.disable_ipv6":  "0",
	}
}

// ---------------------------------------------------------------------------
// Deliberate defects.
//
// Every scenario names one, and TestEveryScenarioCanFail injects it and
// requires the scenario to go red. A scenario nobody has watched fail is not
// evidence, so the fields below are the executable half of the mutation table
// rather than a list somebody typed after the fact.
//
// Each defect is injected at a seam THIS package owns: either in the order the
// appliance composes the packages, or in an artifact on its way to an
// assertion. Nothing under internal/ is edited to produce one. For the
// artifact defects that matters, because injecting a rule into the generated
// ruleset produces exactly the text a regression in the generator would
// produce, which is what the assertion has to be able to catch.
// ---------------------------------------------------------------------------

type defects struct {
	// Composition defects: the appliance wires the packages up wrongly.
	skipRecovery                bool // do not replay a journal left by a killed process
	detectBeforeParse           bool // touch the machine before the pasted text is read
	firewallAfterForwarding     bool // enable forwarding before the ruleset is loaded
	postEngineStepsBeforeEngine bool // apply the tunnel steps before the engine exists
	restartHotspotEveryConnect  bool // rewrite the config on every connect, forcing a restart
	skipTeardownOfRoutes        bool // leave the routing steps out of the journal
	classifyEveryFailureAsParse bool // tell the user to fix their text whatever went wrong
	dropPinnedServerRoute       bool // leave out the host route that keeps the engine off its own tunnel
	swallowRefusals             bool // wrap a typed refusal so the panel cannot word it
	skipFirewallOnUplinkChange  bool // move the routes when the uplink moves and leave the block behind
	collideTheHotspotSubnet     bool // override the hotspot subnet onto the network the box is on

	// Artifact defects: the text an assertion reads is damaged the way a
	// regression in the generator would damage it.
	ruleset   func(string) string
	logLines  func([]string) []string
	engineCfg func([]byte) []byte
	detection func(*panel.Detection)

	// dnsmasqConf damages the generated DHCP and DNS configuration. It is
	// separate from ruleset because the two artefacts fail differently: a
	// firewall defect is visible in the rules an operator can list, and a
	// defect in what this file OFFERS a joining device is not visible
	// anywhere on the wire, because the prerouting redirect rewrites the
	// consequence.
	dnsmasqConf func(string) string
}

func (d defects) mutateRuleset(s string) string {
	if d.ruleset == nil {
		return s
	}
	return d.ruleset(s)
}

func (d defects) mutateLogLines(l []string) []string {
	if d.logLines == nil {
		return l
	}
	return d.logLines(l)
}

func (d defects) mutateEngineConfig(b []byte) []byte {
	if d.engineCfg == nil {
		return b
	}
	return d.engineCfg(b)
}

func (d defects) mutateDnsmasq(s string) string {
	if d.dnsmasqConf == nil {
		return s
	}
	return d.dnsmasqConf(s)
}

// ---------------------------------------------------------------------------
// Fakes at the system boundary.
//
// Each says what it does NOT model. A fake that cannot fail teaches nothing,
// and a fake whose limits are not written down is read as the real thing.
// ---------------------------------------------------------------------------

// resolver turns the host in a share link into addresses to pin a host route
// to. The appliance needs this before the tunnel exists, so it is a real
// question and not a detail.
type resolver interface {
	Resolve(ctx context.Context, host string) ([]netip.Addr, error)
}

// fakeResolver answers with one fixed address.
//
// WHAT IT DOES NOT MODEL, and each of these is a real behaviour of the thing it
// stands in for:
//   - it never returns more than one address, so nothing here exercises a
//     server with several A records, which produces several pinned host routes;
//   - it never returns AAAA, so the IPv6 half of ServerRouteSteps, the half
//     that has to be skipped when the box has no IPv6 gateway, is not driven;
//   - it cannot be slow, so no timeout path is exercised;
//   - and it says nothing about the fact that a real resolution performed
//     BEFORE the tunnel is up leaves the box in the clear, out of the uplink,
//     carrying the name of the user's proxy server. That is a design question
//     this suite raises and does not answer.
type fakeResolver struct {
	addr netip.Addr
	err  error
}

func (r fakeResolver) Resolve(_ context.Context, host string) ([]netip.Addr, error) {
	if r.err != nil {
		return nil, r.err
	}
	return []netip.Addr{r.addr}, nil
}

// reachability answers the third of the design's three failure states: the
// config loaded, and the server did not answer.
type reachability interface {
	Probe(ctx context.Context) error
}

// fakeServer answers, or does not.
//
// WHAT IT DOES NOT MODEL: everything about the connection except whether it
// happened. There is no TLS, no REALITY handshake, no clock in the handshake,
// no authentication, no round trip and no exit IP. It cannot tell a server that
// refused the connection from one that accepted it and rejected the user, and
// those need different words on the panel. It is a two-valued stand-in for the
// exit-IP capture the design requires (section 6) and which no test in this
// package performs.
type fakeServer struct {
	answers bool
}

var errServerSilent = errors.New("the proxy server did not answer")

func (s fakeServer) Probe(_ context.Context) error {
	if s.answers {
		return nil
	}
	return errServerSilent
}

// ---------------------------------------------------------------------------
// The timeline.
//
// Two recorders on their own cannot answer an ordering question that spans
// them: "was the firewall loaded before the engine started" is a fact about one
// sequence, not about two. So every effect the appliance has, from either
// recorder, and every lifecycle event of the engine and the hotspot, is
// appended to one list in the order it happened.
// ---------------------------------------------------------------------------

type timeline struct {
	mu     sync.Mutex
	events []string
}

func (t *timeline) add(e string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, e)
}

func (t *timeline) all() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.events...)
}

// indexOf returns the position of the first event containing needle, or -1.
func (t *timeline) indexOf(needle string) int {
	for i, e := range t.all() {
		if strings.Contains(e, needle) {
			return i
		}
	}
	return -1
}

// lastIndexOf returns the position of the last event containing needle, or -1.
func (t *timeline) lastIndexOf(needle string) int {
	events := t.all()
	for i := len(events) - 1; i >= 0; i-- {
		if strings.Contains(events[i], needle) {
			return i
		}
	}
	return -1
}

func (t *timeline) String() string { return strings.Join(t.all(), "\n  ") }

// tracedRunner records every netcfg command on the shared timeline before
// passing it to the recorder that answers it.
type tracedRunner struct {
	inner netcfg.Runner
	tl    *timeline
}

func (r tracedRunner) Run(ctx context.Context, c netcfg.Command) (netcfg.Result, error) {
	r.tl.add("net: " + netcfg.RunnerKey(c))
	return r.inner.Run(ctx, c)
}

// tracedSystem does the same for the hotspot's System. The interface is
// embedded so that a method added to it upstream is carried through unchanged
// rather than silently dropped.
type tracedSystem struct {
	hotspot.System
	tl *timeline
}

func (s tracedSystem) Run(ctx context.Context, name string, args ...string) (hotspot.Result, error) {
	s.tl.add("hotspot: " + name + " " + strings.Join(args, " "))
	return s.System.Run(ctx, name, args...)
}

func (s tracedSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	s.tl.add("hotspot: write " + path)
	return s.System.WriteFile(path, data, perm)
}

// ---------------------------------------------------------------------------
// The World
// ---------------------------------------------------------------------------

// World is everything one scenario can see and do. One is built per scenario
// and torn down after it, so no scenario can leave state for the next.
type World struct {
	t    *testing.T
	ctx  context.Context
	dir  string
	tl   *timeline
	defs defects

	// The machine, and the two recorders that stand in for it.
	runner  *netcfg.RecordingRunner
	sys     *hotspot.Recorder
	iwList  string // which radio the box has; a scenario may replace it
	knobs   map[string]string
	store   *state.Store
	eng     *engine.Engine
	socksAt uint16
	resolve resolver

	// localDNSAt is the port the engine's DNS listener binds AND the port
	// dnsmasq is told to forward to. ONE field, two consumers, which is how
	// internal/privsvc wires it (plans.go: engineDocument sets
	// o.LocalDNS.Port and hotspotPlanFor sets Upstream, both from
	// Config.LocalDNSPort). Until 2026-08-30 this suite gave the two halves
	// separate values and never enabled the listener, so the chain it checked
	// stopped at dnsmasq's configuration text and the appliance's actual
	// client DNS path was not modelled here at all.
	localDNSAt uint16
	server     reachability
	supervis   *hotspot.Supervisor

	// What the user did.
	pasted string

	// What the run produced. Every field here is written by the appliance and
	// read by the Then steps.
	lnk         *link.Link
	facts       netcfg.Facts
	plan        *netcfg.Plan
	engineCfg   []byte
	preSteps    []netcfg.Step
	postSteps   []netcfg.Step
	hotspotPlan hotspot.Plan
	hotspotStat hotspot.Status
	detection   panel.Detection
	status      panel.SystemStatus
	problem     panel.Problem
	connectErr  error
	teardown    netcfg.Report
	recovered   netcfg.Report
	applier     *netcfg.Applier

	// errs is every error the flow produced, kept so the secret scan can read
	// all of them and not only the last.
	errs []error
	// logs is everything this appliance would have written to a log.
	logs []string

	// leftover is the undo command of the change a killed process left behind,
	// so the recovery scenario can look for it by name.
	leftover string

	// uplinkNow is where the internet moved to, when a scenario moved it. The
	// appliance is deliberately not told; see theInternetMovesToADifferentInterface.
	uplinkNow netcfg.UplinkState

	// rulesetInForce is the firewall text most recently loaded. It is tracked
	// separately because a command's stdin does not appear in the timeline, so
	// "which ruleset is in force" cannot be read back out of it.
	rulesetInForce string

	// Marks taken after the first connect, so a second connect can be compared
	// against it rather than against a hardcoded expectation.
	engineRunningSince time.Time
	hostapdStarts      int
	dnsmasqStarts      int
}

func newWorld(t *testing.T, d defects) *World {
	t.Helper()
	// The state directory is 0700, which is what docs/LAYOUT.md fixes for
	// /var/lib/caspian and what the installer creates. internal/state enforces
	// it on every load rather than trusting whatever Save last set, so a
	// directory the test framework made world-readable is refused here exactly
	// as it would be on the box.
	dir := filepath.Join(t.TempDir(), "caspian")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the state directory: %v", err)
	}
	tl := &timeline{}

	rec := netcfg.NewRecordingRunner()
	w := &World{
		t:          t,
		ctx:        context.Background(),
		dir:        dir,
		tl:         tl,
		defs:       d,
		runner:     rec,
		sys:        hotspot.NewRecorder(),
		iwList:     fixtureIWList,
		knobs:      kernelKnobs(),
		socksAt:    freePort(t),
		localDNSAt: freePort(t),
		resolve:    fakeResolver{addr: netip.MustParseAddr("203.0.113.10")},
		server:     fakeServer{answers: true},
	}
	w.primeMachine()

	store, err := state.Load(dir)
	if err != nil {
		t.Fatalf("state.Load(%s): %v", dir, err)
	}
	w.store = store
	return w
}

// primeMachine loads the fixture answers into the recorder. It is a method so
// that a scenario can change the machine (a radio with no AP support, no
// default route) and re-prime.
func (w *World) primeMachine() {
	w.runner.SetOutput("ip -br addr", fixtureIPBriefAddr)
	w.runner.SetOutput("ip route show default", fixtureIPRouteDefault)
	w.runner.SetOutput("ip -6 route show default", fixtureIPRoute6Default)
	w.runner.SetOutput("ip -d link show", fixtureIPDetailLink)
	w.runner.SetOutput("iw dev", fixtureIWDev)
	w.runner.SetOutput("iw list", w.iwList)

	// The sysctl read's arguments depend on which interfaces the plan chose,
	// so it cannot be keyed exactly.
	//
	// Two things about this answer are deliberate and neither is convenience.
	//
	// It answers in the "name = value" form. The parser splits on "=", so a
	// bare column of numbers, which is what "sysctl -n" prints, yields an empty
	// map, every change records no inverse, and uninstall leaves ip_forward and
	// rp_filter changed on a box that promised to return them.
	//
	// It answers for existing interfaces ONLY. Detection runs before anything
	// is applied, so the access point's interface and the tunnel device do not
	// exist yet, and there is no /proc file for their knobs. "sysctl -e" skips
	// a knob it cannot read rather than failing the whole read, so those knobs
	// come back absent. A fake that answered for them would hide a real
	// consequence: the changes to them have no recorded inverse. See
	// everyRecordedChangeIsUndone, which expects exactly that set and says why
	// each member of it is acceptable.
	knobs := w.knobs
	present := map[string]bool{"lo": true, "eth0": true, "wlan0": true, "all": true, "default": true}
	w.runner.Fallback = func(c netcfg.Command) (netcfg.Result, error) {
		if c.Path != netcfg.BinSysctl {
			return netcfg.Result{}, nil
		}
		var b strings.Builder
		for _, a := range c.Args {
			if strings.HasPrefix(a, "-") {
				continue
			}
			if iface, ok := interfaceOfKnob(a); ok && !present[iface] {
				continue
			}
			if v, ok := knobs[a]; ok {
				fmt.Fprintf(&b, "%s = %s\n", a, v)
			}
		}
		return netcfg.Result{Stdout: b.String()}, nil
	}
}

// interfaceOfKnob returns the interface a per-interface kernel knob belongs to.
// net.ipv4.conf.eth0.rp_filter is eth0; net.ipv4.ip_forward belongs to no
// interface.
func interfaceOfKnob(knob string) (string, bool) {
	parts := strings.Split(knob, ".")
	if len(parts) < 5 || parts[0] != "net" || parts[2] != "conf" {
		return "", false
	}
	return parts[3], true
}

func (w *World) close() {
	if w.eng != nil {
		_ = w.eng.Stop()
	}
}

// tracedNetRunner is the runner the appliance uses: the recorder, with every
// command copied onto the timeline first.
func (w *World) tracedNetRunner() netcfg.Runner { return tracedRunner{inner: w.runner, tl: w.tl} }

func (w *World) tracedHotspotSystem() hotspot.System { return tracedSystem{System: w.sys, tl: w.tl} }

func (w *World) journalPath() string { return filepath.Join(w.dir, "netcfg.journal") }

func (w *World) event(e string) { w.tl.add(e) }

// note records a line the appliance would have logged. Everything here is
// scanned for credentials.
func (w *World) note(format string, args ...any) {
	w.logs = append(w.logs, fmt.Sprintf(format, args...))
}

// fail records an error and returns it, so that every error the flow produced
// is available to the secret scan and not only the one that was returned.
func (w *World) fail(err error) error {
	if err != nil {
		w.errs = append(w.errs, err)
	}
	return err
}

// freePort asks the kernel for a port and gives it straight back, so the
// engine's diagnostics inbound has somewhere to listen that is not in use.
//
// There is a race here between the close and the engine's bind. It is accepted
// rather than hidden: the alternative is a fixed port, which collides with a
// developer's own running engine, and xcfg refuses port 0 because the engine
// accepts it and then listens on nothing.
func freePort(t *testing.T) uint16 {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	defer ln.Close()
	return uint16(ln.Addr().(*net.TCPAddr).Port)
}
