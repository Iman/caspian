// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"fmt"
	"net"
	"testing"

	applog "github.com/xtls/xray-core/app/log"
	"github.com/xtls/xray-core/core"
)

// The tests in this file cover the branches the coverage profile showed
// unreached on 2026-08-30.
//
// All but one are CHARACTERISATION tests: the code already did the right thing
// and nothing would have noticed if it stopped. The exception is
// TestCancelBetweenConstructAndStartIsNoticed, which could not be written at
// all until buildAndStart grew the hookAfterConstruct seam, because the window
// it covers is otherwise a race: a timing-based test would sometimes hit it
// and sometimes not, and an intermittent test is worse than none.

// --- the three ways buildAndStart can fail after the config loads ------------

// TestStartFailsWhenCoreCannotBeConstructed covers the core.New failure path.
//
// The three configurations below were MEASURED on 2026-08-30 against
// xray-core v1.260327.0: each one loads cleanly, so engine.Validate accepts
// it, and then fails inside core.New. That gap is the point. The panel calls
// Validate to tell the user their config is good, and these are configurations
// where that answer is true and the box still cannot start, so the failure has
// to arrive as a redacted *Error with the phase left at failed.
func TestStartFailsWhenCoreCannotBeConstructed(t *testing.T) {
	configs := map[string]string{
		"two inbounds with one tag": `{"log":{"loglevel":"warning"},
			"inbounds":[
			  {"tag":"a","listen":"127.0.0.1","port":18901,"protocol":"socks","settings":{"auth":"noauth"}},
			  {"tag":"a","listen":"127.0.0.1","port":18902,"protocol":"socks","settings":{"auth":"noauth"}}],
			"outbounds":[{"protocol":"freedom","settings":{}}]}`,
		"two outbounds with one tag": `{"log":{"loglevel":"warning"},
			"inbounds":[{"listen":"127.0.0.1","port":18903,"protocol":"socks","settings":{"auth":"noauth"}}],
			"outbounds":[{"tag":"z","protocol":"freedom","settings":{}},{"tag":"z","protocol":"freedom","settings":{}}]}`,
		"a rule naming a balancer that is not there": `{"log":{"loglevel":"warning"},
			"inbounds":[{"listen":"127.0.0.1","port":18904,"protocol":"socks","settings":{"auth":"noauth"}}],
			"outbounds":[{"protocol":"freedom","settings":{}}],
			"routing":{"balancers":[{"tag":"b","selector":["nothing"]}],
			           "rules":[{"balancerTag":"missing","network":"tcp"}]}}`,
	}

	for name, cfg := range configs {
		t.Run(name, func(t *testing.T) {
			// The premise: the loader accepts it. If this stops being true the
			// test is no longer exercising core.New and says so.
			if err := Validate([]byte(cfg)); err != nil {
				t.Fatalf("this configuration no longer loads, so it cannot reach core.New: %v", err)
			}

			e := New()
			err := e.Start(context.Background(), []byte(cfg))
			if err == nil {
				_ = e.Stop()
				t.Fatal("Start accepted a configuration that core.New cannot construct")
			}
			if _, ok := err.(*Error); !ok {
				t.Errorf("Start returned %T, want *Error so the message is already redacted", err)
			}
			st := e.State()
			if st.Phase != PhaseFailed {
				t.Errorf("phase after a failed Start = %v, want failed", st.Phase)
			}
			if st.Reason == "" {
				t.Error("a failed Start left no reason for the panel to show")
			}
			// Nothing was constructed, so Stop must still be safe and must not
			// claim there was something to stop.
			if err := e.Stop(); err != nil {
				t.Errorf("Stop after a failed Start: %v", err)
			}
		})
	}
}

// TestStartFailsWhenTheListenerCannotBind covers the inst.Start() failure path,
// which is a different branch from the one above and a much more likely one on
// a real box: the config is good, the instance constructs, and the socket is
// already taken.
//
// It also pins the thing that branch exists for. core/xray.go:380 warns that
// after a failed Start the instance state is unknown, so the instance is
// CLOSED rather than kept. If it were kept, the engine would hold a half
// started instance that Stop would later close a second time.
func TestStartFailsWhenTheListenerCannotBind(t *testing.T) {
	port := freeLoopbackPort(t)

	// Take the port and hold it for the whole test.
	blocker, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("could not take the port to block it: %v", err)
	}
	defer blocker.Close()

	cfg := socksOnLoopback(port)
	if err := Validate(cfg); err != nil {
		t.Fatalf("the blocking configuration does not even load: %v", err)
	}

	e := New()
	startErr := e.Start(context.Background(), cfg)
	if startErr == nil {
		_ = e.Stop()
		t.Fatal("Start succeeded on a port that was already taken")
	}
	if _, ok := startErr.(*Error); !ok {
		t.Errorf("Start returned %T, want *Error", startErr)
	}
	if got := e.State().Phase; got != PhaseFailed {
		t.Errorf("phase after a bind failure = %v, want failed", got)
	}

	// The instance was closed rather than retained: a second Start on a now
	// free port has to work, which it cannot if the failed instance is still
	// held.
	blocker.Close()
	free := freeLoopbackPort(t)
	if err := e.Start(context.Background(), socksOnLoopback(free)); err != nil {
		t.Fatalf("the engine could not start again after a bind failure: %v", err)
	}
	t.Cleanup(func() { _ = e.Stop() })
	if !waitFor(t, "the retried socks inbound to accept", func() bool { return accepting(t, free) }) {
		t.Error("the engine reports running after the retry but nothing is listening")
	}
}

// TestCancelBetweenConstructAndStartIsNoticed covers the one branch in
// this package that no black-box test can reach.
//
// The window is between core.New returning an instance and (*Instance).Start
// being called on it. Neither call is interruptible, so a context cancelled
// from another goroutine lands in that window only by luck. hookAfterConstruct
// makes it deterministic.
//
// WHAT THIS TEST DOES AND DOES NOT PROVE, stated precisely because an earlier
// draft of this comment got it wrong and the mutation run caught it.
//
// It proves: the cancellation is noticed in that window, an *Error is
// returned, the phase ends at failed, nothing is listening, and the engine is
// still usable afterwards.
//
// It does NOT prove that the half-built instance is CLOSED. Removing the
// `_ = inst.Close()` line leaves this test passing, and it was verified on
// 2026-08-30 that it does. The reason is that the instance was never started,
// so it never bound the port, so closing it changes nothing this process can
// observe from outside. That statement is covered and not guarded, and it is
// recorded as such rather than described as protected.
func TestCancelBetweenConstructAndStartIsNoticed(t *testing.T) {
	port := freeLoopbackPort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e := New()
	e.hookAfterConstruct = cancel

	err := e.Start(ctx, socksOnLoopback(port))
	if err == nil {
		_ = e.Stop()
		t.Fatal("Start succeeded although the context was cancelled before the instance started")
	}
	if _, ok := err.(*Error); !ok {
		t.Errorf("Start returned %T, want *Error", err)
	}
	if got := e.State().Phase; got != PhaseFailed {
		t.Errorf("phase after a cancelled Start = %v, want failed", got)
	}

	// The instance never started, so it never bound, and this assertion holds
	// whether or not the Close ran. It is kept because it would catch a change
	// that moved the cancellation check to AFTER inst.Start, which would bind
	// the port and then fail.
	if accepting(t, port) {
		t.Errorf("something is listening on 127.0.0.1:%d after a cancelled Start; "+
			"the cancellation is being checked after the instance is started", port)
	}

	// And the engine is reusable, which a leaked instance would also break.
	if err := e.Stop(); err != nil {
		t.Errorf("Stop after a cancelled Start: %v", err)
	}
}

// --- LogRing --------------------------------------------------------------

// TestNilLogRingIsSafeOnEveryMethod covers the four nil guards.
//
// They are not decoration. captureEngineLogs installs a process-global handler
// that writes into whichever ring is current, and the ring pointer is reachable
// from an Engine that a caller may have discarded. Every one of these methods
// is called from that handler or from the panel.
func TestNilLogRingIsSafeOnEveryMethod(t *testing.T) {
	var r *LogRing

	r.Add("this must not panic")
	r.Reset()

	if got := r.Entries(); got != nil {
		t.Errorf("(*LogRing)(nil).Entries() = %v, want nil", got)
	}
	if got := r.Dropped(); got != 0 {
		t.Errorf("(*LogRing)(nil).Dropped() = %d, want 0", got)
	}
}

// TestNewLogRingReplacesAnUnusableCapacity covers the capacity guard. A ring of
// zero length would divide by zero in Add, so this is a panic guard rather than
// a tidiness one.
func TestNewLogRingReplacesAnUnusableCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1, -1000} {
		r := NewLogRing(capacity)
		if got := len(r.buf); got != DefaultLogCapacity {
			t.Errorf("NewLogRing(%d) made a ring of %d, want %d", capacity, got, DefaultLogCapacity)
		}
		// It has to actually work, not merely be the right size.
		r.Add("one")
		if got := len(r.Entries()); got != 1 {
			t.Errorf("NewLogRing(%d) produced a ring that holds %d entries after one Add", capacity, got)
		}
	}
}

// TestLogsDroppedIsReportedThroughTheEngine covers Engine.LogsDropped.
//
// The panel needs this to say "you are looking at the last 2 of 5" rather than
// implying the view is complete, so the number has to be the ring's real
// eviction count and not a constant.
func TestLogsDroppedIsReportedThroughTheEngine(t *testing.T) {
	e := NewWithLogCapacity(2)

	if got := e.LogsDropped(); got != 0 {
		t.Errorf("a fresh engine reports %d dropped log lines, want 0", got)
	}

	for _, line := range []string{"one", "two", "three", "four", "five"} {
		e.logs.Add(line)
	}

	if got := e.LogsDropped(); got != 3 {
		t.Errorf("LogsDropped() = %d after 5 lines through a ring of 2, want 3", got)
	}
	if got := len(e.Logs()); got != 2 {
		t.Errorf("Logs() returned %d entries, want 2", got)
	}
	// Oldest first, and the oldest surviving line is the fourth.
	if got := e.Logs()[0].Text; got != "four" {
		t.Errorf("the oldest retained line is %q, want \"four\"", got)
	}
}

// TestRingSinkIgnoresANilMessage covers the guard in ringSink.Handle. A handler
// registered process-globally is called by code this package does not own, so
// a nil message must be dropped rather than dereferenced.
func TestRingSinkIgnoresANilMessage(t *testing.T) {
	// No panic and no entry.
	e := NewWithLogCapacity(4)
	before := len(e.Logs())
	ringSink{}.Handle(nil)
	if got := len(e.Logs()); got != before {
		t.Errorf("a nil log message added %d entries to the ring", got-before)
	}
}

// --- forceLogToRing ---------------------------------------------------------

// TestForceLogToRingSkipsAppsThatAreNotTheLogger covers the type-mismatch
// continue. A config carries several apps and the log app is not first, so
// walking past the others is the normal path, not an edge case.
func TestForceLogToRingSkipsAppsThatAreNotTheLogger(t *testing.T) {
	logApp, err := toTypedMessage(&applog.Config{ErrorLogType: applog.LogType_File, ErrorLogPath: "/tmp/should-be-replaced"})
	if err != nil {
		t.Fatalf("building the log app: %v", err)
	}
	// A second app of a different type, placed FIRST so the loop has to skip
	// it before it finds the logger.
	other, err := toTypedMessage(&applog.Config{})
	if err != nil {
		t.Fatalf("building a second app: %v", err)
	}
	other.Type = "caspian.test.NotALogger"

	cfg := &coreConfig{App: []*typedMessage{other, logApp}}
	if err := forceLogToRing(cfg); err != nil {
		t.Fatalf("forceLogToRing: %v", err)
	}

	if len(cfg.App) != 2 {
		t.Fatalf("forceLogToRing changed the number of apps to %d, want 2", len(cfg.App))
	}
	if cfg.App[0] != other {
		t.Error("forceLogToRing rewrote the app that is not the logger")
	}

	inst, err := cfg.App[1].GetInstance()
	if err != nil {
		t.Fatalf("the rewritten log app does not decode: %v", err)
	}
	lc, ok := inst.(*applog.Config)
	if !ok {
		t.Fatalf("the rewritten log app is %T, want *applog.Config", inst)
	}
	if lc.ErrorLogType != applog.LogType_Console {
		t.Errorf("error log type = %v, want Console so it reaches the ring", lc.ErrorLogType)
	}
	if lc.ErrorLogPath != "" {
		t.Errorf("error log path = %q, want empty; the engine would still write to a file", lc.ErrorLogPath)
	}
	// The privacy decision, restated here because it is the reason this
	// function exists: access records are the destination host and resolved
	// name of every connection a hotspot client makes.
	if lc.AccessLogType != applog.LogType_None {
		t.Errorf("access log type = %v, want None", lc.AccessLogType)
	}
	if lc.EnableDnsLog {
		t.Error("DNS logging was left on")
	}
}

// TestForceLogToRingReportsAnUndecodableLogApp covers the error return.
//
// It matters because the alternative is worse than a failed start: an app entry
// that claims to be the logger and cannot be decoded would, if the error were
// swallowed, leave the engine's default console logger in place and send engine
// text carrying private keys to stdout.
func TestForceLogToRingReportsAnUndecodableLogApp(t *testing.T) {
	broken := &typedMessage{
		Type:  typedMessageName(&applog.Config{}),
		Value: []byte("this is not a protobuf message at all"),
	}
	cfg := &coreConfig{App: []*typedMessage{broken}}

	if err := forceLogToRing(cfg); err == nil {
		t.Error("forceLogToRing accepted an app entry that claims to be the logger and does not decode")
	}
}

// TestValidateAcceptsWhatCoreNewLaterRefuses records a MEASURED property of the
// engine that the panel's wording depends on, and that nothing else asserts.
//
// engine.Validate is a schema check. It is not a promise that the box will
// start. Measured 2026-08-30 against xray-core v1.260327.0: a config with two
// inbounds sharing a tag passes Validate and fails core.New. The panel must
// therefore not say "your config is valid, the box will connect" on the
// strength of Validate alone.
func TestValidateAcceptsWhatCoreNewLaterRefuses(t *testing.T) {
	cfg := []byte(`{"log":{"loglevel":"warning"},
		"inbounds":[
		  {"tag":"same","listen":"127.0.0.1","port":18911,"protocol":"socks","settings":{"auth":"noauth"}},
		  {"tag":"same","listen":"127.0.0.1","port":18912,"protocol":"socks","settings":{"auth":"noauth"}}],
		"outbounds":[{"protocol":"freedom","settings":{}}]}`)

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate rejected the config, so this test no longer shows the gap: %v", err)
	}

	loaded, err := loadConfig(cfg)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	inst, err := core.New(loaded)
	if err == nil {
		_ = inst.Close()
		t.Fatal("core.New now accepts two inbounds sharing a tag; " +
			"the gap between Validate and Start has closed and this test should be revisited")
	}
}
