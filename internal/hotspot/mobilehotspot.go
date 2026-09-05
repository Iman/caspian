// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package hotspot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MobileHotspot is the Windows AccessPoint: Windows 11's Mobile Hotspot,
// driven through a small helper program because the API that controls it is
// WinRT (Windows.Networking.NetworkOperators.NetworkOperatorTetheringManager)
// and Go has no projection of that namespace.
//
// Why Mobile Hotspot and not a hosted network: the legacy
// "netsh wlan hostednetwork" needs a driver capability that every current
// Wi-Fi driver, including the USB dongles this product is for, reports as
// "Hosted network supported: No". Mobile Hotspot runs on Wi-Fi Direct, which
// those drivers do support. The decision and its evidence are in the port
// research (2026-09-03).
//
// The helper is the C# program under tools/caspian-tethering. It is spawned
// once per action with one JSON request on standard input and answers with
// one JSON line on standard output, then exits. That shape is chosen so the
// helper holds no state, so it is testable here through the System double
// exactly like hostapd is, and so a helper that crashes takes nothing down
// with it. The privileged service supervises; the helper only calls the API.
//
// What the API can and cannot set, VERIFIED against Microsoft Learn: SSID,
// passphrase, band (2.4, 5, auto) and the authentication kind. Not the
// channel, not a hidden SSID. Windows turns the hotspot off five minutes after
// the last client leaves unless told not to; the helper tells it not to on
// every start. Clients are reported by MAC address and host name, never by
// IP, so the device list here carries no addresses.
type MobileHotspot struct {
	sys   System
	paths MobileHotspotPaths
	now   func() time.Time

	// StartSettle is how long to wait between status polls after a start,
	// and StartTries how many polls to make before giving up.
	StartSettle time.Duration
	StartTries  int

	mu   sync.Mutex
	plan Plan
	have bool
}

// MobileHotspotPaths is where the helper is.
type MobileHotspotPaths struct {
	// Helper is the absolute path of caspian-tethering.exe. It sits beside
	// caspian.exe in the program directory, which is admin-only writable.
	Helper string
}

// NewMobileHotspot returns the Windows access point over sys.
func NewMobileHotspot(sys System, paths MobileHotspotPaths) *MobileHotspot {
	return &MobileHotspot{
		sys:         sys,
		paths:       paths,
		now:         time.Now,
		StartSettle: 500 * time.Millisecond,
		StartTries:  40,
	}
}

var _ AccessPoint = (*MobileHotspot)(nil)

// SetClock replaces the clock.
func (m *MobileHotspot) SetClock(now func() time.Time) { m.now = now }

// TetheringRequest is what the helper reads on standard input. The field
// names are the helper's contract; tools/caspian-tethering/Program.cs reads
// exactly these.
type TetheringRequest struct {
	// Op is "start", "stop" or "status".
	Op string `json:"op"`
	// Uplink is the alias of the adapter whose connection is shared.
	Uplink string `json:"uplink,omitempty"`
	// Adapter is the alias of the Wi-Fi adapter that hosts the network; empty
	// lets Windows pick.
	Adapter    string `json:"adapter,omitempty"`
	SSID       string `json:"ssid,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	// Band is "2.4", "5" or "auto".
	Band string `json:"band,omitempty"`
}

// TetheringReply is the helper's one line of standard output.
type TetheringReply struct {
	OK bool `json:"ok"`
	// State is "on", "off", "transition" or "unknown".
	State string `json:"state"`
	SSID  string `json:"ssid,omitempty"`
	Band  string `json:"band,omitempty"`
	// Clients are the joined devices as Windows reports them.
	Clients []TetheringClient `json:"clients,omitempty"`
	// Error is the helper's sentence when OK is false. It names a
	// TetheringOperationStatus or a TetheringCapability value where it can,
	// so the fault mapping in internal/privsvc has something to key on.
	Error string `json:"error,omitempty"`
	// Code is that enum's name, for example "NetworkLimitedConnectivity" or
	// "DisabledByGroupPolicy"; empty when there is none.
	Code string `json:"code,omitempty"`
}

// TetheringClient is one joined device.
type TetheringClient struct {
	MAC       string   `json:"mac"`
	Hostnames []string `json:"hostnames,omitempty"`
}

// Start brings Mobile Hotspot up for the plan.
func (m *MobileHotspot) Start(ctx context.Context, plan Plan) (Status, error) {
	if plan.AP.Uplink == "" {
		return Status{Reason: "The plan names no internet connection to share."},
			errors.New("hotspot: mobile hotspot: the plan names no uplink interface")
	}
	m.mu.Lock()
	m.plan, m.have = plan, true
	m.mu.Unlock()

	// Idempotent: on the air with this name already means nothing to do.
	if st, err := m.Status(ctx, plan.AP.Interface); err == nil && st.Running && st.reasonSSID == plan.AP.SSID {
		return st, nil
	}

	req := TetheringRequest{
		Op:         "start",
		Uplink:     plan.AP.Uplink,
		Adapter:    plan.AP.Interface,
		SSID:       plan.AP.SSID,
		Passphrase: plan.AP.Passphrase,
		Band:       tetheringBand(plan.AP.Band),
	}
	rep, err := m.call(ctx, req)
	if err != nil {
		return Status{Reason: "Windows could not be asked to start the hotspot."}, err
	}
	if !rep.OK {
		return Status{Reason: mobileHotspotReason(rep)}, fmt.Errorf("hotspot: mobile hotspot: %s", rep.Error)
	}

	// Read it back until Windows says on, or stop waiting.
	tries := m.StartTries
	if tries <= 0 {
		tries = 1
	}
	for i := 0; i < tries; i++ {
		st, err := m.Status(ctx, plan.AP.Interface)
		if err == nil && st.Running {
			return st, nil
		}
		if err := m.sys.Sleep(ctx, m.StartSettle); err != nil {
			return Status{}, err
		}
	}
	// Status always carries a reason when the hotspot is not on, including
	// when the helper itself could not be reached, so the caller sees why.
	st, _ := m.Status(ctx, plan.AP.Interface)
	return st, errors.New("hotspot: mobile hotspot: the hotspot did not come on")
}

// Stop switches Mobile Hotspot off.
func (m *MobileHotspot) Stop(ctx context.Context) error {
	m.mu.Lock()
	plan, have := m.plan, m.have
	m.mu.Unlock()
	req := TetheringRequest{Op: "stop"}
	if have {
		req.Uplink = plan.AP.Uplink
		req.Adapter = plan.AP.Interface
	}
	rep, err := m.call(ctx, req)
	if err != nil {
		return err
	}
	if !rep.OK && rep.State != "off" {
		return fmt.Errorf("hotspot: mobile hotspot: stopping: %s", rep.Error)
	}
	return nil
}

// Status reports without changing anything.
func (m *MobileHotspot) Status(ctx context.Context, _ string) (Status, error) {
	m.mu.Lock()
	plan, have := m.plan, m.have
	m.mu.Unlock()
	req := TetheringRequest{Op: "status"}
	if have {
		req.Uplink = plan.AP.Uplink
		req.Adapter = plan.AP.Interface
	}
	rep, err := m.call(ctx, req)
	if err != nil {
		return Status{Reason: "The hotspot helper did not answer."}, err
	}
	var st Status
	st.reasonSSID = rep.SSID
	on := rep.OK && rep.State == "on"
	st.AccessPoint = ProcState{Running: on, Beaconing: on}
	// Internet Connection Sharing runs DHCP for the hotspot the moment it is
	// on; there is no separate process to observe.
	st.DHCP = ProcState{Running: on}
	st.Running = on
	for _, c := range rep.Clients {
		l := Lease{MAC: strings.ToLower(c.MAC)}
		if len(c.Hostnames) > 0 {
			l.Hostname = c.Hostnames[0]
		}
		st.Devices = append(st.Devices, l)
	}
	if !on {
		st.Reason = mobileHotspotReason(rep)
	}
	return st, nil
}

func (m *MobileHotspot) call(ctx context.Context, req TetheringRequest) (TetheringReply, error) {
	if m.paths.Helper == "" {
		return TetheringReply{}, errors.New("hotspot: mobile hotspot: no helper path was configured")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return TetheringReply{}, err
	}
	res, err := m.sys.RunInput(ctx, string(body)+"\n", m.paths.Helper, req.Op)
	if err != nil {
		return TetheringReply{}, fmt.Errorf("hotspot: mobile hotspot: running the helper: %w", err)
	}
	var rep TetheringReply
	line := strings.TrimSpace(res.Stdout)
	if line == "" {
		return TetheringReply{}, fmt.Errorf("hotspot: mobile hotspot: the helper exited %d and printed nothing: %s",
			res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if err := json.Unmarshal([]byte(line), &rep); err != nil {
		return TetheringReply{}, fmt.Errorf("hotspot: mobile hotspot: the helper's answer could not be read: %w", err)
	}
	return rep, nil
}

// tetheringBand maps this package's band words onto the helper's.
func tetheringBand(b Band) string {
	switch b {
	case Band5GHz:
		return "5"
	case Band2GHz:
		return "2.4"
	default:
		return "auto"
	}
}

// mobileHotspotReason turns the helper's answer into the sentence the panel
// shows. The codes are TetheringOperationStatus and TetheringCapability
// values from Windows, named so that a person reading the advanced view can
// look them up.
func mobileHotspotReason(rep TetheringReply) string {
	switch rep.Code {
	case "WiFiDeviceOff":
		return "The Wi-Fi radio is switched off."
	case "NetworkLimitedConnectivity":
		return "Windows refused to share a connection it does not consider connected to the internet."
	case "RadioRestriction", "BandInterference":
		return "The radio cannot host the network on the band that was asked for while joined to its current network."
	case "DisabledByGroupPolicy", "DisabledByOperator", "DisabledBySku", "DisabledBySystemCapability":
		return "Mobile Hotspot is disabled on this computer by policy."
	case "DisabledByHardwareLimitation":
		return "No Wi-Fi adapter on this computer can host a hotspot: the driver does not support Wi-Fi Direct."
	}
	if rep.Error != "" {
		return "Windows reported: " + rep.Error
	}
	switch rep.State {
	case "transition":
		return "Windows is still switching the hotspot."
	case "off":
		return "Mobile Hotspot is off."
	}
	return "Mobile Hotspot is not running."
}
