// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

// scenario names the fixtures that stand in for one machine.
//
// There are two kinds and the difference decides what a green test proves.
// Files named capture-* are byte captures from the target Raspberry Pi 5;
// files named scenario-* describe machines nobody has measured. Both are kept
// on purpose. A suite that knows only the captured machine passes on that
// machine and misleads everywhere else, and a suite that knows only authored
// bytes is what let a parser that could not read "2412.0 MHz" stay green.
// testdata/PROVENANCE.md records which file is which.
type scenario struct {
	name       string
	route      string
	route6     string
	addr       string
	dlink      string
	iwdev      string
	iwlist     string
	sysctlBase string
	// nmcli is the "nmcli -t -f DEVICE,STATE device status" fixture. Empty
	// means the command fails, which is a real state: NetworkManager absent.
	nmcli string
}

// pi5Captured is the target as it actually is: a Raspberry Pi 5 with the wired
// port and the built-in radio both holding leases on 10.0.0.0/24, the radio
// associated on channel 10, one radio only, and no IPv6 default route.
func pi5Captured() scenario {
	return scenario{
		name:       "captured pi5",
		route:      "capture-pi5-ip-route-default.txt",
		route6:     "capture-pi5-ip-route6-default.txt",
		addr:       "capture-pi5-ip-br-addr.txt",
		dlink:      "capture-pi5-ip-d-link.txt",
		iwdev:      "capture-pi5-iw-dev.txt",
		iwlist:     "capture-pi5-iw-list.txt",
		sysctlBase: "capture-pi5-sysctl-base.txt",
		nmcli:      "capture-pi5-nmcli-device-status.txt",
	}
}

// modeAScenario is an authored machine, not a measured one: a wired uplink on
// 192.168.1.0/24, an IPv6 default route, and a radio whose iw prints
// frequencies in the older integer form. Each of those covers something the
// captured machine cannot: an address range that collides with the common
// domestic one, the IPv6 pinning path, and the pre-decimal iw output.
func modeAScenario() scenario {
	return scenario{
		name:       "authored mode A",
		route:      "scenario-modea-ip-route-default.txt",
		route6:     "scenario-modea-ip-route6-default.txt",
		addr:       "scenario-modea-ip-br-addr.txt",
		dlink:      "scenario-modea-ip-d-link.txt",
		iwdev:      "scenario-modea-iw-dev.txt",
		iwlist:     "scenario-modea-iw-list.txt",
		sysctlBase: "scenario-modea-sysctl-base.txt",
		nmcli:      "capture-pi5-nmcli-device-status.txt",
	}
}

// modeBScenario is authored: the uplink is the built-in radio and a USB
// adapter reporting AP support is attached. No such adapter was attached to
// the target, so mode B is proven against authored bytes only.
func modeBScenario() scenario {
	return scenario{
		name:       "authored mode B",
		route:      "scenario-modeb-ip-route-default.txt",
		route6:     "",
		addr:       "scenario-modeb-ip-br-addr.txt",
		dlink:      "scenario-modeb-ip-d-link.txt",
		iwdev:      "scenario-modeb-iw-dev.txt",
		iwlist:     "scenario-modeb-iw-list.txt",
		sysctlBase: "scenario-modeb-sysctl-base.txt",
		nmcli:      "capture-pi5-nmcli-device-status.txt",
	}
}

// sysctlKnobsIn returns the knob names a fixture file holds, in order.
func sysctlKnobsIn(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if k, _, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			out = append(out, strings.TrimSpace(k))
		}
	}
	return out
}

func (s scenario) runner(t *testing.T) *RecordingRunner {
	t.Helper()
	r := NewRecordingRunner()
	set := func(key, file string) {
		if file != "" {
			r.SetOutput(key, read(t, file))
		}
	}
	set("ip -br addr", s.addr)
	set("ip route show default", s.route)
	set("ip -6 route show default", s.route6)
	set("ip -d link show", s.dlink)
	set("iw dev", s.iwdev)
	set("iw list", s.iwlist)
	set("nmcli -t -f DEVICE,STATE device status", s.nmcli)

	// The sysctl read is answered with the bytes of a fixture file, verbatim.
	//
	// It used to be answered by formatting "name = value" lines from a Go map.
	// That made the double and ParseSysctl agree by construction: the double
	// produced exactly the shape the parser wanted, so no test could see that
	// the real command was being asked for a different shape entirely. The
	// real command carried "-n", which prints values with no names, and the
	// parser returned an empty map on the box while every test stayed green.
	// The double must never format what the parser will read.
	base := read(t, s.sysctlBase)
	r.Fallback = func(c Command) (Result, error) {
		if c.Path != BinSysctl {
			return Result{}, nil
		}
		var asked []string
		for _, a := range c.Args {
			if !strings.HasPrefix(a, "-") {
				asked = append(asked, a)
			}
		}

		// There is exactly one read, for the global knobs, and it is the only
		// one any fixture models. A read for anything else means a plan has
		// started changing a knob that names an interface, which is the shape
		// that produced an unmeasurable value, an unorderable write and a
		// teardown that would have weakened the box. Fail here and read the
		// note above Plan.SysctlKnobs before adding a fixture to make it pass.
		if !equalStrings(asked, BaseSysctlKnobs()) {
			t.Fatalf("sysctl read asked for a knob set no fixture models.\nasked: %v\nglobal knobs: %v", asked, BaseSysctlKnobs())
		}

		// The fixture's knobs must be an ordered SUBSET of the ask, not equal
		// to it.
		//
		// Detect passes "-e" so that a knob this kernel does not have is
		// skipped instead of failing the whole read. Requiring equality here
		// was the assumption that a line always comes back for every knob
		// asked. The box disproved it: eight knobs asked, five lines returned,
		// exit 0, empty stderr, measured on the target on 2026-08-30 and kept
		// as capture-pi5-sysctl-absent-interfaces.txt. What still has to hold,
		// and what this still catches, is that a fixture never answers with a
		// knob nobody asked for.
		if !orderedSubset(sysctlKnobsIn(base), asked) {
			t.Fatalf("fixture %s answers with a knob that was not asked for.\nasked: %v\nfixture has: %v",
				s.sysctlBase, asked, sysctlKnobsIn(base))
		}
		return Result{Stdout: base}, nil
	}
	return r
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// orderedSubset reports whether every name in sub appears in super, in the same
// order. That is the relation a sysctl read holds to the knobs it asked for:
// "-e" drops a knob the kernel does not have and leaves the rest in the order
// they were given, so the output is a subsequence of the ask and equality is
// the special case where every knob happened to exist.
func orderedSubset(sub, super []string) bool {
	i := 0
	for _, name := range super {
		if i < len(sub) && sub[i] == name {
			i++
		}
	}
	return i == len(sub)
}

func (s scenario) facts(t *testing.T, knobs []string) Facts {
	t.Helper()
	f, err := Detect(context.Background(), s.runner(t), knobs)
	if err != nil {
		t.Fatalf("detect (%s): %v", s.name, err)
	}
	return f
}

// testServer is the user's proxy server address. RFC 5737 documentation space,
// so it belongs to nobody.
var testServer = netip.MustParseAddr("203.0.113.10")

func mustPlan(t *testing.T, s scenario, o Options) (Facts, *Plan) {
	t.Helper()
	f := s.facts(t, BaseSysctlKnobs())
	p, err := PlanNetwork(f, []netip.Addr{testServer}, o)
	if err != nil {
		t.Fatalf("plan (%s): %v", s.name, err)
	}
	full := s.facts(t, p.SysctlKnobs())
	return full, p
}

func stepKeys(steps []Step) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, CommandLine(s.Do))
	}
	return out
}

func undoKeys(steps []Step) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		if s.Undo.IsZero() {
			out = append(out, "(no inverse)")
			continue
		}
		out = append(out, CommandLine(s.Undo))
	}
	return out
}

func wantSequence(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d entries, want %d\ngot:\n  %s\nwant:\n  %s",
			what, len(got), len(want), strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: entry %d\n got: %s\nwant: %s\nfull got:\n  %s",
				what, i, got[i], want[i], strings.Join(got, "\n  "))
		}
	}
}

func contains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected to find:\n  %s\nin:\n%s", needle, haystack)
	}
}

func notContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("did not expect to find:\n  %s\nin:\n%s", needle, haystack)
	}
}

// indexOf returns the position of a command key in a rendered step sequence,
// or -1. Order assertions use it instead of substring searching a joined
// string, where "ip address add ..." would also match a line that merely
// contains it.
func indexOf(keys []string, want string) int {
	for i, k := range keys {
		if k == want {
			return i
		}
	}
	return -1
}
