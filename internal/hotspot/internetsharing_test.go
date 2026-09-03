// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package hotspot

import (
	"context"
	"encoding/base64"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// A responder that plays a Mac: the radio, the network services, the bridge
// that appears after the preferences are committed. Addresses are from the
// documentation ranges and the MACs are locally administered.
type macResponder struct {
	radioOn   bool
	bridgeUp  bool
	bridgeGW  string
	scutilErr bool
	kickstart int
}

const macPrefsJSON = `{"NetworkServices":{"AAAA-1111":{"Interface":{"DeviceName":"en7","Type":"Ethernet"}},` +
	`"BBBB-2222":{"Interface":{"DeviceName":"en0","Type":"IEEE80211"}}}}`

func (m *macResponder) respond(_ *Recorder, name string, args []string) (Result, error) {
	base := name[strings.LastIndex(name, "/")+1:]
	switch base {
	case "networksetup":
		switch args[0] {
		case "-getairportpower":
			if m.radioOn {
				return Result{Stdout: "Wi-Fi Power (en0): On\n"}, nil
			}
			return Result{Stdout: "Wi-Fi Power (en0): Off\n"}, nil
		case "-setairportpower":
			m.radioOn = args[2] == "on"
			return Result{}, nil
		}
	case "plutil":
		if args[1] == "json" {
			return Result{Stdout: macPrefsJSON}, nil
		}
		return Result{Stdout: "<plist><dict><key>NAT</key><dict><key>AirPort</key><dict><key>Enabled</key><integer>1</integer></dict><key>Enabled</key><integer>1</integer></dict></dict></plist>"}, nil
	case "scutil":
		if m.scutilErr {
			return Result{ExitCode: 1, Stderr: "scutil: no"}, nil
		}
		// The commit is what brings the network up or down.
		m.bridgeUp = !m.bridgeUp
		return Result{}, nil
	case "launchctl":
		m.kickstart++
		m.bridgeUp = true
		return Result{}, nil
	case "ifconfig":
		if !m.bridgeUp {
			return Result{ExitCode: 1, Stderr: "ifconfig: interface bridge100 does not exist"}, nil
		}
		return Result{Stdout: "bridge100: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500\n\tinet " +
			m.bridgeGW + " netmask 0xffffff00 broadcast 10.83.51.255\n\tmember: ap1 flags=3<LEARNING,DISCOVER>\n"}, nil
	}
	return Result{}, nil
}

func newMac(t *testing.T, m *macResponder) (*InternetSharing, *Recorder) {
	t.Helper()
	rec := NewRecorder()
	rec.Responder = m.respond
	s := NewInternetSharing(rec, DefaultInternetSharingPaths())
	s.StartSettle = 0
	s.StartTries = 3
	return s, rec
}

func TestInternetSharing_StartWritesThePreferencesAndReadsTheBridgeBack(t *testing.T) {
	m := &macResponder{radioOn: false, bridgeGW: "10.83.51.1"}
	s, rec := newMac(t, m)
	plan := macPlan(t)

	st, err := s.Start(context.Background(), plan)
	if err != nil {
		t.Fatalf("start: %v (%s)", err, st.Reason)
	}
	if !st.Running || !st.AccessPoint.Running || !st.DHCP.Running {
		t.Fatalf("status = %+v", st)
	}
	if !m.radioOn {
		t.Fatal("the radio was off and must have been switched on")
	}
	if m.kickstart != 0 {
		t.Fatal("scutil brought the network up; the daemon must not have been kickstarted")
	}

	prefs := string(rec.Files[DefaultInternetSharingPaths().NATPrefs])
	for _, want := range []string{
		"<key>PrimaryService</key>\n\t\t<string>AAAA-1111</string>",
		"<string>en0</string>",
		"<key>SharingNetworkNumberStart</key>\n\t\t<string>10.83.51.0</string>",
		"<key>SharingNetworkMask</key>\n\t\t<string>255.255.255.0</string>",
		"<key>NetworkName</key>\n\t\t\t<string>Caspian-Wifi</string>",
		"<key>Channel</key>\n\t\t\t<integer>6</integer>",
		"<key>Enabled</key>\n\t\t<integer>1</integer>",
	} {
		if !strings.Contains(prefs, want) {
			t.Errorf("preferences lack %q:\n%s", want, prefs)
		}
	}
	// The passphrase is data in UTF-16LE, never the plain string.
	if strings.Contains(prefs, "example-password") {
		t.Fatal("the passphrase must not appear as plain text")
	}
	want := base64.StdEncoding.EncodeToString([]byte("e\x00x\x00a\x00m\x00p\x00l\x00e\x00-\x00p\x00a\x00s\x00s\x00w\x00o\x00r\x00d\x00"))
	if !strings.Contains(prefs, "<data>"+want+"</data>") {
		t.Fatalf("passphrase encoding is not UTF-16LE base64:\n%s", prefs)
	}
	if rec.Perms[DefaultInternetSharingPaths().NATPrefs] != 0o644 {
		t.Fatalf("mode = %o, want 0644 as the Sharing pane leaves it", rec.Perms[DefaultInternetSharingPaths().NATPrefs])
	}

	// The nudge went through scutil --prefs with the commit and apply script.
	var nudged bool
	for _, c := range rec.Calls {
		if strings.HasSuffix(c.Name, "scutil") {
			nudged = true
			if c.Args[0] != "--prefs" || c.Args[1] != natPrefsName || !strings.Contains(c.Stdin, "commit\napply") {
				t.Errorf("scutil call = %v stdin %q", c.Args, c.Stdin)
			}
		}
	}
	if !nudged {
		t.Fatal("configd was never told about the preferences")
	}

	// Idempotent: the same plan again writes nothing and disconnects nobody.
	before := len(rec.Calls)
	if _, err := s.Start(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Calls[before:] {
		if strings.HasSuffix(c.Name, "scutil") || strings.HasSuffix(c.Name, "launchctl") {
			t.Fatalf("a second start with the same plan must not %s", c.Name)
		}
	}
}

func TestInternetSharing_KickstartsTheDaemonWhenTheNudgeIsIgnored(t *testing.T) {
	m := &macResponder{radioOn: true, bridgeGW: "10.83.51.1", scutilErr: false}
	s, _ := newMac(t, m)
	// scutil flips the bridge; make it flip back down again so the first wait
	// fails and the fallback runs.
	m.bridgeUp = true // scutil will toggle it off
	st, err := s.Start(context.Background(), macPlan(t))
	if err != nil {
		t.Fatalf("start: %v (%s)", err, st.Reason)
	}
	if m.kickstart != 1 {
		t.Fatalf("kickstarts = %d, want exactly one after the nudge did nothing", m.kickstart)
	}
}

func TestInternetSharing_RefusesWhatItCannotDo(t *testing.T) {
	s, _ := newMac(t, &macResponder{radioOn: true, bridgeGW: "10.83.51.1"})
	p := macPlan(t)
	p.AP.Uplink = ""
	if _, err := s.Start(context.Background(), p); err == nil {
		t.Fatal("no uplink: Internet Sharing has nothing to share")
	}
	p = macPlan(t)
	p.AP.Interface = ""
	if _, err := s.Start(context.Background(), p); err == nil {
		t.Fatal("no radio interface")
	}
	p = macPlan(t)
	p.AP.Uplink = "en9"
	if _, err := s.Start(context.Background(), p); err == nil {
		t.Fatal("an uplink with no network service cannot be shared from")
	}
	m := &macResponder{radioOn: true, bridgeGW: "10.83.51.1", scutilErr: true}
	s, _ = newMac(t, m)
	if _, err := s.Start(context.Background(), macPlan(t)); err == nil {
		t.Fatal("scutil failing is a failure, not a success")
	}
}

func TestInternetSharing_StatusCountsLeasesInOurSubnetOnly(t *testing.T) {
	m := &macResponder{radioOn: true, bridgeUp: true, bridgeGW: "10.83.51.1"}
	s, rec := newMac(t, m)
	now := time.Unix(1_800_000_000, 0)
	s.SetClock(func() time.Time { return now })
	if _, err := s.Start(context.Background(), macPlan(t)); err != nil {
		t.Fatal(err)
	}
	live := "0x" + strings.ToLower(strings.TrimLeft(strings.ToUpper(hex64(now.Add(time.Hour).Unix())), "0"))
	dead := "0x" + hex64(now.Add(-time.Hour).Unix())
	rec.Files["/var/db/dhcpd_leases"] = []byte("{\n\tname=phone\n\tip_address=10.83.51.7\n\thw_address=1,02:00:5e:00:00:07\n\tidentifier=1,02:00:5e:00:00:07\n\tlease=" + live + "\n}\n" +
		"{\n\tname=old\n\tip_address=10.83.51.8\n\thw_address=1,02:00:5e:00:00:08\n\tlease=" + dead + "\n}\n" +
		"{\n\tname=other\n\tip_address=192.168.2.9\n\thw_address=1,02:00:5e:00:00:09\n\tlease=" + live + "\n}\n" +
		"{\n\tname=broken\n\thw_address=1,02:00:5e:00:00:0a\n}\n")
	st, err := s.Status(context.Background(), "en0")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Running || len(st.Devices) != 1 || st.Devices[0].IP.String() != "10.83.51.7" || st.Devices[0].MAC != "02:00:5e:00:00:07" {
		t.Fatalf("status = %+v", st)
	}
	if st.MalformedLeaseLines != 1 {
		t.Fatalf("malformed = %d, want 1 for the record with no address", st.MalformedLeaseLines)
	}
	if st.Radio.SoftBlocked {
		t.Fatal("the radio is on")
	}

	// The bridge going away is not running, with a reason.
	m.bridgeUp = false
	st, _ = s.Status(context.Background(), "en0")
	if st.Running || st.Reason == "" {
		t.Fatalf("status = %+v", st)
	}
}

func TestInternetSharing_StopSwitchesOffAndRestoresTheRadio(t *testing.T) {
	m := &macResponder{radioOn: false, bridgeGW: "10.83.51.1"}
	s, rec := newMac(t, m)
	if _, err := s.Start(context.Background(), macPlan(t)); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	prefs := string(rec.Files[DefaultInternetSharingPaths().NATPrefs])
	if !strings.Contains(prefs, "<key>Enabled</key>\n\t\t<integer>0</integer>") {
		t.Fatalf("stop must write sharing off:\n%s", prefs)
	}
	if m.radioOn {
		t.Fatal("the radio this driver switched on must be switched back off")
	}
	if m.bridgeUp {
		t.Fatal("the bridge must be gone")
	}

	// A fresh process stopping what a previous one started works from the
	// file on disk.
	m2 := &macResponder{radioOn: true, bridgeUp: true, bridgeGW: "10.83.51.1"}
	s2, rec2 := newMac(t, m2)
	rec2.Files[DefaultInternetSharingPaths().NATPrefs] = []byte("binary plist stands in here")
	if err := s2.Stop(context.Background()); err != nil {
		t.Fatalf("stop from disk: %v", err)
	}
	if m2.radioOn == false {
		t.Fatal("a radio this process did not switch on must be left alone")
	}
	if !strings.Contains(string(rec2.Files[DefaultInternetSharingPaths().NATPrefs]), "<key>Enabled</key><integer>0</integer></dict></dict></plist>") {
		t.Fatalf("the NAT Enabled key, not AirPort's, must be the one set to 0:\n%s", rec2.Files[DefaultInternetSharingPaths().NATPrefs])
	}
}

func TestParseDHCPDLeasesAndHelpers(t *testing.T) {
	leases, malformed := ParseDHCPDLeases("{\n\tname=a\n\tip_address=192.0.2.5\n\thw_address=1,02:00:5E:00:00:05\n\tlease=0x10\n}\n{\n\tname=b\n}\n")
	if len(leases) != 1 || malformed != 1 || leases[0].MAC != "02:00:5e:00:00:05" || leases[0].Expiry.Unix() != 16 || leases[0].Hostname != "a" {
		t.Fatalf("leases = %+v malformed %d", leases, malformed)
	}
	if id, err := ServiceForInterface(macPrefsJSON, "en7"); err != nil || id != "AAAA-1111" {
		t.Fatalf("service = %q %v", id, err)
	}
	if _, err := ServiceForInterface("not json", "en7"); err == nil {
		t.Fatal("bad JSON must be an error")
	}
	if on, ok := ParseAirportPower("Wi-Fi Power (en0): On\n"); !ok || !on {
		t.Fatal("power on misread")
	}
	if _, ok := ParseAirportPower("en0 is not a Wi-Fi interface\n"); ok {
		t.Fatal("unknown wording must not be known")
	}
	up, addrs := ParseIfconfigBrief("bridge100: flags=8863<UP,BROADCAST> mtu 1500\n\tinet 10.83.51.1 netmask 0xffffff00\n")
	if !up || len(addrs) != 1 {
		t.Fatalf("brief = %v %v", up, addrs)
	}
	if prefixMask(netip.MustParsePrefix("10.0.0.0/8")) != "255.0.0.0" || prefixMask(netip.MustParsePrefix("10.0.0.0/30")) != "255.255.255.252" {
		t.Fatal("mask rendering")
	}
	if xmlEscape(`a<b>&"c`) != "a&lt;b&gt;&amp;&quot;c" {
		t.Fatal("xml escaping")
	}
	if got := disableInNATPrefs("<key>Enabled</key><integer>1</integer><key>Enabled</key><integer>1</integer>"); got != "<key>Enabled</key><integer>1</integer><key>Enabled</key><integer>0</integer>" {
		t.Fatalf("disable = %q", got)
	}
	if disableInNATPrefs("nothing") != "nothing" {
		t.Fatal("no key, no change")
	}
	var ap AccessPoint = NewInternetSharing(NewRecorder(), DefaultInternetSharingPaths())
	if ap == nil {
		t.Fatal("interface")
	}
	if !errors.Is(context.Canceled, context.Canceled) {
		t.Fatal("sanity")
	}
}

func hex64(v int64) string {
	const digits = "0123456789abcdef"
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{digits[v&0xf]}, b...)
		v >>= 4
	}
	return string(b)
}

func TestInternetSharing_EdgesTheMacCanPresent(t *testing.T) {
	// The radio power query answering nothing readable: not known, so Start
	// switches the radio on and records having done so.
	m := &macResponder{radioOn: true, bridgeGW: "10.83.51.1"}
	s, rec := newMac(t, m)
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "networksetup") && args[0] == "-getairportpower" {
			return Result{Stdout: "en0 is not a Wi-Fi interface\n"}, nil
		}
		return m.respond(r, name, args)
	}
	if _, err := s.Start(context.Background(), macPlan(t)); err != nil {
		t.Fatalf("start with an unreadable radio: %v", err)
	}
	if s.poweredOn != "en0" {
		t.Fatal("an unreadable radio is switched on, and remembered so Stop can undo it")
	}
	st, _ := s.Status(context.Background(), "en0")
	if st.Radio.SoftBlocked {
		t.Fatal("unknown power must not read as blocked")
	}

	// networksetup refusing to switch the radio on.
	s, rec = newMac(t, &macResponder{radioOn: false, bridgeGW: "10.83.51.1"})
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "networksetup") && args[0] == "-setairportpower" {
			return Result{ExitCode: 1, Stderr: "Error: no"}, nil
		}
		if strings.HasSuffix(name, "networksetup") {
			return Result{Stdout: "Wi-Fi Power (en0): Off\n"}, nil
		}
		return Result{}, nil
	}
	if _, err := s.Start(context.Background(), macPlan(t)); err == nil || !strings.Contains(err.Error(), "exited 1") {
		t.Fatalf("err = %v", err)
	}
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "networksetup") {
			return Result{}, errors.New("not found")
		}
		return Result{}, nil
	}
	if _, err := s.Start(context.Background(), macPlan(t)); err == nil {
		t.Fatal("a networksetup that cannot run is a failure")
	}

	// plutil failing, plutil exiting non-zero, and a service list with two
	// services on the uplink (the lower UUID wins, deterministically).
	s, rec = newMac(t, &macResponder{radioOn: true, bridgeGW: "10.83.51.1"})
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "plutil") {
			return Result{}, errors.New("no plutil")
		}
		return Result{Stdout: "Wi-Fi Power (en0): On\n"}, nil
	}
	if _, err := s.Start(context.Background(), macPlan(t)); err == nil {
		t.Fatal("no plutil, no service list")
	}
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "plutil") {
			return Result{ExitCode: 1, Stderr: "plutil: bad"}, nil
		}
		return Result{Stdout: "Wi-Fi Power (en0): On\n"}, nil
	}
	if _, err := s.Start(context.Background(), macPlan(t)); err == nil {
		t.Fatal("plutil exiting 1 is a failure")
	}
	two := `{"NetworkServices":{"ZZZZ":{"Interface":{"DeviceName":"en7"}},"AAAA":{"Interface":{"DeviceName":"en7"}}}}`
	if id, err := ServiceForInterface(two, "en7"); err != nil || id != "AAAA" {
		t.Fatalf("two services: %q %v", id, err)
	}
	if _, err := ServiceForInterface(two, "en9"); err == nil {
		t.Fatal("no service for en9")
	}

	// scutil cannot run at all (as opposed to exiting non-zero).
	m = &macResponder{radioOn: true, bridgeGW: "10.83.51.1"}
	s, rec = newMac(t, m)
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "scutil") {
			return Result{}, errors.New("cannot run")
		}
		return m.respond(r, name, args)
	}
	if _, err := s.Start(context.Background(), macPlan(t)); err == nil {
		t.Fatal("scutil not runnable is a failure")
	}

	// The bridge never appearing, kickstart also failing: the reason says so.
	m = &macResponder{radioOn: true, bridgeGW: "10.83.51.1"}
	s, rec = newMac(t, m)
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		switch {
		case strings.HasSuffix(name, "scutil"):
			return Result{}, nil // accepted, does nothing
		case strings.HasSuffix(name, "launchctl"):
			return Result{}, errors.New("no launchctl")
		}
		return m.respond(r, name, args)
	}
	st, err := s.Start(context.Background(), macPlan(t))
	if err == nil || !strings.Contains(st.Reason, "did not bring the network up") {
		t.Fatalf("err = %v reason = %q", err, st.Reason)
	}

	// The bridge carrying an address that is not ours: status names it.
	m = &macResponder{radioOn: true, bridgeUp: true, bridgeGW: "192.168.2.1"}
	s, _ = newMac(t, m)
	s.mu.Lock()
	s.plan, s.have = macPlan(t), true
	s.mu.Unlock()
	st, _ = s.Status(context.Background(), "en0")
	if st.Running || !strings.Contains(st.Reason, "192.168.2.1") {
		t.Fatalf("status = %+v", st)
	}

	// A cancelled context ends the wait for the bridge with the context's error.
	m = &macResponder{radioOn: true, bridgeGW: "10.83.51.1"}
	s, rec = newMac(t, m)
	s.StartSettle = time.Hour
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "scutil") {
			return Result{}, nil
		}
		return m.respond(r, name, args)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Start(ctx, macPlan(t)); err == nil {
		t.Fatal("a cancelled context must end the wait")
	}
	if s.StartTries = 0; s.StartTries != 0 {
		t.Fatal("unreachable")
	}
	if err := s.awaitBridge(context.Background(), netip.Addr{}, true); err == nil {
		t.Fatal("zero tries still checks once and reports")
	}

	// Stop with no plan and an unreadable file changes nothing and reports nothing.
	m = &macResponder{radioOn: true, bridgeUp: false, bridgeGW: "10.83.51.1"}
	s, rec = newMac(t, m)
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "plutil") {
			return Result{ExitCode: 1, Stderr: "no such file"}, nil
		}
		return m.respond(r, name, args)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("stop with nothing to stop: %v", err)
	}

	// Stop where the bridge refuses to go away and the radio refuses to switch
	// off collects both complaints.
	m = &macResponder{radioOn: false, bridgeGW: "10.83.51.1"}
	s, rec = newMac(t, m)
	if _, err := s.Start(context.Background(), macPlan(t)); err != nil {
		t.Fatal(err)
	}
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		switch {
		case strings.HasSuffix(name, "scutil"), strings.HasSuffix(name, "launchctl"):
			return Result{}, nil // bridge stays up
		case strings.HasSuffix(name, "networksetup") && args[0] == "-setairportpower":
			return Result{ExitCode: 1, Stderr: "Error"}, nil
		}
		return m.respond(r, name, args)
	}
	err = s.Stop(context.Background())
	if err == nil || !strings.Contains(err.Error(), "still up") || !strings.Contains(err.Error(), "switching the radio back off") {
		t.Fatalf("stop err = %v", err)
	}

	// A write that fails is reported.
	m = &macResponder{radioOn: true, bridgeGW: "10.83.51.1"}
	s, rec = newMac(t, m)
	rec.WriteErr = errors.New("read-only file system")
	if _, err := s.Start(context.Background(), macPlan(t)); err == nil {
		t.Fatal("an unwritable preferences file is a failure")
	}

	if prefixMask(netip.MustParsePrefix("2001:db8::/64")) != "255.255.255.0" {
		t.Fatal("an IPv6 prefix falls back to the class C mask Internet Sharing expects")
	}
	if _, addrs := ParseIfconfigBrief("bridge100: flags=8863<UP> mtu 1500\n\tinet6 fe80::1%bridge100 prefixlen 64\n"); len(addrs) != 0 {
		t.Fatal("only inet addresses count")
	}
	if up, _ := (&InternetSharing{sys: NewRecorder(), paths: DefaultInternetSharingPaths()}).bridgeUp(context.Background(), netip.MustParseAddr("10.0.0.1")); up {
		t.Fatal("a recorder with the default responder has no bridge")
	}
}

func TestInternetSharing_StopAndStatusRemainingBranches(t *testing.T) {
	// Stop with a plan whose uplink service can no longer be found falls back
	// to the file on disk, read through plutil as XML.
	m := &macResponder{radioOn: true, bridgeUp: true, bridgeGW: "10.83.51.1"}
	s, rec := newMac(t, m)
	s.mu.Lock()
	s.plan, s.have = macPlan(t), true
	s.plan.AP.Uplink = "en9"
	s.mu.Unlock()
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("stop via the file: %v", err)
	}
	if !strings.Contains(string(rec.Files[DefaultInternetSharingPaths().NATPrefs]), "<integer>0</integer></dict></dict></plist>") {
		t.Fatal("the on-disk preferences must have been switched off")
	}
	// Status with a plan and a bridge that has no address at all.
	m = &macResponder{radioOn: true, bridgeUp: true, bridgeGW: ""}
	s, rec = newMac(t, m)
	rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if strings.HasSuffix(name, "ifconfig") {
			return Result{Stdout: "bridge100: flags=8863<UP,BROADCAST> mtu 1500\n"}, nil
		}
		return m.respond(r, name, args)
	}
	s.mu.Lock()
	s.plan, s.have = macPlan(t), true
	s.mu.Unlock()
	st, _ := s.Status(context.Background(), "")
	if st.Running || st.Reason != "Internet Sharing has not brought the network up." {
		t.Fatalf("status = %+v", st)
	}
	// radioPower with no device is unknown.
	if _, known := s.radioPower(context.Background(), ""); known {
		t.Fatal("no device, nothing known")
	}
	// awaitBridge for "gone" when the bridge is up and stays up, with a
	// cancelled context, returns the context's error.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m = &macResponder{radioOn: true, bridgeUp: true, bridgeGW: "10.83.51.1"}
	s, _ = newMac(t, m)
	s.StartSettle = time.Hour
	if err := s.awaitBridge(ctx, netip.Addr{}, false); err == nil {
		t.Fatal("cancelled wait must fail")
	}
	s.SetClock(time.Now)
}

func TestInternetSharing_TheLastBranches(t *testing.T) {
	// Status with no plan and no bridge: not running, and says so plainly.
	m := &macResponder{radioOn: true, bridgeUp: false}
	s, _ := newMac(t, m)
	st, _ := s.Status(context.Background(), "en0")
	if st.Running || st.Reason != "Internet Sharing is not running." {
		t.Fatalf("status = %+v", st)
	}

	// Stop with a plan, where the preferences cannot be written: the error is
	// collected and the nudge still happens.
	m = &macResponder{radioOn: true, bridgeUp: true, bridgeGW: "10.83.51.1"}
	s, rec := newMac(t, m)
	s.mu.Lock()
	s.plan, s.have = macPlan(t), true
	s.mu.Unlock()
	rec.WriteErr = errors.New("disk full")
	if err := s.Stop(context.Background()); err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("stop err = %v", err)
	}

	// A wait whose sleep fails reports the sleep's error.
	m = &macResponder{radioOn: true, bridgeUp: false, bridgeGW: "10.83.51.1"}
	rec = NewRecorder()
	rec.Responder = m.respond
	s = NewInternetSharing(sleepless{Recorder: rec, err: errors.New("no sleep")}, DefaultInternetSharingPaths())
	s.StartTries = 2
	if err := s.awaitBridge(context.Background(), netip.MustParseAddr("10.83.51.1"), true); err == nil || err.Error() != "no sleep" {
		t.Fatalf("err = %v", err)
	}
}
