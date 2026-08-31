// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"errors"
	"io/fs"
	"os/exec"
	"strings"

	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// faultError is how this package returns a refusal.
//
// It is panel.FaultError, which carries a Fault and deliberately nothing else,
// wrapped so that the cause stays available INSIDE this process for a log line
// and can never be reached by the panel: the wire carries the Fault word only
// (see wire.go), so nothing the caller sent and nothing the engine said can
// travel back across the socket in an error.
type faultError struct {
	fault panel.Fault
	cause error
	// where names the step, from a fixed vocabulary in this package. It is
	// never a value from the request.
	where string
}

func (e *faultError) Error() string {
	msg := "privsvc: " + e.where + ": " + string(e.fault)
	if e.cause != nil {
		msg += ": " + e.cause.Error()
	}
	return msg
}

// Unwrap returns a tree with the panel's own error type in it as well as the
// cause.
//
// The panel.FaultError branch is what makes panel.FaultOf work on an error this
// service returned. That matters even though the socket carries only the Fault
// word: a Service is a panel.Privileged, and internal/panel is entitled to call
// FaultOf on anything a panel.Privileged returns. Without this branch an
// in-process Service would report every refusal as unclassified while the same
// refusal over the socket reported the right word, which is two behaviours from
// one implementation.
func (e *faultError) Unwrap() []error {
	errs := []error{&panel.FaultError{Fault: e.fault}}
	if e.cause != nil {
		errs = append(errs, e.cause)
	}
	return errs
}

// Fault returns the machine-readable reason. It is what crosses the socket.
func (e *faultError) Fault() panel.Fault { return e.fault }

func fail(where string, f panel.Fault, cause error) error {
	return &faultError{fault: f, cause: cause, where: where}
}

// faultOf reduces any error this package can produce to the one word that
// crosses the socket.
//
// The default is panel.FaultUnknown and that is deliberate. priv.go: "It exists
// so that an unclassified failure is reported as unclassified rather than being
// forced into the nearest category, which would send the user to fix the wrong
// thing."
func faultOf(err error) panel.Fault {
	if err == nil {
		return panel.FaultNone
	}
	var fe *faultError
	if errors.As(err, &fe) {
		return fe.fault
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return panel.FaultUnavailable

	// internal/netcfg's own refusals. Each is a distinct situation with a
	// distinct remedy, which is why that package made them distinct errors.
	case errors.Is(err, netcfg.ErrNoUplink):
		return panel.FaultNoInternetInterface
	case errors.Is(err, netcfg.ErrNoAPCapableInterface), errors.Is(err, netcfg.ErrAPConflictsWithUplink):
		return panel.FaultNoAPAdapter
	case errors.Is(err, netcfg.ErrNoTakeoverCandidate):
		// The radio would not create a second interface and the only one that
		// could host the access point is carrying the internet. Taking it
		// would cut off the connection the box exists to share, so netcfg
		// refuses. The remedy it gives the user is the remedy this fault's own
		// sentence gives, "plug in a USB WiFi adapter", so it is reported as
		// that rather than as an unclassified failure.
		return panel.FaultNoAPAdapter
	case errors.Is(err, netcfg.ErrNotAccessPoint):
		// The interface was read back and is not an access point, or is
		// broadcasting a name that is not the one it was given. The hotspot
		// did not start, which is exactly what this fault says, and restarting
		// the machine is a reasonable first move for a driver or a daemon that
		// would not come up.
		return panel.FaultHotspotFailed
	case errors.Is(err, netcfg.ErrHotspotNotReleased):
		// The adapter can host a hotspot and is busy holding somebody else's
		// network. internal/panel carries a word for exactly that, and the
		// reason it is its own word rather than one of the neighbours is
		// written where the constant is declared: FaultNoAPAdapter would say
		// no adapter can create a hotspot, which is false here, and
		// FaultHotspotFailed would say to restart, which is the one thing that
		// cannot help, because the box boots, the network manager rejoins the
		// same network, and the refusal is identical.
		return panel.FaultHotspotInterfaceBusy
	case errors.Is(err, netcfg.ErrUnsupportedPlatform):
		return panel.FaultSoftwareMissing
	case errors.Is(err, netcfg.ErrDisallowedBinary):
		// A Command naming a binary off the allowlist can only come from a
		// bug in this tree, never from the caller: every Command originates
		// inside internal/netcfg. Reporting it as unclassified is right,
		// because there is nothing the user can do about it.
		return panel.FaultUnknown

	case errors.Is(err, exec.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return panel.FaultSoftwareMissing
	case errors.Is(err, fs.ErrPermission):
		return panel.FaultPermissionDenied
	}
	return panel.FaultUnknown
}

// hotspotFault classifies a failure from internal/hotspot.
//
// internal/hotspot has already turned the daemon's own output into a sentence
// for the user (status.go, explainFailure) and that sentence stays on this side
// of the socket: it can quote a daemon's stderr, and the rule for this boundary
// is that only the closed Fault vocabulary crosses it. So the sentence is
// matched here to pick a Fault, and then it is logged locally and dropped.
//
// The matching is on internal/hotspot's OWN wording, which this package can
// read from that package's source, and not on hostapd's or dnsmasq's. That is
// the difference between one lookup table and two.
func hotspotFault(unit string, reason string, err error) panel.Fault {
	text := strings.ToLower(reason)
	switch {
	case strings.Contains(text, "switched off by a switch on the machine itself"),
		strings.Contains(text, "switched off in software"),
		strings.Contains(text, "wireless adapter is switched off"):
		return panel.FaultRadioBlocked
	case strings.Contains(text, "would not accept the channel"):
		return panel.FaultChannelRefused
	case strings.Contains(text, "cannot create a hotspot"),
		strings.Contains(text, "reports no wireless adapter"):
		return panel.FaultNoAPAdapter
	case strings.Contains(text, "is not installed on this machine"):
		return panel.FaultSoftwareMissing
	case strings.Contains(text, "needs to run with"),
		strings.Contains(text, "not allowed to start"):
		return panel.FaultPermissionDenied
	}
	// Guarded on err being non-nil, and that guard is the whole of a bug this
	// function had: faultOf(nil) is panel.FaultNone, which is not
	// panel.FaultUnknown, so an unguarded call returned "nothing is wrong" for
	// every failure that arrived as a Status with a Reason and no error. The
	// visible symptom was a panel showing a hotspot that is down and no reason
	// for it. TestABoxThatWasSwitchedOnAndLostItsHotspotDoesReportAFault is
	// what caught it and is what keeps it caught.
	if err != nil {
		if f := faultOf(err); f != panel.FaultUnknown {
			return f
		}
	}
	if unit == unitDHCP {
		return panel.FaultDHCPFailed
	}
	return panel.FaultHotspotFailed
}

// The two units, named the way internal/hotspot names them in the sentences it
// writes. They are repeated here rather than imported because that package
// keeps them unexported.
const (
	unitAP   = "hotspot"
	unitDHCP = "DHCP and DNS server"
)
