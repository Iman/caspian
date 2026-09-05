// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/state"
)

// ---------------------------------------------------------------------------
// What these tests can and cannot prove
//
// FakePrivileged answers exactly the way the panel asks it to, because that is
// what a fake is. So nothing in this file is evidence about the real privileged
// service: it cannot be, since that service does not exist yet and this package
// is not allowed to implement it.
//
// What they do prove is the part that is this package's to get right: that the
// vocabulary is closed, that the arguments are typed rather than composed, that
// a credential in a request does not print, and that the panel behaves
// correctly given each answer the boundary can return. Whether a real service
// returns those answers is a claim for whoever writes it, checked against a
// real box, and it is not checked here.
// ---------------------------------------------------------------------------

// TestActionVocabularyMatchesTheInterface keeps the wire vocabulary and the Go
// interface from drifting apart.
//
// The reflection is the point. A test listing the five names by hand would pass
// after somebody added a sixth method, and the socket protocol on the other
// side would have no name for it.
func TestActionVocabularyMatchesTheInterface(t *testing.T) {
	iface := reflect.TypeOf((*Privileged)(nil)).Elem()

	if iface.NumMethod() != len(Actions) {
		var names []string
		for i := 0; i < iface.NumMethod(); i++ {
			names = append(names, iface.Method(i).Name)
		}
		t.Fatalf("Privileged has %d methods %v but Actions lists %d names %v; every action needs a name on the wire",
			iface.NumMethod(), names, len(Actions), Actions)
	}

	seen := map[Action]bool{}
	for _, a := range Actions {
		if a == "" {
			t.Error("an action has an empty name")
		}
		if seen[a] {
			t.Errorf("action %q is listed twice", a)
		}
		seen[a] = true
	}
}

// TestNoPrivilegedActionTakesACommand is the design rule read back off the
// types: "The privileged side accepts a short list of named actions and never a
// command built from anything the user typed. A privileged helper that takes a
// path and an argument list from its client is not a boundary; it is a way to
// run anything as root."
//
// It walks the argument types looking for the shapes that would be one: a field
// named like a command, a path or an argument list, and any []string, which is
// what an argv is.
func TestNoPrivilegedActionTakesACommand(t *testing.T) {
	iface := reflect.TypeOf((*Privileged)(nil)).Elem()

	banned := []string{"cmd", "command", "argv", "args", "arguments", "exec", "shell", "script", "path", "binary", "program"}

	var walk func(t reflect.Type, path string, depth int)
	walk = func(rt reflect.Type, path string, depth int) {
		if depth > 6 {
			return
		}
		switch rt.Kind() {
		case reflect.Struct:
			for i := 0; i < rt.NumField(); i++ {
				f := rt.Field(i)
				name := strings.ToLower(f.Name)
				for _, b := range banned {
					if name == b {
						t.Errorf("%s.%s is named like a command or a path; the boundary must not carry one", path, f.Name)
					}
				}
				// A []string argument is an argv in every shape that has ever
				// been a privilege-escalation bug. []byte is fine: that is the
				// config document, which design section 6 sanctions.
				if f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.String {
					t.Errorf("%s.%s is a []string, which is the shape of an argument list", path, f.Name)
				}
				walk(f.Type, path+"."+f.Name, depth+1)
			}
		case reflect.Slice, reflect.Ptr:
			walk(rt.Elem(), path+"[]", depth+1)
		}
	}

	checked := 0
	for i := 0; i < iface.NumMethod(); i++ {
		m := iface.Method(i)
		for j := 0; j < m.Type.NumIn(); j++ {
			in := m.Type.In(j)
			if in == reflect.TypeOf((*context.Context)(nil)).Elem() {
				continue
			}
			checked++
			walk(in, m.Name+"("+in.Name()+")", 0)
		}
	}
	if checked == 0 {
		t.Fatal("no argument types were inspected, so this test checked nothing")
	}
}

// TestFaultOf covers the classification the panel's wording depends on.
func TestFaultOf(t *testing.T) {
	if got := FaultOf(nil); got != FaultNone {
		t.Errorf("FaultOf(nil) = %q, want FaultNone", got)
	}
	if got := FaultOf(faultErr(FaultServerNoAnswer)); got != FaultServerNoAnswer {
		t.Errorf("FaultOf = %q, want %q", got, FaultServerNoAnswer)
	}
	// Wrapped, since a real client will wrap.
	wrapped := errors.Join(errors.New("reading from the socket"), faultErr(FaultRadioBlocked))
	if got := FaultOf(wrapped); got != FaultRadioBlocked {
		t.Errorf("FaultOf on a wrapped error = %q, want %q", got, FaultRadioBlocked)
	}
	// A timeout is the service not answering, not an unclassified failure.
	if got := FaultOf(context.DeadlineExceeded); got != FaultUnavailable {
		t.Errorf("FaultOf(DeadlineExceeded) = %q, want %q", got, FaultUnavailable)
	}
	// Anything else is unknown rather than being forced into a category.
	if got := FaultOf(errors.New("something else entirely")); got != FaultUnknown {
		t.Errorf("FaultOf on a plain error = %q, want %q", got, FaultUnknown)
	}
}

func TestValidEngineLogLevel(t *testing.T) {
	for _, ok := range []string{"", "error", "warning", "info", "debug"} {
		if !ValidEngineLogLevel(ok) {
			t.Errorf("ValidEngineLogLevel(%q) = false", ok)
		}
	}
	for _, bad := range []string{"none", "trace", "verbose", "DEBUG", "warn"} {
		if ValidEngineLogLevel(bad) {
			t.Errorf("ValidEngineLogLevel(%q) = true", bad)
		}
	}
}

// TestPanelStillDrawsWhenThePrivilegedServiceIsDown is the availability half of
// design section 5.7's argument, applied one layer in: the panel is needed most
// when the box is broken, so it has to render when the thing it talks to is not
// answering.
func TestPanelStillDrawsWhenThePrivilegedServiceIsDown(t *testing.T) {
	h := newHarness(t)
	h.ready()
	h.priv.FailStatusWith(FaultUnavailable)

	res, body := h.get("/")
	if res.StatusCode != 200 {
		t.Fatalf("GET / with the privileged service down: status %d", res.StatusCode)
	}
	if want := h.msg(MsgFaultUnavailable); !strings.Contains(body, want) {
		t.Error("the page does not say the background service is not answering")
	}
	// The hotspot name and password are read from state, not from the
	// privileged side, so they must still be on the page: they are what the
	// user needs to get a device onto the box and look at this panel.
	if !strings.Contains(body, "Caspian-test") {
		t.Error("the hotspot name is missing when the privileged service is down")
	}
	if !strings.Contains(body, "<svg") {
		t.Error("the join code is missing when the privileged service is down")
	}

	// Advanced mode too, which additionally asks for the engine log.
	res, _ = h.get("/?advanced=1")
	if res.StatusCode != 200 {
		t.Errorf("advanced mode with the privileged service down: status %d", res.StatusCode)
	}
}

// TestSwitchingOnSendsTypedArgumentsNotStrings checks what actually crosses the
// boundary.
func TestSwitchingOnSendsTypedArgumentsNotStrings(t *testing.T) {
	h := newHarness(t)
	h.ready()

	// An override, so the request carries something other than defaults.
	if err := h.store.Update(func(st *state.State) error {
		st.Advanced.HotspotInterface = "wlan0"
		st.Advanced.InternetInterface = "eth0"
		st.Advanced.Channel = 6
		st.Advanced.EngineLogLevel = "info"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if res, _ := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}}); res.StatusCode != 303 {
		t.Fatal("could not switch on")
	}
	starts := h.priv.Starts()
	if len(starts) != 1 {
		t.Fatalf("%d start requests, want 1", len(starts))
	}
	req := starts[0]

	if req.Hotspot.SSID != "Caspian-test" {
		t.Errorf("SSID = %q", req.Hotspot.SSID)
	}
	if req.Hotspot.Passphrase != "sun-rope-glass-mint" {
		t.Error("the hotspot passphrase did not reach the privileged side")
	}
	if req.Hotspot.Interface != "wlan0" || req.Hotspot.Channel != 6 {
		t.Errorf("the hotspot overrides did not reach the privileged side: %+v", req.Hotspot)
	}
	if req.Network.InternetInterface != "eth0" {
		t.Errorf("the uplink override did not reach the privileged side: %+v", req.Network)
	}
	if req.EngineLogLevel != "info" {
		t.Errorf("EngineLogLevel = %q", req.EngineLogLevel)
	}

	// The policy fields must never cross empty: internal/state refuses to
	// persist an empty one because empty must not be readable as "let client
	// traffic out", and the same reasoning applies on the wire.
	if req.Network.DNSMode == "" {
		t.Error("DNSMode crossed the boundary empty")
	}
	if req.Network.OnTunnelDown == "" {
		t.Error("OnTunnelDown crossed the boundary empty")
	}
	if req.Network.DNSMode != state.DNSModeTunnel {
		t.Errorf("DNSMode = %q, want the fail-closed default", req.Network.DNSMode)
	}
	if req.Network.OnTunnelDown != state.OnTunnelDownBlock {
		t.Errorf("OnTunnelDown = %q, want the fail-closed default", req.Network.OnTunnelDown)
	}

	// And the config document is a document, not the pasted text.
	if !strings.HasPrefix(strings.TrimSpace(string(req.ConfigJSON)), "{") {
		t.Errorf("the config document is not JSON: %.40s", req.ConfigJSON)
	}
}

// TestSwitchingOffAsksTheServiceToStop covers the other half of the switch.
func TestSwitchingOffAsksTheServiceToStop(t *testing.T) {
	h := newHarness(t)
	h.ready()

	if res, _ := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}}); res.StatusCode != 303 {
		t.Fatal("could not switch on")
	}
	if res, _ := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"0"}}); res.StatusCode != 303 {
		t.Fatal("could not switch off")
	}
	if h.priv.Stops() != 1 {
		t.Errorf("the privileged service was asked to stop %d times, want 1", h.priv.Stops())
	}
	_, body := h.get("/")
	if want := h.msg(MsgNoticeOff); !strings.Contains(body, want) {
		t.Error("the page does not confirm it was switched off")
	}
}

// TestSwitchingOnRefusesWithoutAConfigOrAHotspot checks the two preconditions
// are reported as themselves rather than as a connection failure.
func TestSwitchingOnRefusesWithoutAConfigOrAHotspot(t *testing.T) {
	t.Run("no config", func(t *testing.T) {
		h := newHarness(t)
		h.setup(testPassword)
		if err := h.store.SetHotspot("Caspian-test", "sun-rope-glass-mint"); err != nil {
			t.Fatal(err)
		}
		h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
		_, body := h.get("/")
		if want := h.msg(MsgNoConfigYet); !strings.Contains(body, want) {
			t.Error("the page does not say there is no config")
		}
		if len(h.priv.Starts()) != 0 {
			t.Error("the privileged service was asked to start with no config")
		}
	})

	t.Run("no hotspot name", func(t *testing.T) {
		h := newHarness(t)
		h.setup(testPassword)
		if err := h.store.SetProxyConfig(testLink(), "vless", ""); err != nil {
			t.Fatal(err)
		}
		h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}})
		_, body := h.get("/")
		if want := h.msg(MsgNoHotspotYet); !strings.Contains(body, want) {
			t.Error("the page does not say the hotspot needs a name")
		}
		if len(h.priv.Starts()) != 0 {
			t.Error("the privileged service was asked to start with no hotspot")
		}
	})
}

// TestAdvancedOverridesAreCheckedAgainstDetection is what stops the advanced
// form being a way to hand an arbitrary string to the privileged process.
func TestAdvancedOverridesAreCheckedAgainstDetection(t *testing.T) {
	h := newHarness(t)
	h.ready()

	bad := []struct {
		name string
		form url.Values
		says Key
	}{
		{
			name: "an interface that does not exist",
			form: url.Values{"internet_interface": {"; rm -rf /"}},
			says: MsgAdvBadInternet,
		},
		{
			name: "a hotspot interface that cannot host one",
			form: url.Values{"hotspot_interface": {"eth0"}},
			says: MsgAdvNoAP,
		},
		{
			name: "a channel the radio will not use",
			form: url.Values{"channel": {"140"}},
			says: MsgAdvBadChannel,
		},
		{
			name: "a channel that is not a number",
			form: url.Values{"channel": {"six"}},
			says: MsgAdvChannelNaN,
		},
		{
			name: "a subnet that is not one",
			form: url.Values{"subnet": {"not-a-subnet"}},
			says: MsgAdvBadSubnet,
		},
		{
			name: "a country that is not a code",
			form: url.Values{"country": {"United Kingdom"}},
			says: MsgAdvBadCountry,
		},
		{
			name: "a log level the engine does not have",
			form: url.Values{"engine_log_level": {"trace"}},
			says: MsgAdvBadLogLevel,
		},
		{
			name: "a band this build does not offer",
			form: url.Values{"band": {"6"}},
			says: MsgAdvBadBand,
		},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			form := url.Values{"csrf": {h.tokenOn("/")}}
			for k, v := range c.form {
				form[k] = v
			}
			if res, _ := h.postForm("/advanced", form); res.StatusCode != 303 {
				t.Fatalf("status %d", res.StatusCode)
			}
			_, body := h.get("/")
			if want := h.msg(c.says); !strings.Contains(body, want) {
				t.Errorf("the page does not say %q", want)
			}
			// Nothing was stored.
			adv := h.store.Advanced()
			if strings.Contains(adv.InternetInterface, "rm -rf") || adv.Channel == 140 ||
				adv.Subnet == "not-a-subnet" || adv.EngineLogLevel == "trace" {
				t.Errorf("a refused override was stored anyway: %+v", adv)
			}
		})
	}

	// The control: a legitimate set of overrides is accepted and stored, or
	// every refusal above could be a form that never works.
	form := url.Values{
		"csrf":               {h.tokenOn("/")},
		"internet_interface": {"eth0"},
		"hotspot_interface":  {"wlan0"},
		"channel":            {"6"},
		"country":            {"GB"},
		"subnet":             {"10.62.0.0/24"},
		"engine_log_level":   {"info"},
		"panel_on_lan":       {"1"},
	}
	if res, _ := h.postForm("/advanced", form); res.StatusCode != 303 {
		t.Fatal("a legitimate set of overrides was refused")
	}
	adv := h.store.Advanced()
	if adv.InternetInterface != "eth0" || adv.HotspotInterface != "wlan0" ||
		adv.Channel != 6 || adv.Country != "GB" || adv.Subnet != "10.62.0.0/24" ||
		adv.EngineLogLevel != "info" || !adv.PanelOnLAN {
		t.Errorf("the overrides were not stored: %+v", adv)
	}
	// The policy fields survive an advanced-mode save. internal/state would
	// refuse an empty one, so this is really checking that the handler does not
	// blank them on the way through.
	if adv.DNSMode == "" || adv.OnTunnelDown == "" {
		t.Errorf("saving the advanced form emptied a policy field: %+v", adv)
	}
	// The basic connection form changes only its three fields, preserving
	// advanced settings that the user cannot see in that form.
	connectionForm := url.Values{
		"csrf": {h.tokenOn("/")}, "connections_only": {"1"},
		"internet_interface": {"eth0"}, "hotspot_interface": {"wlan0"},
		"band": {"2.4GHz"},
	}
	h.postForm("/advanced", connectionForm)
	got := h.store.Advanced()
	adv.Band = "2.4GHz"
	if got != adv {
		t.Errorf("connection form changed unrelated settings: got %+v, want %+v", got, adv)
	}
}

// TestSettingTheHotspotIsValidated covers the other form that writes state.
func TestSettingTheHotspotIsValidated(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)

	for _, c := range []struct {
		name       string
		ssid, pass string
		says       Key
	}{
		{"no name", "", "sun-rope-glass-mint", MsgSSIDMissing},
		{"name too long", strings.Repeat("n", 33), "sun-rope-glass-mint", MsgSSIDTooLong},
		{"password too short", "Caspian-test", "short", MsgPassTooShort},
		{"a known default", "Caspian-test", "SecurePass123", MsgPassRefused},
	} {
		t.Run(c.name, func(t *testing.T) {
			h.postForm("/hotspot", url.Values{
				"csrf": {h.tokenOn("/")}, "ssid": {c.ssid}, "passphrase": {c.pass},
			})
			_, body := h.get("/")
			if want := h.msg(c.says); !strings.Contains(body, want) {
				t.Errorf("the page does not say %q", want)
			}
			if h.store.Hotspot().SSID == c.ssid && c.ssid != "" {
				t.Error("a refused hotspot name was stored anyway")
			}
		})
	}

	// The control.
	if res, _ := h.postForm("/hotspot", url.Values{
		"csrf": {h.tokenOn("/")}, "ssid": {"Caspian-test"}, "passphrase": {"sun-rope-glass-mint"},
	}); res.StatusCode != 303 {
		t.Fatal("a legitimate hotspot name and password were refused")
	}
	if got := h.store.Hotspot(); got.SSID != "Caspian-test" || got.Passphrase.Reveal() != "sun-rope-glass-mint" {
		t.Errorf("the hotspot settings were not stored: %v", got)
	}
}
