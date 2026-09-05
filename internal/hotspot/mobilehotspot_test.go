// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package hotspot

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// winResponder plays the tethering helper: it parses the request it was
// given on standard input and answers the way Windows would.
type winResponder struct {
	state       string // "off", "transition", "on"
	ssid        string
	startErr    string // a TetheringOperationStatus name to fail starts with
	requests    []TetheringRequest
	statusCalls int
}

func (w *winResponder) respond(rec *Recorder, name string, args []string) (Result, error) {
	if !strings.HasSuffix(name, "caspian-tethering.exe") {
		return Result{}, nil
	}
	var req TetheringRequest
	last := rec.Calls[len(rec.Calls)-1]
	if err := json.Unmarshal([]byte(last.Stdin), &req); err != nil {
		return Result{ExitCode: 2, Stderr: "bad request"}, nil
	}
	if req.Op != args[0] {
		return Result{ExitCode: 2, Stderr: "op mismatch"}, nil
	}
	w.requests = append(w.requests, req)
	var rep TetheringReply
	switch req.Op {
	case "start":
		if w.startErr != "" {
			rep = TetheringReply{OK: false, State: "off", Code: w.startErr, Error: "StartTetheringAsync: " + w.startErr}
			break
		}
		w.state, w.ssid = "transition", req.SSID
		rep = TetheringReply{OK: true, State: w.state}
	case "stop":
		w.state = "off"
		rep = TetheringReply{OK: true, State: "off"}
	case "status":
		w.statusCalls++
		if w.state == "transition" && w.statusCalls > 1 {
			w.state = "on"
		}
		rep = TetheringReply{OK: true, State: w.state, SSID: w.ssid}
		if w.state == "on" {
			rep.Clients = []TetheringClient{{MAC: "02:00:5E:00:00:07", Hostnames: []string{"phone"}}}
		}
	}
	b, _ := json.Marshal(rep)
	return Result{Stdout: string(b) + "\n"}, nil
}

func newWin(t *testing.T, w *winResponder) (*MobileHotspot, *Recorder) {
	t.Helper()
	rec := NewRecorder()
	rec.Responder = w.respond
	m := NewMobileHotspot(rec, MobileHotspotPaths{Helper: `C:\Program Files\Caspian\caspian-tethering.exe`})
	m.StartSettle = 0
	m.StartTries = 5
	return m, rec
}

func winPlan(t *testing.T) Plan {
	t.Helper()
	p := macPlan(t)
	p.AP.Interface = "Wi-Fi 2"
	p.AP.Uplink = "Ethernet"
	p.AP.Band = Band5GHz
	return p
}

func TestMobileHotspotPlan_WindowsAliasesDoNotUseLinuxConfigRules(t *testing.T) {
	p := macPlan(t)
	p.AP.Interface, p.AP.Uplink = "Wi-Fi 3", "Ethernet 2"
	p.DNS.Interface = p.AP.Interface
	got, err := NewMobileHotspotPlan(p.AP, p.DNS, p.Radio)
	if err != nil {
		t.Fatal(err)
	}
	if got.AP.Interface != "Wi-Fi 3" || got.HostapdConf != "" || got.DnsmasqConf != "" {
		t.Fatal("Windows plan altered the alias or rendered unused Linux configs")
	}
	if _, err := NewPlan(p.AP, p.DNS, p.Radio); err == nil {
		t.Fatal("Linux config token validation must remain strict")
	}
	p.AP.Interface = "Wi-Fi\n3"
	if _, err := NewMobileHotspotPlan(p.AP, p.DNS, p.Radio); err == nil {
		t.Fatal("Windows aliases must reject control characters")
	}
}

func TestMobileHotspot_StartAsksTheHelperAndWaitsForOn(t *testing.T) {
	w := &winResponder{state: "off"}
	m, rec := newWin(t, w)
	st, err := m.Start(context.Background(), winPlan(t))
	if err != nil {
		t.Fatalf("start: %v (%s)", err, st.Reason)
	}
	if !st.Running || len(st.Devices) != 1 || st.Devices[0].MAC != "02:00:5e:00:00:07" || st.Devices[0].Hostname != "phone" {
		t.Fatalf("status = %+v", st)
	}
	var start TetheringRequest
	for _, r := range w.requests {
		if r.Op == "start" {
			start = r
		}
	}
	if start.SSID != "Caspian-Wifi" || start.Passphrase != "example-password" || start.Band != "5" || start.Uplink != "Ethernet" || start.Adapter != "Wi-Fi 2" {
		t.Fatalf("start request = %+v", start)
	}
	for _, c := range rec.Calls {
		if c.Name != m.paths.Helper {
			t.Fatalf("the driver ran %s; it may run the helper and nothing else", c.Name)
		}
	}
	// On the air with our name: a second start does not start again.
	before := len(w.requests)
	if _, err := m.Start(context.Background(), winPlan(t)); err != nil {
		t.Fatal(err)
	}
	for _, r := range w.requests[before:] {
		if r.Op == "start" {
			t.Fatal("a second start with the same plan must not restart the hotspot")
		}
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if w.state != "off" {
		t.Fatal("stop must switch it off")
	}
	st, _ = m.Status(context.Background(), "")
	if st.Running || st.Reason != "Mobile Hotspot is off." {
		t.Fatalf("status after stop = %+v", st)
	}
}

func TestMobileHotspot_ReportsWindowsRefusalsInWords(t *testing.T) {
	for code, want := range map[string]string{
		"WiFiDeviceOff":                "The Wi-Fi radio is switched off.",
		"NetworkLimitedConnectivity":   "Windows refused to share a connection it does not consider connected to the internet.",
		"DisabledByGroupPolicy":        "Mobile Hotspot is disabled on this computer by policy.",
		"DisabledByHardwareLimitation": "No Wi-Fi adapter on this computer can host a hotspot: the driver does not support Wi-Fi Direct.",
		"BandInterference":             "The radio cannot host the network on the band that was asked for while joined to its current network.",
		"Unknown":                      "Windows reported: StartTetheringAsync: Unknown",
	} {
		m, _ := newWin(t, &winResponder{state: "off", startErr: code})
		st, err := m.Start(context.Background(), winPlan(t))
		if err == nil {
			t.Fatalf("%s: a refused start must be an error", code)
		}
		if st.Reason != want {
			t.Errorf("%s: reason = %q, want %q", code, st.Reason, want)
		}
	}
	if mobileHotspotReason(TetheringReply{State: "transition"}) != "Windows is still switching the hotspot." {
		t.Fatal("transition wording")
	}
	if mobileHotspotReason(TetheringReply{State: "unknown"}) != "Mobile Hotspot is not running." {
		t.Fatal("unknown wording")
	}
}

func TestMobileHotspot_RefusesWhatItCannotDo(t *testing.T) {
	m, _ := newWin(t, &winResponder{state: "off"})
	p := winPlan(t)
	p.AP.Uplink = ""
	if _, err := m.Start(context.Background(), p); err == nil {
		t.Fatal("no uplink, nothing to share")
	}
	m.paths.Helper = ""
	if _, err := m.Status(context.Background(), ""); err == nil {
		t.Fatal("no helper path configured")
	}
	// A helper that prints nothing, or garbage, is a failure with the exit
	// code in it, not a silent off.
	rec := NewRecorder()
	rec.Responder = func(*Recorder, string, []string) (Result, error) { return Result{ExitCode: 3, Stderr: "boom"}, nil }
	m = NewMobileHotspot(rec, MobileHotspotPaths{Helper: `C:\x\caspian-tethering.exe`})
	if _, err := m.Status(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "exited 3") {
		t.Fatalf("err = %v", err)
	}
	rec.Responder = func(*Recorder, string, []string) (Result, error) { return Result{Stdout: "not json\n"}, nil }
	if _, err := m.Status(context.Background(), ""); err == nil {
		t.Fatal("garbage must be refused")
	}
	if err := m.Stop(context.Background()); err == nil {
		t.Fatal("a stop whose helper answers garbage is not a success")
	}
	// A start that never reaches "on" is reported as such.
	w := &winResponder{state: "off"}
	m2, _ := newWin(t, w)
	m2.StartTries = 1
	w.statusCalls = -100 // keeps the responder in "transition"
	if _, err := m2.Start(context.Background(), winPlan(t)); err == nil {
		t.Fatal("a hotspot stuck in transition did not come on")
	}
	if tetheringBand(Band2GHz) != "2.4" || tetheringBand(Band("x")) != "auto" {
		t.Fatal("band words")
	}
}

func TestMobileHotspot_HelperFailuresAreErrorsWithTheirCause(t *testing.T) {
	// The helper cannot be run at all.
	rec := NewRecorder()
	rec.Responder = func(*Recorder, string, []string) (Result, error) { return Result{}, errors.New("no such file") }
	m := NewMobileHotspot(rec, MobileHotspotPaths{Helper: `C:\x\caspian-tethering.exe`})
	m.StartSettle, m.StartTries = 0, 1
	if _, err := m.Start(context.Background(), winPlan(t)); err == nil {
		t.Fatal("a helper that cannot run is a failure")
	}
	if err := m.Stop(context.Background()); err == nil {
		t.Fatal("stop with no helper is a failure")
	}
	// A helper that answers with two lines: only the first is the reply.
	rec.Responder = func(*Recorder, string, []string) (Result, error) {
		return Result{Stdout: `{"ok":true,"state":"off"}` + "\nnoise\n"}, nil
	}
	st, err := m.Status(context.Background(), "")
	if err != nil || st.Running {
		t.Fatalf("status = %+v %v", st, err)
	}
	// A stop the helper refuses while the hotspot is not off.
	rec.Responder = func(*Recorder, string, []string) (Result, error) {
		return Result{Stdout: `{"ok":false,"state":"on","error":"StopTetheringAsync: Unknown"}`}, nil
	}
	if err := m.Stop(context.Background()); err == nil || !strings.Contains(err.Error(), "StopTetheringAsync") {
		t.Fatalf("stop err = %v", err)
	}
	// A start whose wait is cancelled returns the context's error.
	w := &winResponder{state: "off"}
	m2, _ := newWin(t, w)
	m2.StartSettle = time.Hour
	w.statusCalls = -100
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m2.Start(ctx, winPlan(t)); err == nil {
		t.Fatal("a cancelled wait is an error")
	}
	// The idempotence probe failing (helper unreachable on the first status)
	// does not stop a start from being attempted.
	calls := 0
	rec2 := NewRecorder()
	w2 := &winResponder{state: "off"}
	rec2.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		calls++
		if calls == 1 {
			return Result{}, errors.New("flaky")
		}
		return w2.respond(r, name, args)
	}
	m3 := NewMobileHotspot(rec2, MobileHotspotPaths{Helper: `C:\Program Files\Caspian\caspian-tethering.exe`})
	m3.StartSettle, m3.StartTries = 0, 5
	m3.SetClock(time.Now)
	if _, err := m3.Start(context.Background(), winPlan(t)); err != nil {
		t.Fatalf("start after a flaky probe: %v", err)
	}
	// A start whose status never turns on and whose final status carries no
	// reason gets the generic sentence.
	w3 := &winResponder{state: "off"}
	m4, rec4 := newWin(t, w3)
	m4.StartTries = 1
	rec4.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		if args[0] == "status" {
			return Result{Stdout: `{"ok":true,"state":"weird"}`}, nil
		}
		return w3.respond(r, name, args)
	}
	st, err = m4.Start(context.Background(), winPlan(t))
	if err == nil || st.Reason == "" {
		t.Fatalf("st = %+v err = %v", st, err)
	}
}

func TestMobileHotspot_WaitFailuresSurface(t *testing.T) {
	// The sleep between polls failing ends the start with that error.
	w := &winResponder{state: "off"}
	rec := NewRecorder()
	rec.Responder = w.respond
	w.statusCalls = -100
	m := NewMobileHotspot(sleepless{Recorder: rec, err: errors.New("no sleep")}, MobileHotspotPaths{Helper: `C:\Program Files\Caspian\caspian-tethering.exe`})
	m.StartTries = 3
	if _, err := m.Start(context.Background(), winPlan(t)); err == nil || err.Error() != "no sleep" {
		t.Fatalf("err = %v", err)
	}
	// The final status after an exhausted wait failing outright leaves no
	// reason of its own, so the generic sentence is used.
	calls := 0
	rec2 := NewRecorder()
	w2 := &winResponder{state: "off"}
	rec2.Responder = func(r *Recorder, name string, args []string) (Result, error) {
		calls++
		if args[0] == "status" && calls > 2 {
			return Result{}, errors.New("helper gone")
		}
		return w2.respond(r, name, args)
	}
	m2 := NewMobileHotspot(rec2, MobileHotspotPaths{Helper: `C:\Program Files\Caspian\caspian-tethering.exe`})
	m2.StartSettle, m2.StartTries = 0, 1
	w2.statusCalls = -100
	st, err := m2.Start(context.Background(), winPlan(t))
	if err == nil || st.Reason != "The hotspot helper did not answer." {
		t.Fatalf("st = %+v err = %v", st, err)
	}
}
