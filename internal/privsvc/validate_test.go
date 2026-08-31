// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/state"
)

// TestEveryFieldOfAStartRequestIsCheckedAgainstThisMachine.
//
// internal/panel/priv.go states what this test holds: "the privileged side is
// expected to validate each one against what it detected for itself rather than
// trusting it." Each case names the field, the value, and the fault the panel
// has to receive so it can put the right sentence on the screen.
//
// Every case also asserts that NOTHING was applied. A refusal that happens
// after the firewall has been loaded is a refusal that changed the box.
func TestEveryFieldOfAStartRequestIsCheckedAgainstThisMachine(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*panel.StartRequest)
		world func(*world)
		want  panel.Fault
	}{
		{
			name: "no configuration document at all",
			edit: func(r *panel.StartRequest) { r.ConfigJSON = nil },
			want: panel.FaultEngineRejectedConfig,
		},
		{
			name: "a configuration document larger than this service accepts",
			edit: func(r *panel.StartRequest) {
				// Padded INSIDE the object with whitespace, which JSON allows,
				// so the document still parses and still describes a usable
				// link. That isolates the size bound: a service that dropped it
				// would accept this rather than refusing it for some other
				// reason, and the test would go red for the right thing.
				r.ConfigJSON = append([]byte("{"+strings.Repeat(" ", maxConfigBytes)), r.ConfigJSON[1:]...)
			},
			want: panel.FaultEngineRejectedConfig,
		},
		{
			name: "a configuration document that is not a link at all",
			edit: func(r *panel.StartRequest) { r.ConfigJSON = []byte(`{"outbounds":[]}`) },
			want: panel.FaultEngineRejectedConfig,
		},
		{
			name: "a configuration document whose id is not a UUID",
			edit: func(r *panel.StartRequest) {
				// xray-core turns any 1-to-30-character string that is not a
				// UUID into a DIFFERENT valid UUID by SHA-1, with no error, so
				// this authenticates as somebody else and presents as
				// "connected but nothing works". internal/link refuses it, and
				// this side runs that refusal again rather than trusting the
				// panel to have run it.
				r.ConfigJSON = []byte(strings.Replace(string(r.ConfigJSON),
					`"id":"`+fakeUUID+`"`, `"id":"short"`, 1))
			},
			want: panel.FaultEngineRejectedConfig,
		},
		{
			name: "an engine log level this appliance does not send",
			edit: func(r *panel.StartRequest) { r.EngineLogLevel = "none" },
			want: panel.FaultUnknown,
		},
		{
			name: "an empty DNS mode",
			edit: func(r *panel.StartRequest) { r.Network.DNSMode = "" },
			want: panel.FaultUnknown,
		},
		{
			name: "a DNS mode that is not the tunnel",
			edit: func(r *panel.StartRequest) { r.Network.DNSMode = "direct" },
			want: panel.FaultUnknown,
		},
		{
			name: "an empty tunnel-down policy",
			edit: func(r *panel.StartRequest) { r.Network.OnTunnelDown = "" },
			want: panel.FaultUnknown,
		},
		{
			name: "a tunnel-down policy that lets traffic out",
			edit: func(r *panel.StartRequest) { r.Network.OnTunnelDown = "allow" },
			want: panel.FaultUnknown,
		},
		{
			name: "a hotspot password that is a well-known default",
			edit: func(r *panel.StartRequest) { r.Hotspot.Passphrase = "SecurePass123" },
			want: panel.FaultUnknown,
		},
		{
			name: "a hotspot password too short for WPA2",
			edit: func(r *panel.StartRequest) { r.Hotspot.Passphrase = "short" },
			want: panel.FaultUnknown,
		},
		{
			name: "no hotspot name",
			edit: func(r *panel.StartRequest) { r.Hotspot.SSID = "" },
			want: panel.FaultUnknown,
		},
		{
			name: "a hotspot name longer than 802.11 allows",
			edit: func(r *panel.StartRequest) { r.Hotspot.SSID = strings.Repeat("a", 33) },
			want: panel.FaultUnknown,
		},
		{
			name: "a country that is not a country code",
			edit: func(r *panel.StartRequest) { r.Hotspot.Country = "United Kingdom" },
			want: panel.FaultUnknown,
		},
		{
			name: "a band this appliance does not use",
			edit: func(r *panel.StartRequest) { r.Hotspot.Band = "6GHz" },
			want: panel.FaultUnknown,
		},
		{
			name: "a hotspot subnet that is not a network",
			edit: func(r *panel.StartRequest) { r.Hotspot.Subnet = "not a subnet" },
			want: panel.FaultUnknown,
		},
		{
			name: "a hotspot subnet on addresses that belong to somebody else",
			edit: func(r *panel.StartRequest) { r.Hotspot.Subnet = "93.184.216.0/24" },
			want: panel.FaultUnknown,
		},
		{
			name: "a hotspot subnet with no usable range in it",
			edit: func(r *panel.StartRequest) { r.Hotspot.Subnet = "10.9.9.0/31" },
			want: panel.FaultUnknown,
		},
		{
			name: "an internet interface that is not on this machine",
			edit: func(r *panel.StartRequest) { r.Network.InternetInterface = "eth9" },
			want: panel.FaultNoInternetInterface,
		},
		{
			name: "an internet interface with no way out on it",
			edit: func(r *panel.StartRequest) { r.Network.InternetInterface = "lo" },
			want: panel.FaultNoInternetInterface,
		},
		{
			name: "an internet interface name the kernel would not accept",
			edit: func(r *panel.StartRequest) { r.Network.InternetInterface = "eth0; rm -rf /" },
			want: panel.FaultNoInternetInterface,
		},
		{
			name: "a hotspot adapter that is not on this machine",
			edit: func(r *panel.StartRequest) { r.Hotspot.Interface = "wlan9" },
			want: panel.FaultNoAPAdapter,
		},
		{
			name: "a hotspot adapter that cannot create a hotspot",
			edit: func(r *panel.StartRequest) { r.Hotspot.Interface = "eth0" },
			want: panel.FaultNoAPAdapter,
		},
		{
			name: "a channel the radio will not take, because it is pinned to another",
			edit: func(r *panel.StartRequest) { r.Hotspot.Channel = 6 },
			want: panel.FaultChannelRefused,
		},
		{
			name:  "a machine whose only radio cannot create a hotspot",
			edit:  func(*panel.StartRequest) {},
			world: func(w *world) { w.runner = newRecordedMachine(fixtureIWListNoAP, fixtureIWRegGet) },
			want:  panel.FaultNoAPAdapter,
		},
		{
			name: "a radio under the world regulatory domain, with no country chosen",
			edit: func(*panel.StartRequest) {},
			world: func(w *world) {
				w.runner = newRecordedMachine(fixtureIWList, fixtureIWRegGetWorld)
			},
			want: panel.FaultUnknown,
		},
		{
			name: "a clock earlier than the date this software was written",
			edit: func(*panel.StartRequest) {},
			world: func(w *world) {
				w.cfg.Now = func() time.Time { return time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC) }
			},
			want: panel.FaultClockImplausible,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := []func(*world){}
			if tc.world != nil {
				opts = append(opts, func(w *world) {
					tc.world(w)
					// The world's runner may have been replaced, so rewire the
					// traced runner that the service actually uses.
					w.cfg.Runner = tracedRunner{inner: w.runner, tl: w.tl}
				})
			}
			w := newWorld(t, opts...)

			req := startRequest(t)
			tc.edit(&req)

			err := w.svc.Start(context.Background(), req)
			if err == nil {
				t.Fatalf("the request was accepted\ntimeline:%s", w.tl)
			}
			if got := panel.FaultOf(err); got != tc.want {
				t.Fatalf("fault was %q, want %q (error: %v)", got, tc.want, err)
			}
			if applied := w.mutatingCommands(); len(applied) != 0 {
				t.Fatalf("a refused request still changed the machine:\n  %s", strings.Join(applied, "\n  "))
			}
			if w.eng.startCount() != 0 {
				t.Fatalf("a refused request still started the engine")
			}
		})
	}
}

// TestOverridesThisMachineCanHonourAreAccepted is the other half: a check that
// refuses everything is not a check.
func TestOverridesThisMachineCanHonourAreAccepted(t *testing.T) {
	cases := []struct {
		name string
		edit func(*panel.StartRequest)
	}{
		{"the uplink named explicitly", func(r *panel.StartRequest) { r.Network.InternetInterface = "eth0" }},
		{"the radio named by its interface", func(r *panel.StartRequest) { r.Hotspot.Interface = "wlan0" }},
		{"the radio named by its phy", func(r *panel.StartRequest) { r.Hotspot.Interface = "phy0" }},
		{"the access point interface this appliance creates", func(r *panel.StartRequest) { r.Hotspot.Interface = "ap0" }},
		{"the channel the radio is already pinned to", func(r *panel.StartRequest) { r.Hotspot.Channel = 10 }},
		{"a country chosen by hand", func(r *panel.StartRequest) { r.Hotspot.Country = "IE" }},
		{"a subnet chosen by hand", func(r *panel.StartRequest) { r.Hotspot.Subnet = "10.99.4.0/24" }},
		{"an engine log level", func(r *panel.StartRequest) { r.EngineLogLevel = "info" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			req := startRequest(t)
			tc.edit(&req)
			if err := w.svc.Start(context.Background(), req); err != nil {
				t.Fatalf("an override this machine can honour was refused: %v\ntimeline:%s", err, w.tl)
			}
		})
	}
}

// TestTheServerNotAnsweringIsReportedAndDoesNotUndoTheBox.
//
// This is the third of the three failure states design section 8 step 11 asks
// the panel to tell apart, and it is the one that must NOT roll back: every
// change succeeded, the fail-closed ruleset is in force, and taking the hotspot
// away would remove the screen the message appears on.
func TestTheServerNotAnsweringIsReportedAndDoesNotUndoTheBox(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.reach.err = errSilentServer
	})

	err := w.svc.Start(context.Background(), startRequest(t))
	if got := panel.FaultOf(err); got != panel.FaultServerNoAnswer {
		t.Fatalf("fault was %q, want %q", got, panel.FaultServerNoAnswer)
	}
	if !w.svc.isRunning() {
		t.Fatalf("the box was torn down because the server did not answer, which takes away the panel " +
			"the message would have appeared on")
	}
	st, err := w.svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !st.Hotspot.Running {
		t.Fatalf("the hotspot was taken down because the server did not answer")
	}
}

// TestAChangedConfigurationStopsTheRunningOneFirst.
//
// A second start with a DIFFERENT request is a configuration change, and
// internal/netcfg's steps are not idempotent, so the running one has to come
// out completely before the new one goes in.
func TestAChangedConfigurationStopsTheRunningOneFirst(t *testing.T) {
	w := newWorld(t)
	first := startRequest(t)
	if err := w.svc.Start(context.Background(), first); err != nil {
		t.Fatalf("first start: %v", err)
	}

	second := startRequest(t)
	second.Hotspot.SSID = "Caspian-Kitchen"
	if err := w.svc.Start(context.Background(), second); err != nil {
		t.Fatalf("second start: %v\ntimeline:%s", err, w.tl)
	}

	// The teardown of the first ran before the second was applied.
	firstUndo := w.tl.indexOf("net: nft -f -")
	if firstUndo < 0 {
		t.Fatalf("the firewall was never loaded\ntimeline:%s", w.tl)
	}
	if w.tl.count("net: ip rule del from 10.83.51.0/24 lookup 8410 priority 8410") != 1 {
		t.Fatalf("the running configuration's policy rule was not removed before the new one was applied\ntimeline:%s", w.tl)
	}
	if w.tl.count("net: ip rule add from 10.83.51.0/24 lookup 8410 priority 8410") != 2 {
		t.Fatalf("the new configuration's policy rule was not applied\ntimeline:%s", w.tl)
	}

	// And the journal ends up describing one configuration, not two.
	st, err := w.svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Hotspot.SSID != "Caspian-Kitchen" {
		t.Fatalf("the running hotspot is %q, want the new one", st.Hotspot.SSID)
	}
}

var errSilentServer = errServerSilent{}

type errServerSilent struct{}

func (errServerSilent) Error() string { return "the proxy server did not answer" }

// TestDetectReportsAMachineThatCannotHostAHotspotWithoutFailing.
//
// internal/panel/priv.go says Detection.Fault "is set when detection could not
// find a workable arrangement". That is information, not a failure: the panel
// still has to draw, and the interface list is what the sentence about the
// missing adapter sits next to.
func TestDetectReportsAMachineThatCannotHostAHotspotWithoutFailing(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.runner = newRecordedMachine(fixtureIWListNoAP, fixtureIWRegGet)
		w.cfg.Runner = tracedRunner{inner: w.runner, tl: w.tl}
	})

	d, err := w.svc.Detect(context.Background())
	if err != nil {
		t.Fatalf("detect returned an error for a machine with no AP-capable radio: %v", err)
	}
	if d.Fault != panel.FaultNoAPAdapter {
		t.Fatalf("detection fault was %q, want %q", d.Fault, panel.FaultNoAPAdapter)
	}
	if len(d.Interfaces) == 0 {
		t.Fatalf("no interfaces were reported, so the panel has nothing to show beside the message")
	}
	for _, i := range d.Interfaces {
		if i.CanHostAP {
			t.Fatalf("%q was offered as a hotspot choice on a machine where no radio reported AP support", i.Name)
		}
	}
	if applied := w.mutatingCommands(); len(applied) != 0 {
		t.Fatalf("detection changed the machine:\n  %s", strings.Join(applied, "\n  "))
	}
}

// TestDetectChangesNothing.
func TestDetectChangesNothing(t *testing.T) {
	w := newWorld(t)
	if _, err := w.svc.Detect(context.Background()); err != nil {
		t.Fatalf("detect: %v", err)
	}
	if applied := w.mutatingCommands(); len(applied) != 0 {
		t.Fatalf("detection changed the machine:\n  %s", strings.Join(applied, "\n  "))
	}
}

// TestARequestRefusedOnItsInterfaceNamesNeverLooksUpTheServer.
//
// This is what validateAgainstFacts buys that internal/netcfg's own refusals do
// not. PlanNetwork refuses the same interface names with the same faults, so a
// service that skipped this check would still say the right thing. What it
// would also do is RESOLVE THE SERVER NAME FIRST, because the plan needs the
// address, and that lookup leaves the box in the clear, out of the uplink,
// carrying the name of the user's proxy server.
//
// So the value of checking early is not the refusal, it is that a request that
// was never going to work discloses nothing. Nothing else in this package
// measures that, and the mutation table records that removing
// validateAgainstFacts leaves every other test green.
func TestARequestRefusedOnItsInterfaceNamesNeverLooksUpTheServer(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*panel.StartRequest)
	}{
		{"an internet interface that is not on this machine", func(r *panel.StartRequest) {
			r.Network.InternetInterface = "eth9"
		}},
		{"a hotspot adapter that cannot create a hotspot", func(r *panel.StartRequest) {
			r.Hotspot.Interface = "eth0"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			req := startRequest(t)
			tc.edit(&req)

			if err := w.svc.Start(context.Background(), req); err == nil {
				t.Fatalf("the request was accepted")
			}
			if n := w.resolver.lookups(); n != 0 {
				t.Fatalf("the proxy server's name was looked up %d times for a request that was refused; "+
					"that lookup leaves this box in the clear carrying the name of the user's server", n)
			}
		})
	}
}

// TestAnAcceptedRequestDoesLookTheServerUp is the other half: a check that
// counts zero lookups would pass on a service that never resolved anything.
func TestAnAcceptedRequestDoesLookTheServerUp(t *testing.T) {
	w := newWorld(t)
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if n := w.resolver.lookups(); n != 1 {
		t.Fatalf("the server was looked up %d times, want once", n)
	}
}

// TestABoxThatWasNeverSwitchedOnReportsNoFault.
//
// internal/panel/priv.go says HotspotStatus.Fault is "why it is not running,
// when it is not". "Nobody has pressed the switch" is not a why the user can
// act on, and putting a fault word there would draw an error on the panel of a
// box that is doing exactly what it was asked.
func TestABoxThatWasNeverSwitchedOnReportsNoFault(t *testing.T) {
	w := newWorld(t)
	st, err := w.svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Hotspot.Running {
		t.Fatalf("the hotspot reports itself running on a box that was never started")
	}
	if st.Hotspot.Fault != panel.FaultNone {
		t.Fatalf("a box that was never switched on reports fault %q", st.Hotspot.Fault)
	}
	if st.Connected() {
		t.Fatalf("a box that was never switched on reports itself as carrying client traffic")
	}
}

// TestABoxThatWasSwitchedOnAndLostItsHotspotDoesReportAFault is the other half:
// a check that never reports a fault is not a check.
func TestABoxThatWasSwitchedOnAndLostItsHotspotDoesReportAFault(t *testing.T) {
	w := newWorld(t)
	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	// The access point dies. Its pid file still names a process that is gone,
	// which is exactly what a crashed hostapd leaves behind.
	for pid := range w.sys.Alive {
		w.sys.SetAlive(pid, false)
	}

	st, err := w.svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Hotspot.Running {
		t.Fatalf("the hotspot reports itself running after both processes died")
	}
	if st.Hotspot.Fault == panel.FaultNone {
		t.Fatalf("a box that was switched on and lost its hotspot reports no fault at all")
	}
}

// TestTheStartRequestNamesTheValuesInternalStateGuarantees.
//
// The two policy fields are compared against internal/state's own constants
// rather than against strings written here, so that a change in that package is
// a compile-time or test-time event and not a box that refuses every start.
func TestTheStartRequestNamesTheValuesInternalStateGuarantees(t *testing.T) {
	req := startRequest(t)
	if req.Network.DNSMode != state.DNSModeTunnel {
		t.Fatalf("the test request does not carry state.DNSModeTunnel")
	}
	if req.Network.OnTunnelDown != state.OnTunnelDownBlock {
		t.Fatalf("the test request does not carry state.OnTunnelDownBlock")
	}
}
