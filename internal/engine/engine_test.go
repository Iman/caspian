// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	applog "github.com/xtls/xray-core/app/log"
)

// freeLoopbackPort asks the kernel for an unused port and gives it straight
// back. There is a window between the close and the engine's bind in which
// something else could take it, which is why the port is never a constant: a
// hardcoded one collides with a parallel test every time, this collides
// essentially never.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("releasing the reserved port: %v", err)
	}
	return port
}

// socksOnLoopback is the minimal working config for these tests: one socks
// inbound bound to loopback and one freedom outbound. Nothing in it resolves a
// name, dials out or needs a network, so the tests prove lifecycle and nothing
// else. It also names no DNS server, which is deliberate; see
// TestNoGoogleAnywhere.
func socksOnLoopback(port int) []byte {
	return []byte(fmt.Sprintf(`{
  "log": {"loglevel": "warning"},
  "inbounds": [{
    "listen": "127.0.0.1",
    "port": %d,
    "protocol": "socks",
    "settings": {"auth": "noauth", "udp": false}
  }],
  "outbounds": [{"protocol": "freedom", "settings": {}}]
}`, port))
}

// accepting reports whether something is listening on the loopback port.
func accepting(t *testing.T, port int) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// waitFor polls until cond holds or the deadline passes. Instance.Close
// returns before every listener has finished unwinding, so a single immediate
// check after Stop is racy in the direction that would make a broken Stop look
// fine.
func waitFor(t *testing.T, what string, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Logf("timed out waiting for %s", what)
	return false
}

func TestStartAndStop(t *testing.T) {
	port := freeLoopbackPort(t)
	e := New()

	if got := e.State().Phase; got != PhaseStopped {
		t.Fatalf("fresh engine phase = %v, want stopped", got)
	}
	if accepting(t, port) {
		t.Fatalf("port %d was already in use before the test started", port)
	}

	if err := e.Start(context.Background(), socksOnLoopback(port)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = e.Stop() })

	st := e.State()
	if st.Phase != PhaseRunning {
		t.Fatalf("phase after Start = %v (%q), want running", st.Phase, st.Reason)
	}
	if st.Reason != "" {
		t.Errorf("running engine carries a reason: %q", st.Reason)
	}

	// A phase of "running" is not evidence that anything is listening. The
	// listener is.
	if !waitFor(t, "the socks inbound to accept", func() bool { return accepting(t, port) }) {
		t.Fatalf("engine reports running but nothing is listening on 127.0.0.1:%d", port)
	}

	if err := e.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := e.State().Phase; got != PhaseStopped {
		t.Fatalf("phase after Stop = %v, want stopped", got)
	}
	if !waitFor(t, "the socks inbound to stop accepting", func() bool { return !accepting(t, port) }) {
		t.Fatalf("engine reports stopped but 127.0.0.1:%d is still accepting", port)
	}
}

// TestStartTwiceDoesNotDoubleStart proves the idempotence claim behaviourally
// rather than by inspecting a flag.
//
// If the second Start built a second instance, one of two things would show:
// either the second bind fails with address-in-use and Start returns an error,
// or two instances end up sharing the port and a single Stop leaves one of
// them still accepting. Both are checked.
func TestStartTwiceDoesNotDoubleStart(t *testing.T) {
	port := freeLoopbackPort(t)
	cfg := socksOnLoopback(port)
	e := New()
	t.Cleanup(func() { _ = e.Stop() })

	if err := e.Start(context.Background(), cfg); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	since := e.State().Since

	if err := e.Start(context.Background(), cfg); err != nil {
		t.Fatalf("second Start returned an error, so it did not no-op: %v", err)
	}
	if e.State().Phase != PhaseRunning {
		t.Fatalf("phase after second Start = %v, want running", e.State().Phase)
	}
	if !e.State().Since.Equal(since) {
		t.Errorf("second Start changed Since, so it restarted rather than no-opping")
	}

	if !waitFor(t, "the inbound to accept", func() bool { return accepting(t, port) }) {
		t.Fatal("nothing listening after two Starts")
	}

	// One Stop must be enough. If a second instance existed, this would leave
	// it holding the port.
	if err := e.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !waitFor(t, "the port to close after one Stop", func() bool { return !accepting(t, port) }) {
		t.Fatal("port still accepting after one Stop, so a second instance was started")
	}
}

func TestStartIsSafeUnderConcurrency(t *testing.T) {
	port := freeLoopbackPort(t)
	cfg := socksOnLoopback(port)
	e := New()
	t.Cleanup(func() { _ = e.Stop() })

	const goroutines = 8
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	start := make(chan struct{})
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = e.Start(context.Background(), cfg)
		}()
	}
	// A ninth goroutine hammers State throughout, which is what the panel
	// does. If State took the operation lock this would serialise behind the
	// starts; if it took no lock at all the race detector would fire.
	var polls int
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-done:
				return
			default:
			}
			_ = e.State()
			_ = e.Logs()
			polls++
			if polls > 5000 {
				return
			}
		}
	}()

	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: Start returned %v; concurrent Starts must not fail", i, err)
		}
	}
	if e.State().Phase != PhaseRunning {
		t.Fatalf("phase = %v, want running", e.State().Phase)
	}
	if !waitFor(t, "the inbound to accept", func() bool { return accepting(t, port) }) {
		t.Fatal("nothing listening after concurrent Starts")
	}
	if err := e.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !waitFor(t, "the port to close after one Stop", func() bool { return !accepting(t, port) }) {
		t.Fatal("port still accepting after one Stop, so more than one instance was started")
	}
}

func TestStopBeforeStartIsNotAnError(t *testing.T) {
	e := New()
	for i := range 3 {
		if err := e.Stop(); err != nil {
			t.Fatalf("Stop #%d on a stopped engine: %v", i+1, err)
		}
		if got := e.State().Phase; got != PhaseStopped {
			t.Fatalf("phase after Stop #%d = %v, want stopped", i+1, got)
		}
	}
}

func TestStartFailureLeavesFailedStateAndNoInstance(t *testing.T) {
	e := New()
	t.Cleanup(func() { _ = e.Stop() })

	err := e.Start(context.Background(), []byte(`{ this is not json`))
	if err == nil {
		t.Fatal("Start accepted invalid JSON")
	}
	st := e.State()
	if st.Phase != PhaseFailed {
		t.Fatalf("phase after a failed Start = %v, want failed", st.Phase)
	}
	if st.Reason == "" {
		t.Error("failed state carries no reason")
	}
	if !strings.Contains(err.Error(), st.Reason) {
		t.Errorf("State.Reason %q is not the returned error %q", st.Reason, err.Error())
	}

	// A failed Start must not block a later good one.
	port := freeLoopbackPort(t)
	if err := e.Start(context.Background(), socksOnLoopback(port)); err != nil {
		t.Fatalf("Start after a failure: %v", err)
	}
	if e.State().Phase != PhaseRunning {
		t.Fatalf("phase = %v, want running", e.State().Phase)
	}
}

func TestStartHonoursCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	port := freeLoopbackPort(t)
	e := New()
	t.Cleanup(func() { _ = e.Stop() })

	err := e.Start(ctx, socksOnLoopback(port))
	if err == nil {
		t.Fatal("Start with a cancelled context returned nil")
	}
	if e.State().Phase == PhaseRunning {
		t.Fatal("engine is running after a cancelled Start")
	}
	if accepting(t, port) {
		t.Fatal("a cancelled Start left a listener behind")
	}
}

func TestValidate(t *testing.T) {
	port := freeLoopbackPort(t)
	cases := []struct {
		name      string
		config    string
		wantError bool
		// wantIn is a substring the panel would need in order to say
		// something useful; checked only when an error is expected.
		wantIn string
	}{
		{
			name:      "minimal valid config",
			config:    string(socksOnLoopback(port)),
			wantError: false,
		},
		{
			name:      "syntactically invalid json",
			config:    `{"inbounds": [`,
			wantError: true,
			wantIn:    "config",
		},
		{
			name:      "not json at all",
			config:    "this is a share link, not a config",
			wantError: true,
			wantIn:    "config",
		},
		{
			name:      "valid json rejected by the engine: unknown protocol",
			config:    `{"outbounds": [{"protocol": "definitely-not-a-protocol"}]}`,
			wantError: true,
			wantIn:    "definitely-not-a-protocol",
		},
		{
			name:      "valid json rejected by the engine: bad reality key",
			config:    realityClientConfig(`"password": "too-short"`),
			wantError: true,
			wantIn:    "password",
		},
		{
			name:      "valid json rejected by the engine: missing address",
			config:    `{"outbounds":[{"protocol":"vless","settings":{"vnext":[{"port":443,"users":[{"id":"` + validUUID + `","encryption":"none"}]}]}}]}`,
			wantError: true,
		},
		{
			name:      "empty document",
			config:    "",
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate([]byte(tc.config))
			if tc.wantError && err == nil {
				t.Fatal("Validate accepted a config it should reject")
			}
			if !tc.wantError && err != nil {
				t.Fatalf("Validate rejected a valid config: %v", err)
			}
			if err == nil {
				return
			}
			var ee *Error
			if e, ok := err.(*Error); ok {
				ee = e
			} else {
				t.Fatalf("Validate returned %T, want *engine.Error", err)
			}
			if ee.Op != "validate" {
				t.Errorf("Op = %q, want %q", ee.Op, "validate")
			}
			if tc.wantIn != "" && !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q, so the panel cannot explain it", err.Error(), tc.wantIn)
			}
			if ContainsSecretShape(err.Error()) {
				t.Errorf("Validate error still looks like it carries key material: %q", err.Error())
			}
		})
	}
}

// TestValidateOpensNoListener is the check behind the claim in Validate's
// doc comment. Validating a config that names a port must not bind it.
func TestValidateOpensNoListener(t *testing.T) {
	port := freeLoopbackPort(t)
	cfg := socksOnLoopback(port)

	for i := range 3 {
		if err := Validate(cfg); err != nil {
			t.Fatalf("Validate #%d: %v", i+1, err)
		}
	}
	if accepting(t, port) {
		t.Fatalf("Validate bound 127.0.0.1:%d", port)
	}

	// And the port is genuinely bindable, so the check above was not passing
	// because the port was unusable.
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("port %d should still be free after Validate: %v", port, err)
	}
	_ = l.Close()
}

// TestEngineLogGoesToTheRingNotStdout covers requirement 3 in both directions.
// Asserting only that stdout is empty would pass if the engine simply said
// nothing, so the ring is checked for content in the same run.
//
// The two halves are not equally strong, and it matters which is relied on.
// The ring assertion is deterministic: with log capture disabled it failed on
// 3 of 3 runs. The stdout assertion is not: reassigning os.Stdout races the
// engine's logger goroutine reading it (common/log/logger.go:71-96 spawns
// run(), which evaluates os.Stdout at :148), so with capture disabled it
// caught the leak on only 2 of 3 runs. It is kept because it can only miss a
// defect, never invent one, but the deterministic guard for this mechanism is
// TestForceLogToRing plus the ring half below.
func TestEngineLogGoesToTheRingNotStdout(t *testing.T) {
	port := freeLoopbackPort(t)
	e := New()
	t.Cleanup(func() { _ = e.Stop() })

	// common/log/logger.go:146-150 reads os.Stdout when it creates the writer,
	// not at package init, so swapping it here is enough to catch a
	// regression that let the stock console writer back in.
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	captured := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		captured <- b
	}()

	startErr := e.Start(context.Background(), socksOnLoopback(port))
	stopErr := e.Stop()

	// The stock console handler writes from its own goroutine
	// (common/log/logger.go:71-96), so give a regression time to show itself.
	time.Sleep(300 * time.Millisecond)
	os.Stdout = orig
	_ = w.Close()
	out := <-captured
	_ = r.Close()

	if startErr != nil {
		t.Fatalf("Start: %v", startErr)
	}
	if stopErr != nil {
		t.Fatalf("Stop: %v", stopErr)
	}
	if len(out) != 0 {
		t.Errorf("engine wrote %d bytes to stdout: %q", len(out), string(out))
	}

	entries := e.Logs()
	if len(entries) == 0 {
		t.Fatal("nothing was captured in the ring, so the stdout check proves nothing")
	}
	// core/xray.go:394 logs "Xray <version> started" at warning severity on a
	// successful Start, which the default loglevel passes.
	var joined strings.Builder
	for _, en := range entries {
		joined.WriteString(en.Text)
		joined.WriteString("\n")
		if en.At.IsZero() {
			t.Error("log entry has no timestamp")
		}
	}
	if !strings.Contains(joined.String(), "started") {
		t.Errorf("ring does not contain the engine's own start line; got:\n%s", joined.String())
	}
}

// TestLogRingRedactsOnTheWayIn checks that nothing unredacted is ever held in
// the buffer, which is the reason Add redacts rather than Entries.
func TestLogRingRedactsOnTheWayIn(t *testing.T) {
	r := NewLogRing(4)
	r.Add(`infra/conf: invalid "privateKey": ` + key43)

	entries := r.Entries()
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if strings.Contains(entries[0].Text, key43) {
		t.Errorf("secret stored in the ring: %q", entries[0].Text)
	}
	if !strings.Contains(entries[0].Text, markerPrefix) {
		t.Errorf("no redaction marker: %q", entries[0].Text)
	}
}

func TestLogRingBounds(t *testing.T) {
	r := NewLogRing(3)

	if got := r.Entries(); len(got) != 0 {
		t.Fatalf("new ring has %d entries", len(got))
	}
	if got := r.Dropped(); got != 0 {
		t.Fatalf("new ring reports %d dropped", got)
	}

	for i := range 5 {
		r.Add(fmt.Sprintf("line %d", i))
	}

	entries := r.Entries()
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	want := []string{"line 2", "line 3", "line 4"}
	for i, w := range want {
		if entries[i].Text != w {
			t.Errorf("entry %d = %q, want %q (order must be oldest first)", i, entries[i].Text, w)
		}
	}
	if got := r.Dropped(); got != 2 {
		t.Errorf("Dropped = %d, want 2", got)
	}

	r.Reset()
	if got := r.Entries(); len(got) != 0 {
		t.Errorf("Reset left %d entries", len(got))
	}
	if got := r.Dropped(); got != 0 {
		t.Errorf("Reset left Dropped = %d", got)
	}
}

func TestLogRingIsConcurrencySafe(t *testing.T) {
	r := NewLogRing(16)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 50 {
				r.Add(fmt.Sprintf("writer %d line %d", i, j))
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				_ = r.Entries()
				_ = r.Dropped()
			}
		}()
	}
	wg.Wait()
	if got := len(r.Entries()); got != 16 {
		t.Errorf("got %d entries, want 16", got)
	}
}

func TestPhaseString(t *testing.T) {
	cases := map[Phase]string{
		PhaseStopped:  "stopped",
		PhaseStarting: "starting",
		PhaseRunning:  "running",
		PhaseFailed:   "failed",
		Phase(99):     "unknown",
	}
	for p, want := range cases {
		if got := p.String(); got != want {
			t.Errorf("Phase(%d).String() = %q, want %q", int(p), got, want)
		}
	}
}

// TestNoGoogleAnywhere enforces the project constraint in
// docs/2026-08-29-design.md section 6 for this package's own source. This
// package generates no config, so there is nothing today to get wrong; the
// test exists so that the property is checked on every run rather than
// depending on whoever edits the package next remembering the rule.
func TestNoGoogleAnywhere(t *testing.T) {
	// Assembled from fragments so that this test file does not fail its own
	// check.
	needles := []string{
		"8.8." + "8.8",
		"8.8." + "4.4",
		"dns." + "google",
		"2001:4860:" + "4860",
		"google" + "apis.com",
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no source files found; the scan would pass vacuously")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		src := string(b)
		for _, n := range needles {
			if strings.Contains(src, n) {
				t.Errorf("%s contains %q; no Google, including as a resolver default", f, n)
			}
		}
	}
}

// TestForceLogToRing is the deterministic half of the log-capture guarantee.
// It checks the config rewrite directly rather than racing a goroutine for
// stdout, and it covers the privacy decision as well as the routing one.
func TestForceLogToRing(t *testing.T) {
	logDir := t.TempDir()
	accessPath := filepath.Join(logDir, "access.log")
	errorPath := filepath.Join(logDir, "error.log")

	cfgJSON := fmt.Sprintf(`{
  "log": {"access": %q, "error": %q, "loglevel": "debug", "dnsLog": true},
  "outbounds": [{"protocol": "freedom"}]
}`, accessPath, errorPath)

	cfg, err := loadConfig([]byte(cfgJSON))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	// Precondition: the loader really did honour the user's file paths, so the
	// assertions below are testing a change and not a coincidence.
	before := findLogConfig(t, cfg)
	if before.ErrorLogType != applog.LogType_File || before.ErrorLogPath != errorPath {
		t.Fatalf("loader did not set up a file error log: %+v", before)
	}
	if before.AccessLogType != applog.LogType_File || !before.EnableDnsLog {
		t.Fatalf("loader did not set up a file access log with dnsLog: %+v", before)
	}

	if err := forceLogToRing(cfg); err != nil {
		t.Fatalf("forceLogToRing: %v", err)
	}
	after := findLogConfig(t, cfg)

	if after.ErrorLogType != applog.LogType_Console {
		t.Errorf("ErrorLogType = %v, want Console (the creator that writes to the ring)", after.ErrorLogType)
	}
	if after.ErrorLogPath != "" {
		t.Errorf("ErrorLogPath = %q, want empty; the engine must not write a log file of its own", after.ErrorLogPath)
	}
	if after.AccessLogType != applog.LogType_None {
		t.Errorf("AccessLogType = %v, want None; access records are client destinations", after.AccessLogType)
	}
	if after.AccessLogPath != "" {
		t.Errorf("AccessLogPath = %q, want empty", after.AccessLogPath)
	}
	if after.EnableDnsLog {
		t.Error("EnableDnsLog is still set; DNS records are the names clients looked up")
	}
	if after.ErrorLogLevel != before.ErrorLogLevel {
		t.Errorf("ErrorLogLevel changed from %v to %v; the user's loglevel must survive", before.ErrorLogLevel, after.ErrorLogLevel)
	}

	// Nothing was written to either path.
	for _, p := range []string{accessPath, errorPath} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s exists; the engine still has a file log", p)
		}
	}
}

// TestForceLogToRingWithNoLogSection covers the branch where a caller hands us
// a config that never went through the loader, so no app/log config exists.
func TestForceLogToRingWithNoLogSection(t *testing.T) {
	cfg := &coreConfig{}
	if err := forceLogToRing(cfg); err != nil {
		t.Fatalf("forceLogToRing: %v", err)
	}
	got := findLogConfig(t, cfg)
	if got.ErrorLogType != applog.LogType_Console || got.AccessLogType != applog.LogType_None {
		t.Errorf("prepended config = %+v", got)
	}
}

func findLogConfig(t *testing.T, cfg *coreConfig) *applog.Config {
	t.Helper()
	want := typedMessageName(&applog.Config{})
	var found *applog.Config
	for _, app := range cfg.App {
		if app.GetType() != want {
			continue
		}
		inst, err := app.GetInstance()
		if err != nil {
			t.Fatalf("decoding the app/log config: %v", err)
		}
		lc, ok := inst.(*applog.Config)
		if !ok {
			t.Fatalf("app/log config decoded as %T", inst)
		}
		if found != nil {
			t.Fatal("more than one app/log config in cfg.App")
		}
		found = lc
	}
	if found == nil {
		t.Fatal("no app/log config in cfg.App")
	}
	return found
}
