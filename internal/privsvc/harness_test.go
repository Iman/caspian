// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
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
// The credentials. Every value here is invented and none has ever been a
// working credential.
//
// They are the same invented values test/bdd uses, repeated rather than
// imported because that suite's identifiers are unexported in a _test.go file
// in another package. Nothing below depends on the particular bytes, only on
// the fact that these bytes are the secret and must not appear anywhere but the
// two places that need them.
// ---------------------------------------------------------------------------

const (
	fakeUUID    = "11111111-2222-4333-8444-555555555555"
	fakeShortID = "0a1b2c3d4e"
	fakeHost    = "example.invalid"
	fakeSNI     = "www.fake-front.invalid"

	// fakeHotspotPassphrase is 24 printable ASCII characters and is not one of
	// internal/hotspot's banned well-known defaults.
	fakeHotspotPassphrase = "qtdw-3ngz-7mkr-2vsp-9xhb"

	fakeSSID = "Caspian-Living-Room"
)

func fakePublicKey() string {
	// 43 characters of base64url decoding to exactly 32 bytes, which is what
	// the engine requires of a REALITY public key.
	return base64.RawURLEncoding.EncodeToString([]byte("CASPIAN-FAKE-REALITY-PUBKEY-3232"))
}

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

// secrets is every value that must not appear in a log line.
func secrets() []string {
	return []string{fakeUUID, fakePublicKey(), fakeShortID, fakeHotspotPassphrase}
}

// ---------------------------------------------------------------------------
// The machine.
//
// PROVENANCE, because it decides what a green test proves. These are AUTHORED
// bytes in the SHAPE of real command output, not a capture. internal/netcfg
// holds real captures from a Raspberry Pi 5 and its own tests assert against
// them; this package does not read that corpus, because it belongs to that
// package to rename and reorganise. What that costs is worth stating: if the
// real "iw list" output changes shape, internal/netcfg's tests catch it and
// these do not. Parser fidelity is that package's claim, not this one's.
//
// The machine described is mode A of docs/2026-08-29-design.md section 4.7: the
// internet arrives on a wired interface, and the built-in radio already carries
// a client link, so the access point is added beside it on the same channel.
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

	// The internet arrives over WiFi and there is no wired default route, so
	// the only AP-capable radio is the one carrying the uplink. This is mode B
	// of docs/2026-08-29-design.md section 4.7 without the USB adapter that
	// mode assumes, which is the machine on which taking over an interface
	// would cut the box off from the internet it exists to share.
	fixtureIPRouteDefaultWirelessOnly = "" +
		"default via 192.168.1.1 dev wlan0 proto dhcp src 192.168.1.57 metric 600 \n"

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

	// The second combination is the one that matters on this hardware: it
	// permits an access point beside a client link and pins both to one
	// channel.
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

	// The shape "iw reg get" prints. The country line is what
	// parseRegDomain reads.
	fixtureIWRegGet = "" +
		"global\n" +
		"country GB: DFS-ETSI\n" +
		"\t(2402 - 2482 @ 40), (N/A, 20), (N/A)\n" +
		"\t(5170 - 5250 @ 80), (N/A, 20), (N/A), AUTO-BW\n"

	// What NetworkManager says it holds. The shape is the one the target
	// prints, "DEVICE:STATE" per line with a possible parenthetical, and the
	// answer matters: internal/netcfg refuses to take over an interface whose
	// owner it cannot name, because not knowing what holds an interface is the
	// state that put a DHCP server on somebody's home network.
	fixtureNmcliDeviceStatus = "" +
		"eth0:connected\n" +
		"wlan0:connected\n" +
		"lo:connected (externally)\n"

	// A radio under the world domain, which is what a box reports when nothing
	// has set a country.
	fixtureIWRegGetWorld = "" +
		"global\n" +
		"country 00: DFS-UNSET\n" +
		"\t(2402 - 2472 @ 40), (N/A, 20), (N/A)\n"
)

// kernelKnobs is what the fresh box reports: nothing forwards and reverse-path
// filtering is strict, which is the state a teardown has to be able to put back.
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
// One ordered trail of everything that happened.
//
// Two recorders on their own cannot answer an ordering question that spans
// them: "was the firewall loaded before the engine started" is a fact about one
// sequence, not about two. So every netcfg command, every hotspot effect and
// every engine lifecycle event is appended to one list in the order it happened.
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

func (t *timeline) indexOf(needle string) int {
	for i, e := range t.all() {
		if strings.Contains(e, needle) {
			return i
		}
	}
	return -1
}

func (t *timeline) lastIndexOf(needle string) int {
	ev := t.all()
	for i := len(ev) - 1; i >= 0; i-- {
		if strings.Contains(ev[i], needle) {
			return i
		}
	}
	return -1
}

func (t *timeline) count(needle string) int {
	n := 0
	for _, e := range t.all() {
		if strings.Contains(e, needle) {
			n++
		}
	}
	return n
}

func (t *timeline) String() string { return "\n  " + strings.Join(t.all(), "\n  ") }

// mustBefore fails when a is not strictly before b.
func mustBefore(t *testing.T, tl *timeline, a, b, why string) {
	t.Helper()
	ia, ib := tl.indexOf(a), tl.indexOf(b)
	if ia < 0 {
		t.Fatalf("%q never happened, so the ordering %q before %q cannot hold.\ntimeline:%s", a, a, b, tl)
	}
	if ib < 0 {
		t.Fatalf("%q never happened, so the ordering %q before %q cannot hold.\ntimeline:%s", b, a, b, tl)
	}
	if ia >= ib {
		t.Fatalf("%q (at %d) must come before %q (at %d): %s\ntimeline:%s", a, ia, b, ib, why, tl)
	}
}

// tracedRunner records every netcfg command on the shared timeline before
// passing it to the recorder that answers it.
//
// It also REFUSES a command whose context is already done, which
// netcfg.RecordingRunner does not: that type ignores the context entirely. The
// real runner does not. internal/netcfg/exec_linux.go builds every command with
// exec.CommandContext, so on a real box a cancelled context makes every command
// fail before it starts.
//
// This matters for one behaviour and it was found by watching a mutation stay
// green: a rollback that ran on the caller's own cancelled context would fail
// every inverse on a real box and undo nothing, and the recorder would have
// reported a clean rollback. A double that is more forgiving than the thing it
// stands in for turns a defect into a passing test.
type tracedRunner struct {
	inner netcfg.Runner
	tl    *timeline
}

func (r tracedRunner) Run(ctx context.Context, c netcfg.Command) (netcfg.Result, error) {
	r.tl.add("net: " + netcfg.RunnerKey(c))
	if err := ctx.Err(); err != nil {
		return netcfg.Result{}, err
	}
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

// recordingEngine stands in for internal/engine.
//
// WHAT IT DOES NOT MODEL, because a fake whose limits are not written down is
// read as the real thing: it does not load the configuration, it does not
// create a tunnel device, it does not dial anything and it cannot fail for any
// reason the real engine fails for. It exists so the ordering AROUND the moment
// the tunnel device appears can be observed at all: the real engine's TUN
// inbound opens /dev/net/tun, which needs Linux and root.
//
// TestTheRealEngineLoadsTheComposedDocument drives the real engine with the
// same document to cover what this cannot.
type recordingEngine struct {
	tl  *timeline
	now func() time.Time

	mu      sync.Mutex
	starts  int
	stops   int
	running bool
	docs    [][]byte
	logAt   time.Time

	startErr error
}

func (e *recordingEngine) Start(_ context.Context, doc []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		// The real engine's Start is idempotent and returns nil when an
		// instance is already running. Copying that is what makes a second
		// Start here mean what it means there.
		return nil
	}
	e.starts++
	e.docs = append(e.docs, append([]byte(nil), doc...))
	if e.startErr != nil {
		e.tl.add("engine: refused the configuration")
		return e.startErr
	}
	e.running = true
	// Timestamped from the world's clock, so the one line this fake retains
	// interleaves with the service's own diagnostic lines the way the real
	// engine's would. A fake that stamped every line with the same instant, or
	// with the zero time, would make Service.EngineLog's merge trivially
	// ordered and the test of that merge would prove nothing.
	if e.now != nil {
		e.logAt = e.now()
	}
	e.tl.add("engine: started; the tunnel device exists from here on")
	return nil
}

func (e *recordingEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return nil
	}
	e.stops++
	e.running = false
	e.tl.add("engine: stopped; the tunnel device is gone from here on")
	return nil
}

func (e *recordingEngine) State() engine.State {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return engine.State{Phase: engine.PhaseRunning}
	}
	return engine.State{Phase: engine.PhaseStopped}
}

func (e *recordingEngine) Logs() []engine.LogEntry {
	e.mu.Lock()
	defer e.mu.Unlock()
	return []engine.LogEntry{{At: e.logAt, Text: "a redacted engine line"}}
}

func (e *recordingEngine) LogsDropped() uint64 { return 3 }

func (e *recordingEngine) startCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.starts
}

// documents returns every configuration the engine was handed, in order.
func (e *recordingEngine) documents() [][]byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([][]byte(nil), e.docs...)
}

// fakeResolver answers with one fixed address.
//
// WHAT IT DOES NOT MODEL: several A records, any AAAA at all, a slow lookup,
// and the fact that a real lookup performed BEFORE the tunnel is up leaves the
// box in the clear carrying the name of the user's proxy server.
type fakeResolver struct {
	addr netip.Addr
	err  error

	mu sync.Mutex
	n  int
}

func (r *fakeResolver) Resolve(context.Context, string) ([]netip.Addr, error) {
	r.mu.Lock()
	r.n++
	r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	return []netip.Addr{r.addr}, nil
}

// lookups is how many times the server name was looked up. It is asserted on
// because a lookup leaves the box in the clear, out of the uplink, carrying the
// name of the user's proxy server: a request that was going to be refused must
// not cause one.
func (r *fakeResolver) lookups() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.n
}

// fakeReach answers, or does not.
//
// WHAT IT DOES NOT MODEL: everything about the connection except whether it
// happened. No TLS, no REALITY handshake, no authentication, no round trip and
// no exit IP.
type fakeReach struct {
	err error
	mu  sync.Mutex
	n   int
}

func (r *fakeReach) Probe(context.Context, netip.Addr, uint16) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.n++
	return r.err
}

// ---------------------------------------------------------------------------
// The world one test runs in
// ---------------------------------------------------------------------------

type world struct {
	t   *testing.T
	dir string
	tl  *timeline

	clock    *advancingClock
	runner   *machine
	sys      *hotspot.Recorder
	eng      *recordingEngine
	reach    *fakeReach
	resolver *fakeResolver
	logs     *strings.Builder

	cfg Config
	svc *Service
}

// advancingClock moves forward one millisecond every time it is read.
//
// A constant clock would give every log line, from both rings, the same
// instant, and any assertion about the ORDER of those lines would hold whatever
// the code did with them. Moving forward is also closer to the real thing: no
// two of these events happen at the same moment on a real box.
type advancingClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *advancingClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(time.Millisecond)
	return c.t
}

// newWorld builds a service over a recorded machine.
func newWorld(t *testing.T, opts ...func(*world)) *world {
	t.Helper()
	dir := t.TempDir()
	w := &world{
		t:     t,
		dir:   dir,
		tl:    &timeline{},
		sys:   hotspot.NewRecorder(),
		eng:   nil,
		reach: &fakeReach{},
		logs:  &strings.Builder{},
	}
	clock := &advancingClock{t: time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)}
	w.clock = clock
	w.eng = &recordingEngine{tl: w.tl, now: clock.now}
	w.resolver = &fakeResolver{addr: netip.MustParseAddr("203.0.113.7")}
	w.runner = newRecordedMachine(fixtureIWList, fixtureIWRegGet)

	paths := hotspot.DefaultPaths()
	paths.HostapdConf = filepath.Join(dir, "hostapd.conf")
	paths.DnsmasqConf = filepath.Join(dir, "dnsmasq.conf")
	paths.HostapdPID = filepath.Join(dir, "hostapd.pid")
	paths.DnsmasqPID = filepath.Join(dir, "dnsmasq.pid")
	paths.LeaseFile = filepath.Join(dir, "dnsmasq.leases")
	paths.StateDir = dir

	w.cfg = Config{
		Runner:       tracedRunner{inner: w.runner, tl: w.tl},
		System:       tracedSystem{System: w.sys, tl: w.tl},
		HotspotPaths: paths,
		JournalPath:  filepath.Join(dir, "netcfg.journal"),
		TunName:      "xray0",
		SocksPort:    10808,
		LocalDNSPort: 5354,
		DNSPort:      53,
		PanelPort:    8088,
		TUNDisabled:  true,
		Engine:       w.eng,
		Resolver:     w.resolver,
		Reach:        w.reach,
		Logger:       slog.New(slog.NewTextHandler(w.logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Now:          clock.now,
	}
	// hostapd is what puts the network name on the interface, so the fake one
	// has to do that to the modelled machine or no readback of it can pass.
	hostapdRespondsForMachine(w)

	for _, o := range opts {
		o(w)
	}

	svc, err := New(w.cfg)
	if err != nil {
		t.Fatalf("building the service: %v", err)
	}
	w.svc = svc
	t.Cleanup(func() { _ = svc.Close() })
	return w
}

// newRecordedMachine answers the read-only detection commands and lets every
// mutating command succeed.
func newRecordedMachine(iwList, regGet string) *machine {
	r := netcfg.NewRecordingRunner()
	r.SetOutput("ip route show default", fixtureIPRouteDefault)
	r.SetOutput("ip -6 route show default", fixtureIPRoute6Default)
	r.SetOutput("ip -d link show", fixtureIPDetailLink)
	r.SetOutput("iw list", iwList)
	r.SetOutput("iw reg get", regGet)

	knobs := kernelKnobs()
	r.Fallback = func(c netcfg.Command) (netcfg.Result, error) {
		// The sysctl READ, whose argument list depends on which interfaces the
		// plan chose and so cannot be a fixed key.
		if c.Path == netcfg.BinSysctl && len(c.Args) > 0 && c.Args[0] == "-e" {
			var b strings.Builder
			for _, a := range c.Args {
				if v, ok := knobs[a]; ok {
					fmt.Fprintf(&b, "%s = %s\n", a, v)
				}
			}
			return netcfg.Result{Stdout: b.String()}, nil
		}
		// Everything else is a change, and on this machine every change works.
		return netcfg.Result{}, nil
	}

	m := &machine{RecordingRunner: r}
	m.seed(fixtureIPBriefAddr, fixtureIWDev, fixtureNmcliDeviceStatus)
	return m
}

// ---------------------------------------------------------------------------
// A machine that answers the reads this package's own commands CHANGE
// ---------------------------------------------------------------------------

// machine is the recorded Raspberry Pi, with state for the three reads whose
// answers this package's own commands change: the wireless interface list, the
// addresses on an interface, and what NetworkManager holds.
//
// # Why it has to keep state at all
//
// A double that answers "iw dev" from a fixed fixture agrees with the code by
// construction and is blind to the entire defect this change exists to catch.
// The interface takeover releases wlan0, strips its address and sets it to
// access point mode, and the whole question is whether the KERNEL then says
// so. A fixture that keeps reporting "type managed, ssid HomeNet, channel 10"
// however many commands ran can only prove that the readback refuses, never
// that it passes; a fixture that reported AP from the start could only prove
// the opposite. Neither is a test of the readback.
//
// # What it models, and how well each part is known
//
//   - "iw dev": rendered from state. SEEDED by parsing fixtureIWDev, so the
//     double and the fixture cannot drift; see
//     TestTheModelledMachineStartsAsTheFixtureDescribesIt.
//
//   - "ip -br addr" and "ip -br addr show dev <name>": rendered from state,
//     seeded the same way from fixtureIPBriefAddr.
//
//   - "nmcli -t -f DEVICE,STATE device status": rendered from state, seeded
//     from fixtureNmcliDeviceStatus.
//
//   - "iw dev <dev> set type __ap" makes the interface report type AP, not
//     "__ap". MEASURED on the target, recorded in
//     internal/netcfg/testdata/PROVENANCE.md, "The release sequence has been
//     run on the target": that command exits 0 and "iw info" then reports
//     "type AP". A double that echoed the argument back would fail every
//     readback for a reason the real box does not have.
//
//   - A freed and typed interface reports NO SSID and STILL REPORTS THE
//     CHANNEL the station link was on. MEASURED on the target on 2026-08-30 by
//     the coordinator, kernel 6.18.34, brcmfmac: after the release sequence and
//     with no hostapd running, "iw dev" printed "type AP" and
//     "channel 10 (2457 MHz)" for wlan0 and no ssid line at all. The driver
//     keeps reporting the old channel.
//
//     This is modelled rather than tidied away because it is the state the
//     product actually reaches, and a double that cleared the channel too
//     would let a predicate that reads a channel as "joined to a network" pass
//     every test while refusing every real box. See
//     TestAFreedInterfaceStillReportingItsOldChannelIsNotJoinedToAnything.
//
//   - Taking the link down clears the SSID for the same reason: on the target
//     the association ended across the release and did not come back until the
//     inverses ran (PROVENANCE.md, same page).
//
//   - hostapd gives the interface its SSID and channel. That is the real
//     causal chain: hostapd reads the configuration file this package wrote
//     and tells the kernel. See hostapdRespondsForMachine.
//
// # What it does not model
//
// Refusals. Every change succeeds unless a test registers an error with
// SetError, which is what the existing tests do for the driver's
// "Input/output error (-5)". netcfg.SimulatedKernel is the double that models
// refusal, and it belongs to that package's tests; this one models the reads
// that this package's start sequence has to be able to trust.
type machine struct {
	*netcfg.RecordingRunner

	mu        sync.Mutex
	wireless  []netcfg.WirelessIface
	links     []netcfg.Link
	nmState   map[string]string
	nmDevices []string
}

// seed fills the state from the fixture bytes, by parsing them with the same
// parsers the production code uses. A fixture this cannot parse is a fixture
// detection cannot parse either, so it panics rather than starting a test on a
// machine nobody can read.
func (m *machine) seed(brAddr, iwDev, nmcli string) {
	links, err := netcfg.ParseBriefAddr(brAddr)
	if err != nil {
		panic("harness: the ip -br addr fixture does not parse: " + err.Error())
	}
	ifaces, err := netcfg.ParseIwDev(iwDev)
	if err != nil {
		panic("harness: the iw dev fixture does not parse: " + err.Error())
	}
	m.links = links
	m.wireless = ifaces
	m.nmState = map[string]string{}
	for _, raw := range strings.Split(nmcli, "\n") {
		name, state, ok := strings.Cut(strings.TrimSpace(raw), ":")
		if !ok || name == "" {
			continue
		}
		m.nmDevices = append(m.nmDevices, name)
		m.nmState[name] = state
	}
}

// Run records the command, lets the recorder answer anything canned, applies
// the change to this machine's state, and answers the three reads from it.
//
// An explicit SetOutput or SetError always wins, so a test that wants a fixed
// answer or a failure still gets one.
func (m *machine) Run(ctx context.Context, c netcfg.Command) (netcfg.Result, error) {
	res, err := m.RecordingRunner.Run(ctx, c)
	if err != nil {
		return res, err
	}
	if _, canned := m.Responses[netcfg.RunnerKey(c)]; canned {
		return res, nil
	}
	m.observe(c)
	if out, ok := m.answer(c); ok {
		return netcfg.Result{Stdout: out}, nil
	}
	return res, nil
}

// observe applies a command that changed something.
func (m *machine) observe(c netcfg.Command) {
	a := c.Args
	m.mu.Lock()
	defer m.mu.Unlock()

	switch c.Path {
	case netcfg.BinNmcli:
		// nmcli device set <dev> managed no|yes
		if len(a) >= 5 && a[0] == "device" && a[1] == "set" && a[3] == "managed" {
			if a[4] == "no" {
				m.nmState[a[2]] = "unmanaged"
			} else {
				m.nmState[a[2]] = "connected"
			}
		}
	case netcfg.BinIw:
		switch {
		// iw dev <dev> set type <type>
		case len(a) >= 5 && a[0] == "dev" && a[2] == "set" && a[3] == "type":
			if w := m.wirelessLocked(a[1]); w != nil {
				w.Type = renderedIfaceType(a[4])
				// The name goes and the channel stays. Measured, not tidied;
				// see the type comment above.
				w.SSID = ""
			}
		// iw phy <phy> interface add <name> type <type>
		case len(a) >= 7 && a[0] == "phy" && a[2] == "interface" && a[3] == "add":
			m.wireless = append(m.wireless, netcfg.WirelessIface{
				Name: a[4], Phy: a[1], Type: renderedIfaceType(a[6]),
			})
			m.links = append(m.links, netcfg.Link{Name: a[4], State: "DOWN"})
		// iw dev <name> del
		case len(a) >= 3 && a[0] == "dev" && a[2] == "del":
			m.wireless = deleteWireless(m.wireless, a[1])
			m.links = deleteLink(m.links, a[1])
		}
	case netcfg.BinIP:
		for len(a) > 0 && strings.HasPrefix(a[0], "-") {
			a = a[1:]
		}
		switch {
		// ip address add|del <prefix> dev <name>
		case len(a) >= 5 && a[0] == "address" && (a[1] == "add" || a[1] == "del") && a[3] == "dev":
			p, err := netip.ParsePrefix(a[2])
			if err != nil {
				return
			}
			if l := m.linkLocked(a[4]); l != nil {
				if a[1] == "add" {
					l.Prefixes = append(l.Prefixes, p)
				} else {
					l.Prefixes = deletePrefix(l.Prefixes, p)
				}
			}
		// ip link set dev <name> up|down
		case len(a) >= 5 && a[0] == "link" && a[1] == "set" && a[2] == "dev":
			if l := m.linkLocked(a[3]); l != nil {
				l.State = strings.ToUpper(a[4])
			}
			if a[4] == "down" {
				if w := m.wirelessLocked(a[3]); w != nil {
					w.SSID = ""
				}
			}
		}
	}
}

// answer renders the three reads this machine owns.
func (m *machine) answer(c netcfg.Command) (string, bool) {
	a := c.Args
	m.mu.Lock()
	defer m.mu.Unlock()

	switch c.Path {
	case netcfg.BinIw:
		if len(a) == 1 && a[0] == "dev" {
			return renderIwDev(m.wireless), true
		}
		// "iw dev <name> link" is how netcfg asks whether an interface is
		// joined to a network. It used to infer that from the presence of a
		// channel, which called a freshly created access point a station and
		// stopped the appliance starting at all; the readback now asks the
		// question that has a direct answer.
		//
		// This double has to answer it, because netcfg treats output it
		// cannot read as an error rather than as "free". Reading an
		// unanswered probe as "the interface is free" is exactly the failure
		// that readback exists to catch, so the strictness is deliberate and
		// the double is what was incomplete.
		//
		// The two shapes are the real ones, captured from the box on
		// 2026-08-30: an associated station prints "Connected to <bssid> (on
		// <if>)" with a TAB-indented SSID line under it, and everything else,
		// including an access point with nothing serving on it, prints
		// exactly "Not connected.".
		if len(a) == 3 && a[0] == "dev" && a[2] == "link" {
			for _, w := range m.wireless {
				if w.Name != a[1] {
					continue
				}
				if w.SSID != "" && !w.IsAccessPoint() {
					return "Connected to 00:00:00:00:00:00 (on " + w.Name + ")\n\tSSID: " + w.SSID + "\n", true
				}
				return "Not connected.\n", true
			}
		}
	case netcfg.BinNmcli:
		if len(a) >= 2 && a[len(a)-2] == "device" && a[len(a)-1] == "status" {
			var b strings.Builder
			for _, n := range m.nmDevices {
				fmt.Fprintf(&b, "%s:%s\n", n, m.nmState[n])
			}
			return b.String(), true
		}
	case netcfg.BinIP:
		if len(a) >= 2 && a[0] == "-br" && a[1] == "addr" {
			if len(a) == 2 {
				return renderBriefAddr(m.links), true
			}
			if len(a) == 5 && a[2] == "show" && a[3] == "dev" {
				for _, l := range m.links {
					if l.Name == a[4] {
						return renderBriefAddr([]netcfg.Link{l}), true
					}
				}
				return "", true
			}
		}
	}
	return "", false
}

// startAccessPoint is what hostapd does to the kernel: it puts the network
// name and the channel on the interface it was configured for.
func (m *machine) startAccessPoint(iface, ssid string, channel int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w := m.wirelessLocked(iface); w != nil {
		w.Type = "AP"
		w.SSID = ssid
		w.Channel = channel
	}
}

// wirelessState returns one interface as this machine currently reports it.
func (m *machine) wirelessState(name string) (netcfg.WirelessIface, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w := m.wirelessLocked(name); w != nil {
		return *w, true
	}
	return netcfg.WirelessIface{}, false
}

// rejoin puts an interface back on another network, which is what happens when
// something outside this appliance takes the radio back.
func (m *machine) rejoin(name, ssid string, channel int, addr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w := m.wirelessLocked(name); w != nil {
		w.Type = "managed"
		w.SSID = ssid
		w.Channel = channel
	}
	if l := m.linkLocked(name); l != nil && addr != "" {
		if p, err := netip.ParsePrefix(addr); err == nil {
			l.Prefixes = append(l.Prefixes, p)
		}
	}
	m.nmState[name] = "connected"
}

func (m *machine) wirelessLocked(name string) *netcfg.WirelessIface {
	for i := range m.wireless {
		if m.wireless[i].Name == name {
			return &m.wireless[i]
		}
	}
	return nil
}

func (m *machine) linkLocked(name string) *netcfg.Link {
	for i := range m.links {
		if m.links[i].Name == name {
			return &m.links[i]
		}
	}
	return nil
}

// renderedIfaceType is what the kernel REPORTS after being asked for a type.
//
// "iw dev wlan0 set type __ap" is answered by "iw" reporting "type AP". The
// two spellings are not interchangeable and the difference is the whole of a
// readback that would otherwise never pass. MEASURED on the target and
// recorded in internal/netcfg/testdata/PROVENANCE.md.
func renderedIfaceType(arg string) string {
	if strings.EqualFold(arg, "__ap") {
		return "AP"
	}
	return arg
}

func renderIwDev(ifaces []netcfg.WirelessIface) string {
	var b strings.Builder
	phy := ""
	for _, w := range ifaces {
		if w.Phy != phy {
			phy = w.Phy
			fmt.Fprintf(&b, "%s\n", strings.Replace(phy, "phy", "phy#", 1))
		}
		fmt.Fprintf(&b, "\tInterface %s\n", w.Name)
		if w.MAC != "" {
			fmt.Fprintf(&b, "\t\taddr %s\n", w.MAC)
		}
		if w.SSID != "" {
			fmt.Fprintf(&b, "\t\tssid %s\n", w.SSID)
		}
		fmt.Fprintf(&b, "\t\ttype %s\n", w.Type)
		if w.Channel > 0 {
			fmt.Fprintf(&b, "\t\tchannel %d (%d MHz), width: 20 MHz\n", w.Channel, w.FreqMHz)
		}
	}
	return b.String()
}

func renderBriefAddr(links []netcfg.Link) string {
	var b strings.Builder
	for _, l := range links {
		fmt.Fprintf(&b, "%-16s %-14s ", l.Name, l.State)
		for _, p := range l.Prefixes {
			fmt.Fprintf(&b, "%s ", p)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func deleteWireless(in []netcfg.WirelessIface, name string) []netcfg.WirelessIface {
	out := in[:0]
	for _, w := range in {
		if w.Name != name {
			out = append(out, w)
		}
	}
	return out
}

func deleteLink(in []netcfg.Link, name string) []netcfg.Link {
	out := in[:0]
	for _, l := range in {
		if l.Name != name {
			out = append(out, l)
		}
	}
	return out
}

func deletePrefix(in []netip.Prefix, p netip.Prefix) []netip.Prefix {
	out := in[:0]
	for _, have := range in {
		if have != p {
			out = append(out, have)
		}
	}
	return out
}

// hostapdRespondsForMachine wires the fake hostapd to the modelled kernel.
//
// The real hostapd reads the configuration file this package wrote and tells
// the kernel the interface, the name and the channel. Modelling that chain,
// rather than setting the state directly from the test, is what makes a test
// that breaks the configuration break the readback too.
func hostapdRespondsForMachine(w *world) {
	base := hotspot.DefaultResponder
	bin := w.cfg.HotspotPaths.HostapdBinary
	w.sys.Responder = func(rec *hotspot.Recorder, name string, args []string) (hotspot.Result, error) {
		res, err := base(rec, name, args)
		if name != bin || err != nil {
			return res, err
		}
		conf := w.sys.Files[w.cfg.HotspotPaths.HostapdConf]
		iface, ssid, channel := hostapdConfValues(string(conf))
		if iface != "" {
			w.runner.startAccessPoint(iface, ssid, channel)
		}
		return res, nil
	}
}

// hostapdConfValues reads back the three settings that decide what appears on
// the air, from the file internal/hotspot rendered.
func hostapdConfValues(conf string) (iface, ssid string, channel int) {
	for _, line := range strings.Split(conf, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.HasPrefix(k, "#") {
			continue
		}
		switch k {
		case "interface":
			iface = v
		case "ssid":
			ssid = v
		case "channel":
			if n, err := strconv.Atoi(v); err == nil {
				channel = n
			}
		}
	}
	return iface, ssid, channel
}

// startRequest is a request the recorded machine can satisfy.
func startRequest(t *testing.T) panel.StartRequest {
	t.Helper()
	l, err := link.Parse(realityShareLink())
	if err != nil {
		t.Fatalf("parsing the share link: %v", err)
	}
	doc, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("building the config document: %v", err)
	}
	return panel.StartRequest{
		ConfigJSON: doc,
		Hotspot: panel.HotspotSpec{
			SSID:       fakeSSID,
			Passphrase: fakeHotspotPassphrase,
		},
		Network: panel.NetworkSpec{
			DNSMode:      state.DNSModeTunnel,
			OnTunnelDown: state.OnTunnelDownBlock,
			ClientIPv6:   state.ClientIPv6Block,
		},
	}
}

// mutatingCommands is every command the recorder was asked to run that changes
// something. The read-only detection commands are excluded by name.
func (w *world) mutatingCommands() []string {
	var out []string
	for _, c := range w.runner.Commands() {
		if isReadOnly(c) {
			continue
		}
		out = append(out, netcfg.RunnerKey(c))
	}
	return out
}

func isReadOnly(c netcfg.Command) bool {
	switch c.Path {
	case netcfg.BinIw:
		// "iw dev", "iw list" and "iw reg get" read; "iw phy ... interface add"
		// and "iw dev ... del" change.
		if len(c.Args) >= 2 && (c.Args[0] == "phy" || c.Args[1] == "del") {
			return false
		}
		return true
	case netcfg.BinSysctl:
		return len(c.Args) > 0 && c.Args[0] == "-e"
	case netcfg.BinIP:
		for _, a := range c.Args {
			switch a {
			case "add", "del", "set":
				return false
			}
		}
		return true
	case netcfg.BinNmcli:
		// "nmcli -t -f DEVICE,STATE device status" reads. "nmcli device set
		// <dev> managed no" is the release, and is the change.
		for _, a := range c.Args {
			if a == "set" {
				return false
			}
		}
		return true
	case netcfg.BinNft:
		return false
	}
	return false
}

// freePort reserves and releases a port, so a test that starts the real engine
// does not fight another test for a fixed one.
func freePort(t *testing.T) uint16 {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	defer ln.Close()
	return uint16(ln.Addr().(*net.TCPAddr).Port)
}
