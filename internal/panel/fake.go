// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"context"
	"sync"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/hotspot"
)

// FakePrivileged is an in-memory Privileged for tests.
//
// It ships in the package rather than in a _test.go file on purpose. The panel
// is not the only thing that will want to exercise this boundary: whoever
// writes the real privileged service needs something to check their client
// against, and a command that wants to run the panel on a developer machine
// with no root and no radio needs something to talk to. Keeping one fake means
// there is one definition of how the boundary behaves rather than three.
//
// It is safe for concurrent use, because the panel polls Status from one
// request while another is running Start.
//
// It does not simulate a network, a radio or an engine. Start records what it
// was asked for and flips a flag. Anything a test wants to be true, it sets.
type FakePrivileged struct {
	mu sync.Mutex

	detection Detection
	hotspot   HotspotStatus
	engine    engine.State
	log       EngineLog

	// Faults to return from the corresponding action, if any.
	detectErr, statusErr, startErr, stopErr, logErr error

	// starts records every StartRequest received, in order, so a test can
	// assert what actually crossed the boundary. It holds credentials, which
	// is exactly why it is here: the test that matters most asserts that the
	// config reached this slice and reached no response body and no log line.
	starts     []StartRequest
	stops      int
	recovers   int
	recoverErr error

	cut    bool
	cuts   int
	cutErr error

	now func() time.Time
}

var _ Privileged = (*FakePrivileged)(nil)

// NewFakePrivileged returns a fake describing a plausible healthy machine: an
// Ethernet uplink, a built-in radio that can host an access point, and nothing
// running yet.
//
// The shape of the machine is the one measured in design section 4.6, including
// the detail that matters most for the panel: the built-in radio's access point
// is pinned to the channel of the WiFi client link, so ChannelPinned is true.
func NewFakePrivileged() *FakePrivileged {
	return &FakePrivileged{
		now: time.Now,
		detection: Detection{
			InternetInterface:   "eth0",
			HotspotInterface:    "wlan0",
			HotspotAddress:      "10.62.0.1",
			LocalNetworkAddress: "192.168.1.42",
			Interfaces: []InterfaceInfo{
				{Name: "eth0", Kind: KindEthernet, HasDefaultRoute: true},
				{Name: "wlan0", Kind: KindBuiltinWiFi, CanHostAP: true},
			},
			Channel: 10,
			// The band string is internal/hotspot's constant rather than a
			// literal, because that is the value the real privileged side will
			// report and the value hostapd is configured from. Writing "2.4"
			// here would have made the fake agree with an earlier draft of the
			// panel and with nothing else.
			Band:           string(hotspot.Band2GHz),
			Country:        "GB",
			UsableChannels: []int{1, 6, 10, 11},
			ChannelPinned:  true,
			Subnet:         "10.62.0.0/24",
			At:             time.Now(),
		},
		engine: engine.State{Phase: engine.PhaseStopped},
	}
}

func (f *FakePrivileged) Detect(ctx context.Context) (Detection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Detection{}, err
	}
	if f.detectErr != nil {
		return Detection{}, f.detectErr
	}
	return f.detection, nil
}

func (f *FakePrivileged) Status(ctx context.Context) (SystemStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return SystemStatus{}, err
	}
	if f.statusErr != nil {
		return SystemStatus{}, f.statusErr
	}
	return SystemStatus{
		Engine:           f.engine,
		Hotspot:          f.hotspot,
		Detection:        f.detection,
		ClientTrafficCut: f.cut,
		At:               f.now(),
	}, nil
}

func (f *FakePrivileged) Start(ctx context.Context, req StartRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	f.starts = append(f.starts, req)
	if f.startErr != nil {
		// A failed start leaves nothing running, which is the fail-closed
		// position design section 7 requires.
		f.engine = engine.State{Phase: engine.PhaseFailed, Since: f.now()}
		f.hotspot.Running = false
		return f.startErr
	}
	f.engine = engine.State{Phase: engine.PhaseRunning, Since: f.now()}
	f.hotspot.Running = true
	f.hotspot.SSID = req.Hotspot.SSID
	return nil
}

func (f *FakePrivileged) Stop(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	f.stops++
	if f.stopErr != nil {
		return f.stopErr
	}
	f.engine = engine.State{Phase: engine.PhaseStopped, Since: f.now()}
	f.hotspot.Running = false
	return nil
}

// Recover models the real one: everything down, the machine put back, then a
// start. It records the recover so a test can tell it apart from a plain
// stop-then-start, and it clears the cut, because the real one does too by
// going through a stop.
func (f *FakePrivileged) Recover(ctx context.Context, req StartRequest) error {
	f.mu.Lock()
	if err := ctx.Err(); err != nil {
		f.mu.Unlock()
		return err
	}
	f.recovers++
	f.cut = false
	f.engine = engine.State{Phase: engine.PhaseStopped, Since: f.now()}
	f.hotspot.Running = false
	recErr := f.recoverErr
	f.mu.Unlock()

	if recErr != nil {
		return recErr
	}
	return f.Start(ctx, req)
}

// Recovers is how many times Recover was called.
func (f *FakePrivileged) Recovers() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.recovers
}

// SetRecoverError makes the next Recover fail before it starts anything, which
// is the shape of a machine whose journal could not be replayed.
func (f *FakePrivileged) SetRecoverError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recoverErr = err
}

// Cut and Restore model the privileged side's runtime flag: set while running,
// refused while not, and never written anywhere.
func (f *FakePrivileged) Cut(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.cutErr != nil {
		return f.cutErr
	}
	if f.engine.Phase != engine.PhaseRunning {
		return &FaultError{Fault: FaultNotRunning}
	}
	f.cut = true
	f.cuts++
	return nil
}

func (f *FakePrivileged) Restore(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.cutErr != nil {
		return f.cutErr
	}
	if f.engine.Phase != engine.PhaseRunning {
		return &FaultError{Fault: FaultNotRunning}
	}
	f.cut = false
	return nil
}

// FailCutWith makes the next Cut or Restore fail with a fault.
func (f *FakePrivileged) FailCutWith(fault Fault) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cutErr = &FaultError{Fault: fault}
}

// Cuts is how many times the traffic was cut, so a test can assert that a
// second press of an already cut box did not go through.
func (f *FakePrivileged) Cuts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cuts
}

func (f *FakePrivileged) EngineLog(ctx context.Context) (EngineLog, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return EngineLog{}, err
	}
	if f.logErr != nil {
		return EngineLog{}, f.logErr
	}
	return f.log, nil
}

// ---------------------------------------------------------------------------
// Setters. Each takes the lock, so a test may drive the fake from another
// goroutine while the panel is serving.
// ---------------------------------------------------------------------------

// SetDetection replaces what Detect reports.
func (f *FakePrivileged) SetDetection(d Detection) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.detection = d
}

// SetHotspot replaces the access point half of Status.
func (f *FakePrivileged) SetHotspot(h HotspotStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hotspot = h
}

// SetEngineState replaces the tunnel half of Status.
func (f *FakePrivileged) SetEngineState(s engine.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.engine = s
}

// SetEngineLog replaces what EngineLog reports.
func (f *FakePrivileged) SetEngineLog(l EngineLog) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = l
}

// FailStartWith makes the next and every following Start return this fault.
// Pass FaultNone to stop failing.
func (f *FakePrivileged) FailStartWith(fault Fault) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fault == FaultNone {
		f.startErr = nil
		return
	}
	f.startErr = faultErr(fault)
}

// FailStopWith makes Stop return this fault.
func (f *FakePrivileged) FailStopWith(fault Fault) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fault == FaultNone {
		f.stopErr = nil
		return
	}
	f.stopErr = faultErr(fault)
}

// FailStatusWith makes Detect and Status return this fault, which is how a test
// reaches the "the privileged service is not answering" path.
func (f *FakePrivileged) FailStatusWith(fault Fault) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fault == FaultNone {
		f.statusErr, f.detectErr = nil, nil
		return
	}
	f.statusErr, f.detectErr = faultErr(fault), faultErr(fault)
}

// Starts returns a copy of every StartRequest received.
func (f *FakePrivileged) Starts() []StartRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]StartRequest(nil), f.starts...)
}

// Stops returns how many times Stop was called.
func (f *FakePrivileged) Stops() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stops
}
