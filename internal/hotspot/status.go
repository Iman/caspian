// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"fmt"
	"strings"
)

// ProcState is what one of the two processes is doing.
type ProcState struct {
	// Running is whether the process exists.
	Running bool
	// PID is its process id, 0 when it is not running.
	PID int
	// Beaconing is meaningful for the access point only: hostapd is running
	// AND its control interface reports the AP is enabled. A hostapd that is
	// alive and beaconing nothing is the failure a process check cannot see.
	Beaconing bool
	// Detail is a sentence about this process, empty when there is nothing
	// to say.
	Detail string
}

// Status is what the panel shows.
type Status struct {
	// Running is true only when both processes are up and the access point is
	// actually beaconing. A half-started hotspot is not running.
	Running bool

	// reasonSSID is the network name the access point reports itself as
	// broadcasting, where the platform can say (Windows reports it directly).
	// Unexported: the panel takes the name from the plan, and this is only
	// for a driver to tell "on with our name" from "on with somebody else's".
	reasonSSID string

	// AccessPoint and DHCP are the two processes.
	AccessPoint ProcState
	DHCP        ProcState

	// Radio is what rfkill said, and what this program did about it.
	Radio RadioState

	// Devices is the list of devices holding a live DHCP lease.
	Devices []Lease

	// MalformedLeaseLines counts lease file lines that could not be read, so
	// the panel can say the count may be short instead of quietly under-
	// reporting.
	MalformedLeaseLines int

	// Reason is why the hotspot is not running, in words a non-technical
	// person can act on. Empty when Running is true.
	Reason string
}

// DeviceCount is what the panel puts next to "devices connected".
func (s Status) DeviceCount() int { return len(s.Devices) }

// explainFailure turns a process failure into a sentence the user can act on.
//
// The rule this follows: name the thing that is wrong and the action that
// fixes it, in the words the user would use. "No AP-capable phy" is not an
// error message for this audience (docs/2026-08-29-design.md section 5.2).
//
// unit is "hotspot" or "DHCP and DNS server" and appears in the fallback.
// Matching is on the message text hostapd and dnsmasq actually print, so this
// function is a lookup table that will need adding to as new failures are
// seen. It never invents a cause: when nothing matches, the last line says
// plainly that the reason is not understood and hands over the raw text.
func explainFailure(unit string, exitCode int, stderr, stdout string) string {
	text := strings.ToLower(stderr + "\n" + stdout)

	switch {
	case strings.Contains(text, "rfkill") && strings.Contains(text, "block"):
		return "The wireless adapter is switched off. Caspian tried to switch it back on and could not."

	case strings.Contains(text, "could not set channel"),
		strings.Contains(text, "failed to set channel"):
		return "The wireless adapter would not accept the channel it was given. " +
			"If this machine is also using WiFi for its internet connection, the hotspot has to " +
			"use the same channel as that connection."

	case strings.Contains(text, "nl80211: could not configure driver mode"),
		strings.Contains(text, "does not support ap mode"),
		strings.Contains(text, "driver does not support"):
		return "This wireless adapter cannot create a hotspot. Plug in a USB WiFi adapter that supports one."

	case strings.Contains(text, "interface initialization failed"),
		strings.Contains(text, "could not read interface"),
		strings.Contains(text, "no such device"):
		return "The wireless adapter Caspian was told to use is not there any more. " +
			"If it is a USB adapter, unplug it and plug it back in."

	case strings.Contains(text, "invalid configuration"),
		strings.Contains(text, "unknown configuration item"),
		strings.Contains(text, "bad option"),
		strings.Contains(text, "unknown option"):
		return "The " + unit + " software on this machine did not understand its settings. " +
			"This is a fault in Caspian, not in anything you did."

	// These two were one arm, and they are opposite faults.
	//
	// EADDRINUSE, "address already in use", means somebody else holds the
	// address. EADDRNOTAVAIL, "cannot assign requested address", means the
	// address is on NO interface at all. Both were answered with the
	// EADDRINUSE sentence, because "failed to create listening socket" is the
	// prefix dnsmasq prints for BOTH and it was matched first.
	//
	// MEASURED on the box 2026-08-30. NetworkManager took the interface this
	// program had just created and flushed its address, so dnsmasq printed
	//
	//	failed to create listening socket for 10.83.51.1: Cannot assign requested address
	//
	// and the user was told another program held the address and that they
	// should restart the machine. There was no other program, and restarting
	// reproduces it, so the advice sent somebody hunting for something that
	// does not exist and could not have worked.
	case strings.Contains(text, "cannot assign requested address"):
		return "The address this machine keeps for the hotspot was not there when the " + unit +
			" tried to use it. This is a fault in Caspian, not in anything you did. " +
			"Restarting does not help. The line in the log naming the address is the one to send on."

	case strings.Contains(text, "address already in use"),
		strings.Contains(text, "failed to create listening socket"):
		return "Another program on this machine is already answering on the address the " + unit +
			" needs. Restart the machine, and if it happens again the other program has to be turned off."

	case strings.Contains(text, "permission denied"),
		strings.Contains(text, "operation not permitted"):
		return "Caspian was not allowed to start the " + unit + ". It needs to run with " +
			"administrator rights on this machine."

	case exitCode == 127, strings.Contains(text, "no such file or directory"),
		strings.Contains(text, "command not found"):
		return "The software Caspian needs for the " + unit + " is not installed on this machine."
	}

	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = strings.TrimSpace(stdout)
	}
	if detail == "" {
		return fmt.Sprintf("The %s stopped with no explanation (code %d).", unit, exitCode)
	}
	// Deliberately not dressed up as a diagnosis. Claiming a cause that was
	// not identified sends the user to fix the wrong thing.
	return fmt.Sprintf("The %s would not start and Caspian does not recognise the reason. "+
		"It reported: %s", unit, firstLine(detail))
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
