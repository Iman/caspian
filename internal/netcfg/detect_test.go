// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func TestDetect_ReadsEverythingItNeeds(t *testing.T) {
	s := modeAScenario()
	r := s.runner(t)
	f, err := Detect(context.Background(), r, BaseSysctlKnobs())
	if err != nil {
		t.Fatal(err)
	}

	wantSequence(t, "detection commands", r.Lines(), []string{
		"ip -br addr",
		"ip route show default",
		"ip -6 route show default",
		"ip -d link show",
		"iw dev",
		// One probe per NON-ACCESS-POINT wireless interface. This scenario
		// has one. It is what makes a channel stop being evidence of an
		// association; see the comment in Detect.
		"iw dev wlan0 link",
		"iw list",
		"nmcli -t -f DEVICE,STATE device status",
		"sysctl -e -- net.ipv4.ip_forward net.ipv4.conf.all.rp_filter net.ipv4.conf.default.rp_filter net.ipv6.conf.all.forwarding",
	})

	if len(f.Links) != 3 || len(f.Wireless) != 1 || len(f.Phys) != 1 {
		t.Fatalf("links=%d wireless=%d phys=%d", len(f.Links), len(f.Wireless), len(f.Phys))
	}
	if len(f.Routes) != 3 {
		t.Fatalf("routes = %+v, want two IPv4 defaults and one IPv6", f.Routes)
	}
	if f.Sysctl["net.ipv4.conf.all.rp_filter"] != "1" {
		t.Errorf("sysctl = %v", f.Sysctl)
	}
	if !f.IsWireless("wlan0") || f.IsWireless("eth0") {
		t.Error("wireless classification is wrong")
	}
}

// Every command detection runs is read-only. A detection that changes the
// machine cannot be run before deciding whether to change the machine.
func TestDetect_OnlyRunsReadOnlyCommands(t *testing.T) {
	s := modeAScenario()
	r := s.runner(t)
	if _, err := Detect(context.Background(), r, BaseSysctlKnobs()); err != nil {
		t.Fatal(err)
	}
	// The check is per binary, because the same flag means different things
	// to different tools: "-f" loads a ruleset for nft and selects output
	// fields for nmcli.
	writes := map[string][]string{
		BinIP:     {"add", "del", "set", "flush"},
		BinIw:     {"add", "del", "set"},
		BinNft:    {"-f"},
		BinSysctl: {"-w"},
		BinNmcli:  {"set", "connect", "disconnect", "up", "down", "modify", "delete"},
	}
	for _, c := range r.Commands() {
		for _, a := range c.Args {
			for _, bad := range writes[c.Path] {
				if a == bad {
					t.Errorf("detection ran a command that changes the machine: %s", RunnerKey(c))
				}
			}
		}
	}
}

// A machine with no wireless tooling is a real machine. Detection reports what
// it found and the refusal, in plain words, belongs to the planner.
func TestDetect_SurvivesMissingWirelessTools(t *testing.T) {
	s := modeAScenario()
	r := s.runner(t)
	r.SetError("iw dev", errors.New("exec: \"iw\": executable file not found"))
	r.SetError("iw list", errors.New("exec: \"iw\": executable file not found"))

	f, err := Detect(context.Background(), r, BaseSysctlKnobs())
	if err != nil {
		t.Fatalf("a machine with no iw is not a detection failure: %v", err)
	}
	if len(f.Wireless) != 0 || len(f.Phys) != 0 {
		t.Errorf("wireless=%v phys=%v", f.Wireless, f.Phys)
	}

	_, err = PlanNetwork(f, []netip.Addr{testServer}, DefaultOptions())
	if !errors.Is(err, ErrNoAPCapableInterface) {
		t.Fatalf("err = %v, want ErrNoAPCapableInterface", err)
	}
}

// Interfaces and routes are the two reads that must succeed: facts without
// them are not facts.
func TestDetect_FailsWhenInterfacesCannotBeRead(t *testing.T) {
	s := modeAScenario()
	r := s.runner(t)
	r.SetError("ip -br addr", errors.New("permission denied"))
	if _, err := Detect(context.Background(), r, nil); err == nil {
		t.Fatal("detection must fail when it cannot list interfaces")
	}
}

// The knob read must not pass "-n".
//
// This is the defect that survived every green run before real bytes arrived.
// "-n" prints values with no names, so the output is a column of bare numbers,
// ParseSysctl finds no "=" and returns an empty map, Facts.Sysctl is empty and
// every sysctl step records no inverse. The box then keeps ip_forward and
// rp_filter after an uninstall that promised to return it to how it was found.
func TestDetect_SysctlReadKeepsTheNames(t *testing.T) {
	s := pi5Captured()
	r := s.runner(t)
	if _, err := Detect(context.Background(), r, BaseSysctlKnobs()); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range r.Commands() {
		if c.Path != BinSysctl {
			continue
		}
		found = true
		for _, a := range c.Args {
			if a == "-n" {
				t.Errorf("the sysctl read passes -n, which suppresses the names ParseSysctl needs: %s", RunnerKey(c))
			}
		}
	}
	if !found {
		t.Fatal("no sysctl read was made at all")
	}
}

// The bytes "-n" actually produced on the target. Keeping them as a fixture
// makes the defect reproducible rather than a story about it.
func TestParseSysctl_BareValuesYieldNothing(t *testing.T) {
	got := ParseSysctl(read(t, "capture-pi5-sysctl-n-flag.txt"))
	if len(got) != 0 {
		t.Fatalf("got %v, want an empty map: bare values carry no knob names", got)
	}
}

// And the shape the command produces without "-n" parses into every knob.
func TestParseSysctl_NamedValuesParse(t *testing.T) {
	got := ParseSysctl(read(t, "capture-pi5-sysctl-base.txt"))
	want := map[string]string{
		"net.ipv4.ip_forward":             "0",
		"net.ipv4.conf.all.rp_filter":     "0",
		"net.ipv4.conf.default.rp_filter": "2",
		"net.ipv6.conf.all.forwarding":    "0",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

// Every knob a plan changes must come back from the read with a value, or the
// change has no exact inverse and teardown cannot restore it. This is the
// end-to-end version of the defect above: it fails if the map is empty for any
// reason, not only because of "-n".
func TestDetectAndPlan_EveryChangedKnobHasAMeasuredValue(t *testing.T) {
	for _, sc := range []scenario{pi5Captured(), modeAScenario(), modeBScenario()} {
		t.Run(sc.name, func(t *testing.T) {
			f, p, err := DetectAndPlan(context.Background(), sc.runner(t), []netip.Addr{testServer}, DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			steps := p.AllSteps(f.Sysctl)
			changed := 0
			for _, st := range steps {
				if st.Do.Path != BinSysctl {
					continue
				}
				changed++
				if len(st.Do.Args) < 2 {
					t.Errorf("sysctl step has too few arguments: %v", st.Do.Args)
					continue
				}
				knob, _, _ := strings.Cut(st.Do.Args[1], "=")
				if _, ok := f.Sysctl[knob]; !ok {
					t.Errorf("%s is changed but was never measured, so teardown cannot restore it", knob)
				}
				if st.Undo.IsZero() {
					t.Errorf("%s is changed with no recorded inverse", knob)
				}
			}
			if changed == 0 {
				t.Fatal("no sysctl step was generated at all")
			}
		})
	}
}

// Detection makes one knob read, not two.
//
// The second read existed to fetch knobs whose names were only known once the
// plan had chosen its interfaces. No plan changes such a knob any more, so the
// second read has nothing to fetch and is skipped. DetectAndPlan still checks
// whether the plan needs a knob the first read did not return, which is where
// a future per-interface knob would be noticed rather than silently changed
// with no measured value.
func TestDetectAndPlan_MakesASingleKnobRead(t *testing.T) {
	s := pi5Captured()
	r := s.runner(t)
	f, p, err := DetectAndPlan(context.Background(), r, []netip.Addr{testServer}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	reads := 0
	for _, c := range r.Commands() {
		if c.Path == BinSysctl {
			reads++
		}
	}
	if reads != 1 {
		t.Errorf("made %d sysctl reads, want 1: every knob a plan changes is global", reads)
	}
	if !equalStrings(p.SysctlKnobs(), BaseSysctlKnobs()) {
		t.Errorf("SysctlKnobs %v differs from BaseSysctlKnobs %v, so one read cannot serve both",
			p.SysctlKnobs(), BaseSysctlKnobs())
	}
	for _, k := range p.SysctlKnobs() {
		if _, ok := f.Sysctl[k]; !ok {
			t.Errorf("%s was never returned by the single read", k)
		}
	}
}
