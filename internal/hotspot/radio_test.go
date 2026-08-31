// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"strings"
	"testing"
)

// TestPinnedChannel encodes the measurement in docs/2026-08-29-design.md
// section 4.6: the Raspberry Pi 5's built-in radio reports "#{ AP } <= 1" and
// "#channels <= 1", so when it also holds a client link the access point has
// no channel of its own.
func TestPinnedChannelConstraint(t *testing.T) {
	pi5 := RadioConstraint{
		SupportsAP:    true,
		MaxAPs:        1,
		MaxChannels:   1,
		ClientChannel: 10,
	}
	if ch, ok := pi5.PinnedChannel(); !ok || ch != 10 {
		t.Fatalf("PinnedChannel() = %d, %v; want 10, true", ch, ok)
	}

	cfg := testAP()
	cfg.Channel = 10
	if err := pi5.Check(cfg); err != nil {
		t.Errorf("the pinned channel was refused: %v", err)
	}

	cfg.Channel = 6
	err := pi5.Check(cfg)
	if err == nil {
		t.Fatal("a channel other than the pinned one was accepted")
	}
	// The message has to tell a non-technical user what happened and what the
	// channel will be, not just that something is wrong.
	for _, want := range []string{"channel 10", "internet connection"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message does not mention %q: %v", want, err)
		}
	}
}

func TestRadioWithNoClientLinkIsFree(t *testing.T) {
	// A USB adapter with no client link of its own: any valid channel goes.
	usb := RadioConstraint{SupportsAP: true, MaxAPs: 1, MaxChannels: 1, ClientChannel: 0}
	if _, ok := usb.PinnedChannel(); ok {
		t.Error("a radio with no client link reported a pinned channel")
	}
	cfg := testAP()
	cfg.Channel = 6
	if err := usb.Check(cfg); err != nil {
		t.Errorf("channel 6 was refused on a radio with no client link: %v", err)
	}
}

func TestRadioWithTwoChannelsIsFree(t *testing.T) {
	dual := RadioConstraint{SupportsAP: true, MaxAPs: 2, MaxChannels: 2, ClientChannel: 10}
	cfg := testAP()
	cfg.Channel = 6
	if err := dual.Check(cfg); err != nil {
		t.Errorf("a radio that can hold two channels refused a second one: %v", err)
	}
}

func TestRadioWithoutAPSupportIsRefusedInPlainWords(t *testing.T) {
	none := RadioConstraint{SupportsAP: false}
	err := none.Check(testAP())
	if err == nil {
		t.Fatal("an adapter that cannot host an access point was accepted")
	}
	// docs/2026-08-29-design.md section 5.2: "No adapter on this machine can
	// create a hotspot. Plug in a USB WiFi adapter" is correct, "No AP-capable
	// phy" is not.
	msg := err.Error()
	if !strings.Contains(msg, "USB WiFi adapter") {
		t.Errorf("the message does not tell the user what to do: %q", msg)
	}
	for _, jargon := range []string{"phy", "nl80211", "errno", "AP-capable"} {
		if strings.Contains(msg, jargon) {
			t.Errorf("the message contains jargon %q: %q", jargon, msg)
		}
	}
}

func TestAllowedChannelsMembership(t *testing.T) {
	rc := RadioConstraint{SupportsAP: true, MaxAPs: 1, MaxChannels: 4, AllowedChannels: []int{1, 6, 11}}
	cfg := testAP()
	cfg.Channel = 6
	if err := rc.Check(cfg); err != nil {
		t.Errorf("an allowed channel was refused: %v", err)
	}
	cfg.Channel = 10
	if err := rc.Check(cfg); err == nil {
		t.Error("a channel outside the allowed set was accepted")
	}

	// An empty set means the caller did not measure it, and nothing is
	// checked rather than something being guessed.
	rc.AllowedChannels = nil
	if err := rc.Check(cfg); err != nil {
		t.Errorf("an unmeasured allowed set was treated as empty: %v", err)
	}
}

func TestParseRfkillList(t *testing.T) {
	out := "0: phy0: Wireless LAN\n" +
		"\tSoft blocked: yes\n" +
		"\tHard blocked: no\n" +
		"1: hci0: Bluetooth\n" +
		"\tSoft blocked: no\n" +
		"\tHard blocked: no\n"

	devs := parseRfkillList(out)
	if len(devs) != 2 {
		t.Fatalf("parsed %d devices, want 2: %+v", len(devs), devs)
	}
	if devs[0].Name != "phy0" || devs[0].Type != "Wireless LAN" {
		t.Errorf("device 0 = %+v", devs[0])
	}
	if !devs[0].SoftBlocked || devs[0].HardBlocked {
		t.Errorf("device 0 block state = %+v", devs[0])
	}

	// Only the wireless device counts. A soft-blocked Bluetooth radio must
	// not stop the hotspot.
	wireless := wirelessDevices(devs)
	if len(wireless) != 1 || wireless[0].Name != "phy0" {
		t.Errorf("wirelessDevices = %+v", wireless)
	}
}

func TestParseRfkillListTolerance(t *testing.T) {
	// Unrecognised lines are ignored rather than failing: a future rfkill that
	// adds a field must not stop the hotspot from starting.
	out := "0: phy0: Wireless LAN\n" +
		"\tSoft blocked: no\n" +
		"\tHard blocked: no\n" +
		"\tSomething new: 42\n" +
		"garbage without a colon\n"
	devs := parseRfkillList(out)
	if len(devs) != 1 || devs[0].SoftBlocked || devs[0].HardBlocked {
		t.Errorf("parseRfkillList = %+v", devs)
	}
	if got := parseRfkillList(""); len(got) != 0 {
		t.Errorf("empty output parsed as %+v", got)
	}
}

func TestRadioStateWording(t *testing.T) {
	hard := radioStateFrom([]RfkillDevice{{Name: "phy0", HardBlocked: true}}, false)
	if !hard.HardBlocked {
		t.Fatal("a hard block was not reported")
	}
	if strings.Contains(hard.Detail, "rfkill") || strings.Contains(hard.Detail, "hard block") {
		t.Errorf("the hard block message uses jargon: %q", hard.Detail)
	}
	if !strings.Contains(hard.Detail, "switch") {
		t.Errorf("the hard block message does not say what to do: %q", hard.Detail)
	}

	fixed := radioStateFrom([]RfkillDevice{{Name: "phy0"}}, true)
	if fixed.SoftBlocked || fixed.HardBlocked {
		t.Errorf("an unblocked radio still reports a block: %+v", fixed)
	}
	if !fixed.Unblocked || fixed.Detail == "" {
		t.Errorf("the status does not say the adapter had to be switched on: %+v", fixed)
	}
}
