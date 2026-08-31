// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The tests in this file are CHARACTERISATION tests. Every behaviour asserted
// here was already correct on 2026-08-30; what was missing was anything that
// would notice if it stopped. They cover the branches the coverage profile
// showed unreached.
//
// The rule this package's messages follow, and which most of these tests
// assert alongside the behaviour, is from docs/2026-08-29-design.md section
// 5.2: name the thing that is wrong and the action that fixes it, in the words
// the user would use. Several of the branches below exist ONLY to produce such
// a message, so a test that checked the error was non-nil would not be testing
// the thing the branch is for.

// --- APConfig.Validate ------------------------------------------------------

func TestAPConfigRejectsABadNetworkName(t *testing.T) {
	tests := []struct {
		name string
		ssid string
		want string
	}{
		{"empty", "", "empty"},
		// A lone continuation byte is not valid UTF-8. hostapd writes the SSID
		// into a config file and the panel renders it, so invalid text has to
		// be refused rather than passed on.
		{"not valid text", "Caspian\xff", "not valid text"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testAP()
			cfg.SSID = tc.ssid
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate accepted an SSID that is %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the message does not say what is wrong: %v", err)
			}
		})
	}
}

// TestAPConfigRejectsABadControlDirectory covers the ControlDir branch, which
// is skipped entirely when the field is empty.
func TestAPConfigRejectsABadControlDirectory(t *testing.T) {
	cfg := testAP()
	cfg.ControlDir = "run/hostapd" // relative
	if err := cfg.Validate(); err == nil {
		t.Error("Validate accepted a relative hostapd control directory")
	}

	// A newline would end the ctrl_interface line and let the rest be read as
	// further hostapd directives.
	cfg.ControlDir = "/run/hostapd\nssid=evil"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate accepted a control directory carrying a newline")
	}
	if !strings.Contains(err.Error(), "control directory") {
		t.Errorf("the message does not name the field: %v", err)
	}

	// Empty is legal and means the default; RenderHostapd fills it in.
	cfg.ControlDir = ""
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate refused an empty control directory, which means the default: %v", err)
	}
}

// TestConfigTokensAreBounded covers the two branches of validConfigToken that
// nothing else reaches: the empty value and the over-long one.
//
// The length bound is not cosmetic. These values are written into a file a
// root process reads, and the interface name also reaches the kernel, so an
// unbounded one is a value nobody has checked reaching two places that matter.
func TestConfigTokensAreBounded(t *testing.T) {
	cfg := testAP()
	cfg.Interface = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate accepted an empty interface name")
	}
	if !strings.Contains(err.Error(), "no interface was given") {
		t.Errorf("the message does not say the interface is missing: %v", err)
	}

	cfg.Interface = strings.Repeat("w", 65)
	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate accepted a 65-character interface name")
	}
	if !strings.Contains(err.Error(), "too long") {
		t.Errorf("the message does not say the name is too long: %v", err)
	}

	// 64 is the boundary and is accepted, so this test fails if the bound is
	// moved rather than merely if it is removed.
	cfg.Interface = strings.Repeat("w", 64)
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate refused a 64-character interface name: %v", err)
	}
}

// TestEnsurePassphraseRefusesABadSuppliedOne covers the validation branch of
// EnsurePassphrase. Generating a replacement for a passphrase the caller
// supplied and got wrong would be worse than refusing: the user would be shown
// a password they did not choose and told nothing.
func TestEnsurePassphraseRefusesABadSuppliedOne(t *testing.T) {
	cfg := testAP()
	cfg.Passphrase = "short" // below the WPA2 minimum

	out, generated, err := EnsurePassphrase(cfg)
	if err == nil {
		t.Fatal("EnsurePassphrase accepted a passphrase that is too short")
	}
	if generated {
		t.Error("EnsurePassphrase reported that it generated a passphrase after refusing one")
	}
	if out.Passphrase != "" {
		t.Error("EnsurePassphrase returned a configuration carrying a passphrase after an error")
	}
}

// --- the band switches ------------------------------------------------------

// TestAnUnknownBandIsRefusedByBothSwitches covers the default branch of
// validChannel and of hwMode.
//
// Both exist for the same reason, and it is a forward-looking one: Band is a
// string type, so a future 6 GHz constant added to one switch and not the
// other would otherwise render hw_mode as the empty string and hostapd would
// fail with a message about the driver. The two switches are tested together
// because the second is unreachable through RenderHostapd, which validates
// the channel first and so never reaches hwMode with an unknown band.
func TestAnUnknownBandIsRefusedByBothSwitches(t *testing.T) {
	const bogus Band = "6"

	err := validChannel(bogus, 36)
	if err == nil {
		t.Fatal("validChannel accepted an unknown band")
	}
	if !strings.Contains(err.Error(), "unknown radio band") {
		t.Errorf("the message does not name the problem: %v", err)
	}

	cfg := testAP()
	cfg.Band = bogus
	if _, err := cfg.hwMode(); err == nil {
		t.Error("hwMode accepted an unknown band and would have rendered an empty hw_mode")
	}

	// Through the public entry point the channel check is what fires first,
	// and that is deliberate: it is the message that names a value the user
	// chose.
	if _, err := RenderHostapd(cfg); err == nil {
		t.Error("RenderHostapd rendered a configuration for an unknown band")
	}

	// The two real bands still work.
	for _, b := range []Band{Band2GHz, Band5GHz} {
		c := testAP()
		c.Band = b
		if b == Band5GHz {
			c.Channel = 36
		}
		if _, err := c.hwMode(); err != nil {
			t.Errorf("hwMode refused the %s band: %v", b, err)
		}
	}
}

// --- RenderHostapd's default control directory ------------------------------

// TestRenderHostapdFillsInTheDefaultControlDirectory covers the ctrlDir
// default.
//
// It matters because the control socket is how apBeaconing asks hostapd
// whether the access point is actually broadcasting. A rendered file with an
// empty ctrl_interface produces a hostapd that runs, answers nothing, and is
// reported as not beaconing forever.
func TestRenderHostapdFillsInTheDefaultControlDirectory(t *testing.T) {
	cfg := testAP()
	cfg.ControlDir = ""

	got, err := RenderHostapd(cfg)
	if err != nil {
		t.Fatalf("RenderHostapd: %v", err)
	}

	want := DefaultPaths().HostapdControlDir
	if !strings.Contains(got, "ctrl_interface="+want) {
		t.Errorf("the rendered file does not carry the default control directory %q:\n%s", want, got)
	}

	// An explicit value is still honoured.
	cfg.ControlDir = "/run/caspian/hostapd-ctrl"
	got, err = RenderHostapd(cfg)
	if err != nil {
		t.Fatalf("RenderHostapd: %v", err)
	}
	if !strings.Contains(got, "ctrl_interface=/run/caspian/hostapd-ctrl") {
		t.Errorf("an explicit control directory was not used:\n%s", got)
	}
}

// --- DNSConfig.Validate -----------------------------------------------------

func TestDNSConfigRejectsABadSubnet(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DNSConfig)
		want   string
	}{
		{"no subnet at all", func(c *DNSConfig) { c.Subnet = netip.Prefix{} }, "no hotspot subnet"},
		{"an IPv6 subnet", func(c *DNSConfig) {
			c.Subnet = netip.MustParsePrefix("fd00::/64")
		}, "IPv4 network"},
		{"a /31 with no usable addresses", func(c *DNSConfig) {
			c.Subnet = netip.MustParsePrefix("192.168.66.0/31")
		}, "no usable range"},
		{"a /7 that is absurdly large", func(c *DNSConfig) {
			c.Subnet = netip.MustParsePrefix("10.0.0.0/7")
		}, "no usable range"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := testDNS()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the message does not say what is wrong: %v", err)
			}
		})
	}
}

// TestDNSConfigRejectsANonIPv4Address covers the address-family branch of the
// gateway and range check. The loop is written over a slice so the message is
// deterministic, and this test pins that too: it asserts WHICH field is named.
func TestDNSConfigRejectsANonIPv4Address(t *testing.T) {
	tests := []struct {
		name  string
		set   func(*DNSConfig, netip.Addr)
		field string
	}{
		{"gateway", func(c *DNSConfig, a netip.Addr) { c.Gateway = a }, "gateway address"},
		{"range start", func(c *DNSConfig, a netip.Addr) { c.RangeStart = a }, "first DHCP address"},
		{"range end", func(c *DNSConfig, a netip.Addr) { c.RangeEnd = a }, "last DHCP address"},
	}
	for _, tc := range tests {
		t.Run(tc.name+" unset", func(t *testing.T) {
			c := testDNS()
			tc.set(&c, netip.Addr{})
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate accepted an unset %s", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("the message names the wrong field: %v", err)
			}
		})
		t.Run(tc.name+" is IPv6", func(t *testing.T) {
			c := testDNS()
			tc.set(&c, netip.MustParseAddr("fd00::1"))
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate accepted an IPv6 %s", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("the message names the wrong field: %v", err)
			}
		})
	}
}

// TestDNSConfigRejectsAnUnsetResolver covers the upstream validity branch.
//
// The zero AddrPort and a valid address with port 0 are both "nobody set
// this". Rendering either produces a dnsmasq that forwards to nothing, and the
// hotspot then resolves no names at all while everything else looks healthy.
func TestDNSConfigRejectsAnUnsetResolver(t *testing.T) {
	for _, tc := range []struct {
		name string
		up   netip.AddrPort
	}{
		{"unset", netip.AddrPort{}},
		{"port zero", netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testDNS()
			c.Upstream = tc.up
			err := c.Validate()
			if err == nil {
				t.Fatal("Validate accepted a resolver address that is not set")
			}
			if !strings.Contains(err.Error(), "no local resolver address") {
				t.Errorf("the message does not say what is missing: %v", err)
			}
		})
	}
}

// TestDNSConfigRejectsANegativeCacheSize covers the cache-size branch.
func TestDNSConfigRejectsANegativeCacheSize(t *testing.T) {
	c := testDNS()
	c.CacheSize = -1
	if err := c.Validate(); err == nil {
		t.Error("Validate accepted a negative DNS cache size")
	}
}

// TestCacheSizeZeroIsWrittenOutExplicitly covers the cache-size=0 branch of
// the renderer.
//
// Zero has to be EMITTED rather than omitted. dnsmasq's built-in default is
// 150 entries, so leaving the directive out when the operator asked for no
// cache would silently give them a cache.
func TestCacheSizeZeroIsWrittenOutExplicitly(t *testing.T) {
	c := testDNS()
	c.CacheSize = 0

	got, err := RenderDnsmasq(c)
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	if !containsDirective(got, "cache-size=0") {
		t.Errorf("a cache size of 0 was not written out, so dnsmasq would apply its own default:\n%s", got)
	}

	// And a non-zero size is written as itself.
	c.CacheSize = 1000
	got, err = RenderDnsmasq(c)
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	if !containsDirective(got, "cache-size=1000") {
		t.Errorf("a cache size of 1000 was not written out:\n%s", got)
	}
}

// --- leases -----------------------------------------------------------------

// TestDisplayNameWithNothingToShow covers the last fallback of DisplayName.
//
// The first two branches are covered elsewhere. This one is what the panel
// shows for a lease that has neither a hostname nor a usable address, and it
// has to be a phrase rather than an empty string, because an empty row in the
// device list reads as a rendering bug.
func TestDisplayNameWithNothingToShow(t *testing.T) {
	var l Lease
	if got := l.DisplayName(); got != "unknown device" {
		t.Errorf("DisplayName of an empty lease = %q, want \"unknown device\"", got)
	}
}

// errReader fails on the first read, standing in for a lease file on failing
// storage.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("the medium reported a read error") }

// TestParseLeasesReportsAReadFailure covers the scanner error branch.
//
// It is the difference between "nobody is connected" and "the disk is
// failing". ParseLeases returns err only for a read failure and never for file
// content, so this is the one path that produces one.
func TestParseLeasesReportsAReadFailure(t *testing.T) {
	leases, malformed, err := ParseLeases(errReader{})
	if err == nil {
		t.Fatal("a failing reader was reported as an empty lease file")
	}
	if !strings.Contains(err.Error(), "could not read the lease file") {
		t.Errorf("the error does not say what failed: %v", err)
	}
	if len(leases) != 0 || malformed != 0 {
		t.Errorf("a failing reader produced %d leases and %d malformed lines", len(leases), malformed)
	}

	// A reader that returns EOF immediately is an empty file, not an error.
	if _, _, err := ParseLeases(strings.NewReader("")); err != nil {
		t.Errorf("an empty lease file was reported as a read failure: %v", err)
	}
	var _ io.Reader = errReader{}
}

// --- radio ------------------------------------------------------------------

// TestARadioThatCanHostNoAPIsRefused covers the MaxAPs branch, which is
// separate from SupportsAP: an adapter can report AP capability and a maximum
// of zero concurrent access points.
func TestARadioThatCanHostNoAPIsRefused(t *testing.T) {
	rc := RadioConstraint{SupportsAP: true, MaxAPs: 0, MaxChannels: 1}
	err := rc.Check(testAP())
	if err == nil {
		t.Fatal("a radio reporting it can host no access point was accepted")
	}
	// docs/2026-08-29-design.md section 5.2: the message tells the user what
	// to do, in their words.
	if !strings.Contains(err.Error(), "USB WiFi adapter") {
		t.Errorf("the message does not tell the user what to do: %v", err)
	}
	for _, jargon := range []string{"phy", "nl80211", "MaxAPs"} {
		if strings.Contains(err.Error(), jargon) {
			t.Errorf("the message contains jargon %q: %v", jargon, err)
		}
	}
}

// TestParseRfkillHeaderNeedsBothColons covers the second Cut in
// parseRfkillHeader, and the "no current device" continue in parseRfkillList.
func TestParseRfkillHeaderNeedsBothColons(t *testing.T) {
	// A line with an index and no type: not a header, so no device starts,
	// and the indented lines that follow have no device to attach to.
	devs := parseRfkillList("0: phy0\n\tSoft blocked: yes\n\tHard blocked: no\n")
	if len(devs) != 0 {
		t.Errorf("a malformed header started a device: %+v", devs)
	}

	// Property lines arriving before any header are ignored rather than
	// panicking on a nil current device.
	devs = parseRfkillList("\tSoft blocked: yes\n\tHard blocked: no\n0: phy0: Wireless LAN\n\tSoft blocked: no\n")
	if len(devs) != 1 {
		t.Fatalf("parsed %d devices, want 1: %+v", len(devs), devs)
	}
	if devs[0].SoftBlocked {
		t.Error("a property line from before the header was attached to the device")
	}

	// A first field that is not a number is not a header either.
	if got := parseRfkillList("phy0: Wireless LAN\n"); len(got) != 0 {
		t.Errorf("a header with no index started a device: %+v", got)
	}
}

// --- Signal -----------------------------------------------------------------

// TestSignalString covers the String method, which is not decoration: it is
// interpolated into "could not send %s to process %d", the message an operator
// sees when a stop fails.
func TestSignalString(t *testing.T) {
	if got := SignalTerm.String(); got != "TERM" {
		t.Errorf("SignalTerm.String() = %q, want TERM", got)
	}
	if got := SignalKill.String(); got != "KILL" {
		t.Errorf("SignalKill.String() = %q, want KILL", got)
	}
	if got := Signal(99).String(); got != "unknown" {
		t.Errorf("Signal(99).String() = %q, want unknown", got)
	}
}

// --- explainFailure ---------------------------------------------------------

// TestExplainFailureRfkillBranch covers the rfkill case, which needs BOTH
// "rfkill" and "block" in the text and so is not reached by either word alone.
func TestExplainFailureRfkillBranch(t *testing.T) {
	got := explainFailure(unitAP, 1, "nl80211: Could not configure driver mode\nrfkill: WLAN soft blocked", "")
	if !strings.Contains(got, "switched off") {
		t.Errorf("the rfkill failure was not explained as the adapter being switched off: %q", got)
	}
	if strings.Contains(got, "rfkill") {
		t.Errorf("the message uses the word rfkill: %q", got)
	}

	// "rfkill" without "block" must NOT take this branch.
	other := explainFailure(unitAP, 1, "rfkill: could not open control device", "")
	if strings.Contains(other, "switched off") {
		t.Errorf("a message mentioning rfkill but no block was explained as a block: %q", other)
	}
}

// TestExplainFailureFallsBackToStdout covers the stdout fallback and the
// no-explanation-at-all branch.
//
// The design rule is that the message never invents a cause. These two
// branches are where that rule is actually implemented: when nothing matched,
// the text is handed over as-is or the failure is reported as unexplained.
func TestExplainFailureFallsBackToStdout(t *testing.T) {
	// Nothing on stderr, something on stdout: the stdout text is used.
	got := explainFailure(unitDHCP, 3, "", "something unrecognised happened\nand then more")
	if !strings.Contains(got, "something unrecognised happened") {
		t.Errorf("the stdout text was not used when stderr was empty: %q", got)
	}
	// Only the first line, so a multi-line dump does not reach the panel.
	if strings.Contains(got, "and then more") {
		t.Errorf("more than the first line was included: %q", got)
	}
	if !strings.Contains(got, "does not recognise the reason") {
		t.Errorf("the message claims to know the cause: %q", got)
	}

	// Nothing on either stream: the exit code is all there is.
	silent := explainFailure(unitDHCP, 3, "   ", "  \n ")
	if !strings.Contains(silent, "no explanation") {
		t.Errorf("a silent failure was not reported as unexplained: %q", silent)
	}
	if !strings.Contains(silent, "code 3") {
		t.Errorf("the exit code is not in the message: %q", silent)
	}
	if !strings.Contains(silent, unitDHCP) {
		t.Errorf("the unit is not named: %q", silent)
	}
}

// TestFirstLineWithNoNewline covers the branch of firstLine where there is
// nothing to cut.
func TestFirstLineWithNoNewline(t *testing.T) {
	if got := firstLine("one line only"); got != "one line only" {
		t.Errorf("firstLine of a single line = %q", got)
	}
	if got := firstLine("first\nsecond"); got != "first" {
		t.Errorf("firstLine of two lines = %q, want \"first\"", got)
	}
	if got := firstLine("  padded  \nsecond"); got != "padded" {
		t.Errorf("firstLine does not trim: %q", got)
	}
}

// --- NewPlan error propagation ----------------------------------------------

// TestNewPlanStopsAtTheFirstProblem covers the four error returns in NewPlan
// that nothing else reached, and pins the ORDER they fire in.
//
// The order is a product decision recorded in NewPlan's own comment: the radio
// check comes after the config check so that a user with two problems is told
// about the one they chose rather than about a channel they never set.
func TestNewPlanStopsAtTheFirstProblem(t *testing.T) {
	goodRadio := RadioConstraint{SupportsAP: true, MaxAPs: 1, MaxChannels: 1, ClientChannel: 10}

	t.Run("a bad passphrase", func(t *testing.T) {
		ap := testAP()
		ap.Passphrase = "short"
		if _, err := NewPlan(ap, testDNS(), goodRadio); err == nil {
			t.Error("NewPlan accepted a passphrase below the WPA2 minimum")
		}
	})

	t.Run("a bad access point configuration", func(t *testing.T) {
		ap := testAP()
		ap.CountryCode = ""
		_, err := NewPlan(ap, testDNS(), goodRadio)
		if err == nil {
			t.Fatal("NewPlan accepted a configuration with no country")
		}
		if !strings.Contains(err.Error(), "country") {
			t.Errorf("the message does not name the missing field: %v", err)
		}
	})

	t.Run("the config is checked before the radio", func(t *testing.T) {
		// Two problems at once: no country AND a channel the radio cannot use.
		ap := testAP()
		ap.CountryCode = ""
		ap.Channel = 6
		_, err := NewPlan(ap, testDNS(), goodRadio)
		if err == nil {
			t.Fatal("NewPlan accepted a configuration with two problems")
		}
		if !strings.Contains(err.Error(), "country") {
			t.Errorf("the radio complaint was reported before the one the user can act on: %v", err)
		}
	})

	t.Run("a bad DHCP configuration", func(t *testing.T) {
		dns := testDNS()
		dns.LeaseTime = time.Second
		_, err := NewPlan(testAP(), dns, goodRadio)
		if err == nil {
			t.Fatal("NewPlan accepted a lease time below the dnsmasq minimum")
		}
		if !strings.Contains(err.Error(), "minimum") {
			t.Errorf("the message does not say what the limit is: %v", err)
		}
	})

	t.Run("a radio that cannot run it", func(t *testing.T) {
		ap := testAP()
		ap.Channel = 6
		if _, err := NewPlan(ap, testDNS(), goodRadio); err == nil {
			t.Error("NewPlan accepted a channel the radio is pinned away from")
		}
	})
}

// --- the Recorder itself ----------------------------------------------------
//
// The Recorder ships in the package rather than in a _test file, so that the
// panel and the engine can drive a Supervisor against the same double. That
// makes a bug in it a source of FALSE PASSES in three packages, which is why
// it is tested rather than trusted.

// TestRecorderRefusesToAnswerAnUnknownCommand is the most important of these.
// A responder that returned success for anything would let a supervisor that
// runs the wrong program pass every test in this package.
func TestRecorderRefusesToAnswerAnUnknownCommand(t *testing.T) {
	rec := NewRecorder()
	_, err := rec.Run(context.Background(), "/usr/sbin/iptables", "-L")
	if err == nil {
		t.Fatal("the recorder invented a successful answer for a command it does not emulate")
	}
	if !strings.Contains(err.Error(), "iptables") {
		t.Errorf("the error does not name the command: %v", err)
	}
	// It still recorded the attempt, so a test can assert on what was tried.
	if got := rec.CountCalls("/usr/sbin/iptables"); got != 1 {
		t.Errorf("the unanswered call was recorded %d times, want 1", got)
	}
}

func TestRecorderHonoursACancelledContext(t *testing.T) {
	rec := NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := rec.Run(ctx, "/usr/sbin/rfkill", "list"); !errors.Is(err, context.Canceled) {
		t.Errorf("Run with a cancelled context returned %v, want context.Canceled", err)
	}
	// A cancelled call is not a call: recording it would make the argument
	// assertions in the supervisor tests wrong.
	if got := rec.CountCalls("/usr/sbin/rfkill"); got != 0 {
		t.Errorf("a cancelled call was recorded %d times, want 0", got)
	}
}

func TestRecorderRfkillAnswersOnlyList(t *testing.T) {
	rec := NewRecorder()

	res, err := rec.Run(context.Background(), "/usr/sbin/rfkill", "list")
	if err != nil {
		t.Fatalf("rfkill list: %v", err)
	}
	if !strings.Contains(res.Stdout, "phy0") {
		t.Errorf("rfkill list produced no device: %q", res.Stdout)
	}

	// Any other rfkill subcommand, "unblock" in particular, succeeds with no
	// output. That is what makes the supervisor's read-back meaningful.
	res, err = rec.Run(context.Background(), "/usr/sbin/rfkill", "unblock", "wifi")
	if err != nil {
		t.Fatalf("rfkill unblock: %v", err)
	}
	if res.Stdout != "" || res.ExitCode != 0 {
		t.Errorf("rfkill unblock produced %+v, want an empty success", res)
	}

	// And with no arguments at all.
	if _, err := rec.Run(context.Background(), "/usr/sbin/rfkill"); err != nil {
		t.Errorf("bare rfkill: %v", err)
	}
}

// TestCallStringRendersACommandLine covers Call.String for a call with no
// arguments. It is the rendering the supervisor tests' failure messages use,
// so a wrong one costs debugging time on an already failing test.
func TestCallStringRendersACommandLine(t *testing.T) {
	if got := (Call{Name: "/usr/bin/pgrep"}).String(); got != "/usr/bin/pgrep" {
		t.Errorf("a call with no arguments rendered as %q", got)
	}
	if got := (Call{Name: "/usr/bin/pgrep", Args: []string{"-f", "/run/x.conf"}}).String(); got != "/usr/bin/pgrep -f /run/x.conf" {
		t.Errorf("a call with arguments rendered as %q", got)
	}
}

// TestArgumentLookupsMissTheirTarget covers the not-found returns of flagValue
// and optValue. A silent wrong answer here would make DefaultResponder write a
// pid file to the wrong path, and every supervisor test would then be testing
// a machine that does not resemble the real one.
func TestArgumentLookupsMissTheirTarget(t *testing.T) {
	if v, ok := flagValue([]string{"-B", "/run/hostapd.conf"}, "-P"); ok {
		t.Errorf("flagValue found %q for a flag that is not there", v)
	}
	// A flag in the last position has no value after it.
	if v, ok := flagValue([]string{"-B", "-P"}, "-P"); ok {
		t.Errorf("flagValue returned %q for a trailing flag with no value", v)
	}
	if v, ok := flagValue([]string{"-B", "-P", "/run/x.pid"}, "-P"); !ok || v != "/run/x.pid" {
		t.Errorf("flagValue = %q, %v; want /run/x.pid, true", v, ok)
	}

	if v, ok := optValue([]string{"--conf-file=/run/x.conf"}, "--pid-file="); ok {
		t.Errorf("optValue found %q for an option that is not there", v)
	}
	if v, ok := optValue([]string{"--pid-file=/run/x.pid"}, "--pid-file="); !ok || v != "/run/x.pid" {
		t.Errorf("optValue = %q, %v; want /run/x.pid, true", v, ok)
	}
}

// --- the real System --------------------------------------------------------
//
// These exercise execSystem, the implementation that touches real files and
// real processes. They run on any unix, which is what NewExecSystem exists for.

func TestExecSystemWriteFileFailures(t *testing.T) {
	sys := NewExecSystem()

	t.Run("a parent that is a regular file", func(t *testing.T) {
		dir := t.TempDir()
		blocker := filepath.Join(dir, "blocker")
		if err := os.WriteFile(blocker, []byte("in the way"), 0o600); err != nil {
			t.Fatalf("setup: %v", err)
		}
		err := sys.WriteFile(filepath.Join(blocker, "sub", "hostapd.conf"), []byte("x"), 0o600)
		if err == nil {
			t.Fatal("WriteFile created a directory underneath a regular file")
		}
		if !strings.Contains(err.Error(), "could not create") {
			t.Errorf("the error does not say the directory creation failed: %v", err)
		}
	})

	t.Run("a directory that cannot be written to", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root, which can write into a directory with no write bit")
		}
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

		err := sys.WriteFile(filepath.Join(dir, "hostapd.conf"), []byte("x"), 0o600)
		if err == nil {
			t.Fatal("WriteFile wrote into a directory with no write permission")
		}
		if !strings.Contains(err.Error(), "could not write") {
			t.Errorf("the error does not say the write failed: %v", err)
		}
	})
}

// TestExecSystemRemoveReportsARealFailure covers the Remove error branch.
//
// Removing something that is not there is success, and that is load-bearing:
// the supervisor removes pid files unconditionally. A real failure, here a
// directory that is not empty, must still be reported.
func TestExecSystemRemoveReportsARealFailure(t *testing.T) {
	sys := NewExecSystem()
	dir := t.TempDir()

	// Absent is success.
	if err := sys.Remove(filepath.Join(dir, "not-there.pid")); err != nil {
		t.Errorf("removing a file that is not there was an error: %v", err)
	}

	// A non-empty directory cannot be removed.
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "child"), []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	err := sys.Remove(sub)
	if err == nil {
		t.Fatal("Remove reported success for a non-empty directory")
	}
	if !strings.Contains(err.Error(), "could not remove") {
		t.Errorf("the error does not say the removal failed: %v", err)
	}
}

// TestExecSystemProcessAliveOnADeadAndAForeignProcess covers the ESRCH and
// EPERM branches.
//
// EPERM is the interesting one, and it is why this cannot be a simple "is the
// pid in the table" check: a process that exists and belongs to somebody else
// answers EPERM, and reading that as "not running" would have the supervisor
// start a second hostapd while the first still holds the radio.
func TestExecSystemProcessAliveOnADeadAndAForeignProcess(t *testing.T) {
	sys := NewExecSystem()

	// Our own process is alive.
	alive, err := sys.ProcessAlive(os.Getpid())
	if err != nil {
		t.Fatalf("ProcessAlive on ourselves: %v", err)
	}
	if !alive {
		t.Error("ProcessAlive reported this process as not running")
	}

	// A pid that cannot exist.
	alive, err = sys.ProcessAlive(-1)
	if err != nil {
		t.Fatalf("ProcessAlive on a negative pid: %v", err)
	}
	if alive {
		t.Error("ProcessAlive reported a negative pid as running")
	}

	// pid 1 belongs to root and is always there. As a non-root user the kernel
	// answers EPERM, which means alive.
	if os.Geteuid() != 0 {
		alive, err = sys.ProcessAlive(1)
		if err != nil {
			t.Fatalf("ProcessAlive on pid 1: %v", err)
		}
		if !alive {
			t.Error("a process owned by another user was reported as not running; " +
				"the supervisor would start a second daemon beside it")
		}
	}
}

// TestExecSystemSignalProcess covers the signal mapping, the ESRCH tolerance
// and the refusal to signal a nonsense pid.
func TestExecSystemSignalProcess(t *testing.T) {
	sys := NewExecSystem()

	// This guard is load-bearing in a way that is easy to underrate. Measured
	// on 2026-08-30 by deleting it: syscall.Kill(0, SIGTERM) signals the
	// CALLER'S ENTIRE PROCESS GROUP, so the mutated test binary terminated
	// itself with SIGTERM (exit status -15) and took the mutation harness and
	// its shell with it on the first two attempts. On the appliance the caller
	// is the privileged service, and a pid file that read as 0 would have it
	// signal itself and everything sharing its group.
	t.Run("a pid that is not a pid", func(t *testing.T) {
		for _, pid := range []int{0, -1} {
			if err := sys.SignalProcess(pid, SignalTerm); err == nil {
				t.Errorf("SignalProcess accepted pid %d; pid 0 signals the whole process group", pid)
			}
		}
	})

	t.Run("an unknown signal", func(t *testing.T) {
		err := sys.SignalProcess(os.Getpid(), Signal(99))
		if err == nil {
			t.Fatal("SignalProcess accepted a signal it does not know")
		}
		if !strings.Contains(err.Error(), "unknown signal") {
			t.Errorf("the error does not say the signal is unknown: %v", err)
		}
	})

	t.Run("a process that has already gone", func(t *testing.T) {
		// A pid that is almost certainly free. If it happens to exist the
		// call still succeeds, so this cannot produce a false failure; it is
		// the error path that is being checked and ESRCH is the common answer.
		if err := sys.SignalProcess(0x7FFFFFF0, SignalTerm); err != nil {
			t.Errorf("signalling a process that is not there was an error: %v", err)
		}
	})
}

// TestExecSystemSleepReturnsWhenTheTimerFires covers the timer branch. The
// cancellation branch is covered by an existing test; this is the other half,
// and without it a Sleep that never returned would still pass.
func TestExecSystemSleepReturnsWhenTheTimerFires(t *testing.T) {
	sys := NewExecSystem()
	start := time.Now()
	if err := sys.Sleep(context.Background(), 5*time.Millisecond); err != nil {
		t.Fatalf("Sleep: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 5*time.Millisecond {
		t.Errorf("Sleep returned after %s, which is less than the 5ms it was asked for", elapsed)
	}
}

// TestExecSystemRunSeparatesTheTwoKindsOfFailure is the property the whole
// System interface is written around, checked here on the real implementation
// for a command that does not exist at all.
func TestExecSystemRunCannotFindTheProgram(t *testing.T) {
	sys := NewExecSystem()
	_, err := sys.Run(context.Background(), filepath.Join(t.TempDir(), "no-such-program"))
	if err == nil {
		t.Fatal("Run reported success for a program that does not exist")
	}
	if !strings.Contains(err.Error(), "could not run") {
		t.Errorf("the error does not say the program could not be run: %v", err)
	}
}

// TestExecSystemSignalsARealProcess covers the SIGKILL mapping and the
// successful return of SignalProcess, against a real child process.
//
// The Recorder cannot cover these: it records the signal and marks the pid
// dead without any kernel involved, so the mapping from this package's Signal
// to syscall.Signal is exactly the part it cannot check.
func TestExecSystemSignalsARealProcess(t *testing.T) {
	sys := NewExecSystem()

	for _, tc := range []struct {
		name string
		sig  Signal
	}{
		{"TERM", SignalTerm},
		{"KILL", SignalKill},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("/bin/sh", "-c", "sleep 30")
			if err := cmd.Start(); err != nil {
				t.Fatalf("could not start a child process: %v", err)
			}
			pid := cmd.Process.Pid

			alive, err := sys.ProcessAlive(pid)
			if err != nil {
				t.Fatalf("ProcessAlive: %v", err)
			}
			if !alive {
				t.Fatal("the child process was not alive right after starting it")
			}

			if err := sys.SignalProcess(pid, tc.sig); err != nil {
				t.Fatalf("SignalProcess(%v): %v", tc.sig, err)
			}

			// The signal has to actually STOP it, and promptly. The child
			// sleeps for 30 seconds, so a signal that was mapped to something
			// harmless (signal 0, say) would let this test pass simply by
			// waiting the child out. Verified by mutation on 2026-08-30:
			// mapping SignalKill to syscall.Signal(0) DID pass before this
			// deadline was added, because cmd.Wait blocked for the full 30
			// seconds and then reported a clean exit.
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()

			select {
			case waitErr := <-done:
				// A signalled process does not exit cleanly.
				if waitErr == nil {
					t.Fatalf("the child exited cleanly after %v; the signal did not stop it", tc.sig)
				}
			case <-time.After(10 * time.Second):
				_ = cmd.Process.Kill()
				t.Fatalf("the child was still running 10s after %v", tc.sig)
			}

			// After the wait the pid is reaped, so the kernel answers ESRCH.
			alive, err = sys.ProcessAlive(pid)
			if err != nil {
				t.Fatalf("ProcessAlive after the process was reaped: %v", err)
			}
			if alive {
				t.Errorf("a reaped process was still reported as running after %v", tc.sig)
			}
		})
	}
}

// TestExecSystemSignalToAProcessItMayNotSignal covers the non-ESRCH error
// branch of SignalProcess.
//
// pid 1 belongs to root, so as an ordinary user the kernel refuses with EPERM
// and delivers nothing. That has to be reported: a stop that could not be
// performed must not read as a stop that succeeded.
func TestExecSystemSignalToAProcessItMayNotSignal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which may signal pid 1; this test must never actually signal init")
	}
	sys := NewExecSystem()

	err := sys.SignalProcess(1, SignalTerm)
	if err == nil {
		t.Fatal("SignalProcess reported success for a process it is not allowed to signal")
	}
	if !strings.Contains(err.Error(), "could not send") {
		t.Errorf("the error does not say the signal could not be sent: %v", err)
	}
	// The signal name is in the message, which is what Signal.String is for.
	if !strings.Contains(err.Error(), "TERM") {
		t.Errorf("the error does not name the signal: %v", err)
	}
}

// The two address failures are opposite faults and must not share an answer.
//
// dnsmasq prints "failed to create listening socket for <addr>: ..." for BOTH
// EADDRINUSE and EADDRNOTAVAIL, and that shared prefix used to be ORed into the
// "somebody else holds it" arm, so the wrong one won.
//
// MEASURED on the box 2026-08-30: NetworkManager took the interface this
// appliance had just created and flushed its address, dnsmasq printed
// "Cannot assign requested address", and the user was told another program held
// it and to restart the machine. Nothing held it, and restarting reproduces it,
// so the advice sent somebody after a program that did not exist.
func TestTheTwoAddressFailuresAreToldApart(t *testing.T) {
	notAvail := explainFailure(unitDHCP, 2,
		"dnsmasq: failed to create listening socket for 10.83.51.1: Cannot assign requested address\n", "")
	inUse := explainFailure(unitDHCP, 2,
		"dnsmasq: failed to create listening socket for 10.83.51.1: Address already in use\n", "")

	if notAvail == inUse {
		t.Fatalf("both address failures give the same answer, so one of them is wrong:\n%s", notAvail)
	}
	if strings.Contains(strings.ToLower(notAvail), "another program") {
		t.Errorf("an address that is on no interface is reported as another program holding it: %s", notAvail)
	}
	if !strings.Contains(strings.ToLower(inUse), "another program") {
		t.Errorf("an address genuinely held by something else no longer says so: %s", inUse)
	}
	// Restarting reproduces the flush, so advising it wastes the one action a
	// person in this state is most likely to try.
	if strings.Contains(strings.ToLower(notAvail), "restart the machine") {
		t.Errorf("the answer advises a restart, which reproduces this fault: %s", notAvail)
	}
}
