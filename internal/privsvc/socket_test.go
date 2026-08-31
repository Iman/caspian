// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/panel"
)

// shortTempDir returns a directory with a short path.
//
// t.TempDir() on macOS lives under /var/folders/<long>/T/<test name>/001, and a
// unix socket path is limited to about 104 bytes by the size of sun_path. A
// test that failed for that reason would look like a bug in this package.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "caspian-privsvc-")
	if err != nil {
		t.Fatalf("making a temporary directory: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// currentAccount is the account this test process runs as. Telling the listener
// to permit it is what makes the happy-path tests exercise the REAL peer
// credential syscall rather than a substitute for it.
func currentAccount(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatalf("looking up the current account: %v", err)
	}
	return u.Username
}

// serving starts a listener on a short path and returns its socket.
func serving(t *testing.T, w *world, cfg ListenConfig) string {
	t.Helper()
	if cfg.Path == "" {
		cfg.Path = filepath.Join(shortTempDir(t), "priv.sock")
	}
	ln, err := Listen(w.svc, cfg)
	if err != nil {
		t.Fatalf("opening the socket: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = ln.Serve(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return cfg.Path
}

// TestTheSocketAnswersEveryActionInTheClosedSet drives all five actions through
// a real socket, with the real peer credential check in force.
func TestTheSocketAnswersEveryActionInTheClosedSet(t *testing.T) {
	w := newWorld(t)
	path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})
	c := NewClient(path)
	ctx := context.Background()

	d, err := c.Detect(ctx)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if d.InternetInterface != "eth0" || d.HotspotInterface != "ap0" {
		t.Fatalf("detect reported internet %q and hotspot %q", d.InternetInterface, d.HotspotInterface)
	}

	if err := c.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v\ntimeline:%s", err, w.tl)
	}

	st, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !st.Connected() {
		t.Fatalf("status says the box is not carrying client traffic after a successful start: engine=%v hotspot=%+v",
			st.Engine.Phase, st.Hotspot)
	}
	if st.Hotspot.SSID != fakeSSID {
		t.Fatalf("status reported ssid %q, want %q", st.Hotspot.SSID, fakeSSID)
	}

	lg, err := c.EngineLog(ctx)
	if err != nil {
		t.Fatalf("engine log: %v", err)
	}
	if len(lg.Entries) == 0 || lg.Dropped != 3 {
		t.Fatalf("the engine log did not survive the socket: %d entries, %d dropped", len(lg.Entries), lg.Dropped)
	}

	if err := c.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if w.svc.isRunning() {
		t.Fatalf("the service still reports itself as running after a stop over the socket")
	}
}

// TestAFaultCrossesTheSocketAsItsOwnWord.
//
// The panel branches on the Fault, so a fault that arrives as
// panel.FaultUnknown sends the user to fix the wrong thing.
func TestAFaultCrossesTheSocketAsItsOwnWord(t *testing.T) {
	w := newWorld(t)
	failHostapd(w)
	path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})

	err := NewClient(path).Start(context.Background(), startRequest(t))
	if err == nil {
		t.Fatalf("start over the socket reported success with a hostapd that would not start")
	}
	if got := panel.FaultOf(err); got != panel.FaultHotspotFailed {
		t.Fatalf("the fault arrived as %q, want %q", got, panel.FaultHotspotFailed)
	}
}

// TestNothingBeyondTheClosedVocabularyIsAccepted sends every shape of message a
// caller could get wrong, and requires that none of them reaches the service.
//
// The check that nothing privileged ran is the point of the test. A refusal
// that happened AFTER the service had been touched would still look like a
// refusal from the outside.
func TestNothingBeyondTheClosedVocabularyIsAccepted(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		want    Refusal
	}{
		{"an action that is not one of the five", mustJSON(t, map[string]any{"v": protocolVersion, "action": "run"}), RefusalUnknownAction},
		{"an empty action", mustJSON(t, map[string]any{"v": protocolVersion, "action": ""}), RefusalUnknownAction},
		{"an action that looks like a command", mustJSON(t, map[string]any{"v": protocolVersion, "action": "/bin/sh"}), RefusalUnknownAction},
		{"a start with no argument", mustJSON(t, map[string]any{"v": protocolVersion, "action": "start"}), RefusalMissingArg},
		{"a stop carrying a start argument", mustJSON(t, map[string]any{
			"v": protocolVersion, "action": "stop", "start": map[string]any{},
		}), RefusalUnexpectedArg},
		{"a field this build does not know", mustJSON(t, map[string]any{
			"v": protocolVersion, "action": "status", "exec": []string{"/bin/sh", "-c", "id"},
		}), RefusalBadJSON},
		{"not JSON at all", []byte("this is not JSON"), RefusalBadJSON},
		{"JSON that is not an object", []byte(`["start"]`), RefusalBadJSON},
		{"a second message in one frame", []byte(`{"v":1,"action":"status"}{"v":1,"action":"stop"}`), RefusalBadJSON},
		{"the wrong protocol version", mustJSON(t, map[string]any{"v": protocolVersion + 1, "action": "status"}), RefusalBadVersion},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})

			resp := sendRaw(t, path, tc.payload)
			if resp.Refusal != tc.want {
				t.Fatalf("refusal was %q, want %q (fault %q)", resp.Refusal, tc.want, resp.Fault)
			}
			if n := len(w.runner.Commands()); n != 0 {
				t.Fatalf("%d commands ran on the machine for a message that was refused", n)
			}
			if w.eng.startCount() != 0 {
				t.Fatalf("the engine was started for a message that was refused")
			}
			if len(w.sys.Calls) != 0 {
				t.Fatalf("%d hotspot commands ran for a message that was refused", len(w.sys.Calls))
			}
		})
	}
}

// TestAMalformedFrameIsRefusedWithoutBeingRead covers the framing rather than
// the JSON: a length that says more than this service will ever accept, and a
// body shorter than the length claims.
func TestAMalformedFrameIsRefusedWithoutBeingRead(t *testing.T) {
	t.Run("a length larger than the limit", func(t *testing.T) {
		w := newWorld(t)
		path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})

		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close()

		var hdr [4]byte
		binary.BigEndian.PutUint32(hdr[:], maxFrameBytes+1)
		if _, err := conn.Write(hdr[:]); err != nil {
			t.Fatalf("write: %v", err)
		}
		// Deliberately no body. The length alone has to be enough to refuse,
		// which is the whole reason the frame carries one.
		_ = conn.(*net.UnixConn).SetReadDeadline(time.Now().Add(5 * time.Second))
		resp := readResponse(t, conn)
		if resp.Refusal != RefusalTooLarge {
			t.Fatalf("refusal was %q, want %q", resp.Refusal, RefusalTooLarge)
		}
		if n := len(w.runner.Commands()); n != 0 {
			t.Fatalf("%d commands ran on the machine for a frame that was refused", n)
		}
	})

	t.Run("a body shorter than the length claims", func(t *testing.T) {
		w := newWorld(t)
		path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t), ReadTimeout: 300 * time.Millisecond})

		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close()

		var hdr [4]byte
		binary.BigEndian.PutUint32(hdr[:], 100)
		if _, err := conn.Write(append(hdr[:], []byte(`{"v":1,`)...)); err != nil {
			t.Fatalf("write: %v", err)
		}
		_ = conn.(*net.UnixConn).SetReadDeadline(time.Now().Add(5 * time.Second))
		// The read deadline on the service side fires, so the answer is either
		// a bad-frame refusal or a closed connection. Both are refusals; what
		// must not happen is anything running.
		_, _ = io.ReadAll(conn)
		if n := len(w.runner.Commands()); n != 0 {
			t.Fatalf("%d commands ran on the machine for a truncated frame", n)
		}
	})
}

// TestAnUnauthorisedPeerIsRefused exercises the REAL peer credential syscall.
//
// The listener is told that only root may connect, and the test process is not
// root, so the kernel-recorded uid of this very connection is what refuses it.
// Nothing is substituted.
//
// The file modes on the socket keep other accounts out already. This check is
// stricter in one way that matters: docs/LAYOUT.md puts the socket at 0660
// root:caspian, which admits anyone in the caspian GROUP, and this admits only
// the caspian ACCOUNT and root.
func TestAnUnauthorisedPeerIsRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("this test proves a non-root account is refused, and it is running as root")
	}
	w := newWorld(t)
	// ServiceAccount empty, so the permitted set is root and nobody else.
	path := serving(t, w, ListenConfig{})

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	body := mustJSON(t, map[string]any{"v": protocolVersion, "action": "status"})
	if err := writeFrame(conn, body); err != nil {
		// A write to a socket the service has already closed is itself the
		// refusal, and is a pass.
		t.Logf("the service closed the connection before the request was written: %v", err)
	}
	_ = conn.(*net.UnixConn).SetReadDeadline(time.Now().Add(5 * time.Second))

	if _, err := readFrame(conn); err == nil {
		t.Fatalf("an account that may not drive this service got an answer")
	}
	if n := len(w.runner.Commands()); n != 0 {
		t.Fatalf("%d commands ran on the machine for a peer that was refused", n)
	}
}

// TestAnAuthorisedPeerIsAccepted is the other half of the same syscall: with
// this account named, the same connection is served.
//
// Without it, the test above would pass on a machine where the credential read
// simply failed for every connection, which is a boundary that refuses
// everybody and proves nothing.
func TestAnAuthorisedPeerIsAccepted(t *testing.T) {
	w := newWorld(t)
	path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})
	if _, err := NewClient(path).Status(context.Background()); err != nil {
		t.Fatalf("the account this service was told to permit was refused: %v", err)
	}
}

// TestTheSocketCarriesTheModeLayoutFixes.
func TestTheSocketCarriesTheModeLayoutFixes(t *testing.T) {
	w := newWorld(t)
	path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode()&fs.ModeSocket == 0 {
		t.Fatalf("%s is not a socket", path)
	}
	// Compared against the literal from docs/LAYOUT.md, "Paths", and NOT
	// against SocketMode. Asserting a constant equals itself is a test that
	// passes whatever the constant is changed to, which is what this one used
	// to do.
	const layoutFixes = 0o660
	if got := fi.Mode().Perm(); got != layoutFixes {
		t.Fatalf("the socket is mode %04o and docs/LAYOUT.md fixes %04o", got, layoutFixes)
	}
	if SocketMode.Perm() != layoutFixes {
		t.Fatalf("SocketMode is %04o and docs/LAYOUT.md fixes %04o", SocketMode.Perm(), layoutFixes)
	}
}

// TestASecondCopyIsRefusedRatherThanTakingOver.
//
// Removing a live socket would leave the first service running as root and
// serving nobody, while the second served the panel. The message names the
// situation because "bind: address already in use" is not something the person
// reading it can act on.
func TestASecondCopyIsRefusedRatherThanTakingOver(t *testing.T) {
	w := newWorld(t)
	path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})

	second := newWorld(t)
	_, err := Listen(second.svc, ListenConfig{Path: path, ServiceAccount: currentAccount(t)})
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("a second copy got %v, want ErrAlreadyRunning", err)
	}

	// The first is still answering.
	if _, err := NewClient(path).Status(context.Background()); err != nil {
		t.Fatalf("the running service stopped answering after a second copy tried to start: %v", err)
	}
}

// TestALeftoverSocketIsReplaced is the other side of the test above: a socket
// from a process that died has to be cleared, or the service can never restart.
func TestALeftoverSocketIsReplaced(t *testing.T) {
	dir := shortTempDir(t)
	path := filepath.Join(dir, "priv.sock")

	// A socket file with nothing behind it, which is what a killed process
	// leaves on a filesystem that is not a tmpfs.
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("making a stale socket: %v", err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false)
	stale.Close()

	w := newWorld(t)
	serving(t, w, ListenConfig{Path: path, ServiceAccount: currentAccount(t)})
	if _, err := NewClient(path).Status(context.Background()); err != nil {
		t.Fatalf("the service did not come up over a leftover socket: %v", err)
	}
}

// TestSomethingThatIsNotASocketIsLeftAlone.
func TestSomethingThatIsNotASocketIsLeftAlone(t *testing.T) {
	dir := shortTempDir(t)
	path := filepath.Join(dir, "priv.sock")
	if err := os.WriteFile(path, []byte("somebody else's file"), 0o600); err != nil {
		t.Fatalf("writing a file in the way: %v", err)
	}

	w := newWorld(t)
	if _, err := Listen(w.svc, ListenConfig{Path: path}); err == nil {
		t.Fatalf("a regular file at the socket path was accepted")
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "somebody else's file" {
		t.Fatalf("the file that was in the way was changed or removed: %q, %v", b, err)
	}
}

// TestTheClientReportsAnAbsentServiceAsUnavailable.
//
// internal/panel needs this exact fault to draw a panel that says the box is
// not answering rather than failing to draw at all.
func TestTheClientReportsAnAbsentServiceAsUnavailable(t *testing.T) {
	path := filepath.Join(shortTempDir(t), "nothing-here.sock")
	c := NewClient(path)
	_, err := c.Status(context.Background())
	if got := panel.FaultOf(err); got != panel.FaultUnavailable {
		t.Fatalf("a missing service reported %q, want %q", got, panel.FaultUnavailable)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encoding a test message: %v", err)
	}
	return b
}

// sendRaw writes one frame and reads the answer, bypassing Client so that a
// message Client would never build can still be sent.
func sendRaw(t *testing.T, path string, payload []byte) wireResponse {
	t.Helper()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if err := writeFrame(conn, payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.(*net.UnixConn).SetReadDeadline(time.Now().Add(5 * time.Second))
	return readResponse(t, conn)
}

func readResponse(t *testing.T, conn net.Conn) wireResponse {
	t.Helper()
	b, err := readFrame(conn)
	if err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	var resp wireResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}
	return resp
}
