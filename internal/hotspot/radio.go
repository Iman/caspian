// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// RadioConstraint is what the radio reported, as measured by internal/netcfg.
//
// This package never asks the radio anything. It is given the answer and its
// job is to refuse a configuration the radio cannot run, before hostapd is
// started and fails in a way the user cannot read.
//
// The field names follow the shape of `iw phy` output because that is where
// the caller gets them. On the Raspberry Pi 5's built-in radio the measured
// values recorded in docs/2026-08-29-design.md section 4.6 are
// "#{ AP } <= 1" and "#channels <= 1", which map to MaxAPs=1 and
// MaxChannels=1: an access point can run beside the existing client link, but
// it is pinned to that link's channel.
type RadioConstraint struct {
	// SupportsAP is whether the interface can be an access point at all. A
	// USB adapter that cannot is the case the panel must explain in words.
	SupportsAP bool

	// MaxAPs is how many access points this radio can host at once.
	MaxAPs int

	// MaxChannels is how many distinct channels this radio can be on at once.
	// 1 means an access point must share the channel of any existing link.
	MaxChannels int

	// ClientChannel is the channel the radio's client link is currently on,
	// or 0 when the radio holds no client link. When MaxChannels is 1 and
	// this is non-zero, it is the only channel the access point may use.
	ClientChannel int

	// AllowedChannels, when non-empty, is the set the regulatory domain
	// permits. Empty means the caller did not measure it, and no membership
	// check is made rather than a guessed one.
	AllowedChannels []int
}

// PinnedChannel reports the channel the access point has no choice about, if
// there is one. The panel uses it to explain why the channel cannot be changed.
func (rc RadioConstraint) PinnedChannel() (int, bool) {
	if rc.MaxChannels <= 1 && rc.ClientChannel != 0 {
		return rc.ClientChannel, true
	}
	return 0, false
}

// Check reports whether this radio can run the given access point.
func (rc RadioConstraint) Check(cfg APConfig) error {
	if !rc.SupportsAP {
		return errors.New("hotspot: this wireless adapter cannot create a hotspot; " +
			"plug in a USB WiFi adapter that supports one")
	}
	if rc.MaxAPs < 1 {
		return errors.New("hotspot: this wireless adapter reports it can host no access point; " +
			"plug in a USB WiFi adapter that supports one")
	}
	if pinned, ok := rc.PinnedChannel(); ok && cfg.Channel != pinned {
		// The failure this prevents: hostapd is asked for channel N while the
		// same radio holds a client link on channel M. The driver either
		// refuses to start the AP or moves the client link, dropping the box's
		// own internet. Neither is readable from the outside.
		return fmt.Errorf("hotspot: this radio can only be on one channel at a time and its "+
			"internet connection is on channel %d, so the hotspot must use channel %d, not %d",
			pinned, pinned, cfg.Channel)
	}
	if len(rc.AllowedChannels) > 0 {
		found := false
		for _, c := range rc.AllowedChannels {
			if c == cfg.Channel {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("hotspot: channel %d is not one this adapter is allowed to use in %s (allowed: %v)",
				cfg.Channel, cfg.CountryCode, rc.AllowedChannels)
		}
	}
	return nil
}

// RfkillDevice is one line group of `rfkill list` output.
type RfkillDevice struct {
	Index       int
	Name        string // for example "phy0"
	Type        string // for example "Wireless LAN"
	SoftBlocked bool
	HardBlocked bool
}

// RadioState is what the supervisor found and did about rfkill.
type RadioState struct {
	// Present is whether any wireless rfkill device was listed at all.
	Present bool
	// SoftBlocked is the state after any unblock this program performed.
	SoftBlocked bool
	// HardBlocked is a physical or firmware block. Software cannot clear it.
	HardBlocked bool
	// Unblocked is true when this program cleared a soft block during start.
	Unblocked bool
	// Detail is a sentence for the panel, empty when there is nothing to say.
	Detail string
}

// parseRfkillList parses the classic `rfkill list` output.
//
// The format has been stable across the standalone rfkill and the util-linux
// one that Raspberry Pi OS ships:
//
//	0: phy0: Wireless LAN
//		Soft blocked: yes
//		Hard blocked: no
//
// The human format is parsed rather than the JSON one on purpose: --json is a
// util-linux addition and this has to work on whatever rfkill the box has.
// Unrecognised lines are ignored rather than treated as an error, because a
// future rfkill adding a field must not stop the hotspot from starting.
func parseRfkillList(out string) []RfkillDevice {
	var devs []RfkillDevice
	var cur *RfkillDevice
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if idx, name, typ, ok := parseRfkillHeader(line); ok {
			devs = append(devs, RfkillDevice{Index: idx, Name: name, Type: typ})
			cur = &devs[len(devs)-1]
			continue
		}
		if cur == nil {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "soft blocked":
			cur.SoftBlocked = strings.EqualFold(val, "yes")
		case "hard blocked":
			cur.HardBlocked = strings.EqualFold(val, "yes")
		}
	}
	return devs
}

// parseRfkillHeader matches "0: phy0: Wireless LAN".
func parseRfkillHeader(line string) (index int, name, typ string, ok bool) {
	first, rest, found := strings.Cut(line, ":")
	if !found {
		return 0, "", "", false
	}
	idx, err := strconv.Atoi(strings.TrimSpace(first))
	if err != nil {
		return 0, "", "", false
	}
	name, typ, found = strings.Cut(strings.TrimSpace(rest), ":")
	if !found {
		return 0, "", "", false
	}
	return idx, strings.TrimSpace(name), strings.TrimSpace(typ), true
}

// wirelessDevices keeps the entries rfkill calls wireless.
func wirelessDevices(devs []RfkillDevice) []RfkillDevice {
	var out []RfkillDevice
	for _, d := range devs {
		t := strings.ToLower(d.Type)
		if strings.Contains(t, "wlan") || strings.Contains(t, "wireless") {
			out = append(out, d)
		}
	}
	return out
}
