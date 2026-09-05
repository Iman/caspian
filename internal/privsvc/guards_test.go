// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// TestMissingKnobsNoticesAKnobWithNoMeasuredValue.
//
// This guards a branch that is UNREACHABLE today and is kept on purpose. Every
// knob a plan changes is currently global, so the first detection always has
// all of them and the second read never runs. The branch exists for the day a
// plan needs a knob that names an interface, and internal/netcfg is explicit
// about what goes wrong then: "A knob with no measured value gets no inverse,
// and teardown then cannot put it back."
//
// A helper whose polarity was wrong would disable that future re-read silently,
// and no other test in this package could notice, because no fixture can reach
// the branch. So the helper is tested directly.
func TestMissingKnobsNoticesAKnobWithNoMeasuredValue(t *testing.T) {
	f := netcfg.Facts{Sysctl: map[string]string{
		"net.ipv4.ip_forward":         "0",
		"net.ipv4.conf.all.rp_filter": "1",
	}}
	if missingKnobs(f, []string{"net.ipv4.ip_forward", "net.ipv4.conf.all.rp_filter"}) {
		t.Fatalf("a knob set that was fully measured was reported as missing one")
	}
	if !missingKnobs(f, []string{"net.ipv4.ip_forward", "net.ipv4.conf.wlan0.rp_filter"}) {
		t.Fatalf("a knob with no measured value was not noticed, so it would be changed with no inverse to restore it")
	}
	if missingKnobs(f, nil) {
		t.Fatalf("an empty knob set was reported as missing one")
	}
}

// TestPrivsvcReadsNoStateFile.
//
// docs/LAYOUT.md, "Who writes what": the panel process owns state.json and the
// privileged process owns netcfg.journal, and neither writes the other's file.
// internal/state is imported here for two string constants; this is the guard
// that the import never becomes a read.
//
// It scans the source rather than asserting on behaviour, because the behaviour
// it forbids is one nobody would write a passing test for: a call added in a
// later change would simply work.
func TestPrivsvcReadsNoStateFile(t *testing.T) {
	banned := []string{"state.Load(", "state.Store", "state.FileName", "state.DefaultDir", "\"state.json\""}
	for _, name := range packageSourceFiles(t) {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		src := string(b)
		for _, bad := range banned {
			if strings.Contains(src, bad) {
				t.Errorf("%s mentions %q. The privileged side receives everything it needs in the request "+
					"and reads no state file (docs/LAYOUT.md, \"Who writes what\")", filepath.Base(name), bad)
			}
		}
	}
}

// packageSourceFiles lists this package's non-test .go files.
func packageSourceFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		t.Fatalf("no source files found, so this test would pass on an empty package")
	}
	return out
}

// TestConfigDocumentSurvivesTheRoundTrip is the measurement the whole start
// path rests on.
//
// internal/xcfg composes the complete engine document from a *link.Link, and a
// *link.Link can only be produced by link.Parse: its outbound is an unexported
// field so that the credential material cannot be reached by encoding/json or
// by a fmt verb. What crosses the socket is the OUTPUT of that type. So the
// privileged side re-parses the document to get a link back.
//
// If this ever stops holding, the privileged side cannot compose an engine
// document at all, and the failure would be a box that starts an engine with
// outbounds and no inbound: a tunnel that carries nothing, reported as running.
func TestConfigDocumentSurvivesTheRoundTrip(t *testing.T) {
	first, err := link.Parse(realityShareLink())
	if err != nil {
		t.Fatalf("parsing the share link: %v", err)
	}
	doc, err := first.XrayConfig()
	if err != nil {
		t.Fatalf("building the document: %v", err)
	}

	again, err := configFromRequest(doc)
	if err != nil {
		t.Fatalf("the document internal/link produced could not be read back: %v", err)
	}

	for _, f := range []struct {
		what      string
		got, want any
	}{
		{"protocol", again.Protocol, first.Protocol},
		{"address", again.Address, first.Address},
		{"port", again.Port, first.Port},
		{"transport", again.Network, first.Network},
		{"security", again.Security, first.Security},
		{"server name", again.ServerName, first.ServerName},
		{"fingerprint", again.Fingerprint, first.Fingerprint},
		{"flow", again.Flow, first.Flow},
		{"reality material", again.Reality, first.Reality},
	} {
		if again.Protocol == "" || f.got != f.want {
			t.Errorf("the %s did not survive the round trip: got %v, want %v", f.what, f.got, f.want)
		}
	}

	// And the document is stable, so a second start composes the same bytes.
	doc2, err := again.XrayConfig()
	if err != nil {
		t.Fatalf("re-serialising: %v", err)
	}
	if string(doc2) != string(doc) {
		t.Fatalf("the document is not stable across a round trip:\nfirst:  %s\nsecond: %s", doc, doc2)
	}
}

// TestTheRoundTripRunsInternalLinksValidationAgain.
//
// The re-parse is not plumbing: it is the validation this boundary owes.
// xray-core turns any 1-to-30-character string that is not a UUID into a
// DIFFERENT valid UUID by SHA-1 with no error, so a truncated id authenticates
// as somebody else and presents as "connected but nothing works". The panel
// refuses that. This side must not depend on the panel having done so.
func TestTheRoundTripRunsInternalLinksValidationAgain(t *testing.T) {
	l, err := link.Parse(realityShareLink())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := l.XrayConfig()
	if err != nil {
		t.Fatalf("document: %v", err)
	}

	for _, tc := range []struct {
		name    string
		mutated string
	}{
		{"an id that is not a UUID", strings.Replace(string(doc), `"id":"`+fakeUUID+`"`, `"id":"short"`, 1)},
		{"no server address", strings.Replace(string(doc), `"address":"`+fakeHost+`"`, `"address":""`, 1)},
		{"port zero", strings.Replace(string(doc), `"port":443`, `"port":0`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mutated == string(doc) {
				t.Fatalf("the mutation changed nothing, so this case proves nothing")
			}
			if _, err := configFromRequest([]byte(tc.mutated)); err == nil {
				t.Fatalf("a document with %s was accepted", tc.name)
			}
		})
	}
}

// TestTheRealEngineSatisfiesTheInterface. The Engine interface exists so the
// ordering around the tunnel device can be observed; it is worth nothing if the
// real engine does not fit it.
func TestTheRealEngineSatisfiesTheInterface(t *testing.T) {
	var _ Engine = engine.New()
}

// TestTheRealEngineLoadsAndStartsTheComposedDocument.
//
// The other tests drive a recording engine, which cannot fail the way the real
// one fails. This one hands the real engine the document this service composes.
//
// WHAT IT DOES NOT PROVE, and the limit is the same one internal/engine and
// test/bdd record: the TUN inbound is off, because a developer machine has no
// /dev/net/tun and no root. So the engine that starts here has a loopback SOCKS
// inbound and a loopback DNS listener and NO way for client traffic to reach
// it. Nothing here is evidence that the tunnel carries traffic; that needs an
// exit IP captured on a real box.
func TestTheRealEngineLoadsAndStartsTheComposedDocument(t *testing.T) {
	socks := freePort(t)
	dns := freePort(t)

	w := newWorld(t, func(w *world) {
		w.cfg.Engine = engine.NewWithLogCapacity(32)
		w.cfg.SocksPort = socks
		w.cfg.LocalDNSPort = dns
	})

	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("the real engine would not take the document this service composed: %v", err)
	}
	st, err := w.svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Engine.Phase != engine.PhaseRunning {
		t.Fatalf("the engine is %v, want running: %s", st.Engine.Phase, st.Engine.Reason)
	}

	// The listener is really there, on the port dnsmasq was told to forward to.
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(dns))), 2*time.Second)
	if err != nil {
		t.Fatalf("nothing is listening on %d, which is the port the hotspot forwards client DNS to: %v", dns, err)
	}
	c.Close()

	if err := w.svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

// TestTheComposedDocumentCarriesTheTunInboundOnTheAppliance.
//
// Every other test in this package sets TUNDisabled, because a developer
// machine cannot open /dev/net/tun. That makes it worth asserting once, on the
// document rather than on a running engine, that the appliance's document
// carries the inbound client traffic actually arrives on. Without it the box
// starts an engine, reports itself running, and carries nothing.
func TestTheComposedDocumentCarriesTheTunInboundOnTheAppliance(t *testing.T) {
	w := newWorld(t, func(w *world) { w.cfg.TUNDisabled = false })

	l, err := link.Parse(realityShareLink())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := w.svc.engineDocument(l, startRequest(t), w.cfg.netOptions())
	if err != nil {
		t.Fatalf("composing: %v", err)
	}
	for _, want := range []string{
		`"protocol": "tun"`,
		`"name": "` + w.cfg.TunName + `"`,
		`"tag": "` + xcfg.TagTUNIn + `"`,
		`"ruleTag": "client-dns-intercept"`,
	} {
		if !strings.Contains(string(doc), want) {
			t.Fatalf("the document has no %s, so client traffic has no way in:\n%s", want, doc)
		}
	}

	// And the engine will take it, which is the check that the inbound is
	// registered in this build rather than merely spelled correctly.
	if err := engine.Validate(doc); err != nil {
		t.Fatalf("the engine refused a document carrying the TUN inbound: %v", err)
	}
}

// TestTheTunnelDeviceHasOneNameAcrossBothPackages.
//
// internal/netcfg writes an address on the tunnel device and routes through it
// BY NAME; internal/xcfg tells the engine to create it. A drift between them is
// a tunnel the routes do not name, which presents as a box that connects and
// carries nothing.
func TestTheTunnelDeviceHasOneNameAcrossBothPackages(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.cfg.TunName = "casp0"
		w.cfg.TUNDisabled = false
	})

	l, err := link.Parse(realityShareLink())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := w.svc.engineDocument(l, startRequest(t), w.cfg.netOptions())
	if err != nil {
		t.Fatalf("composing: %v", err)
	}
	if !strings.Contains(string(doc), `"name": "casp0"`) {
		t.Fatalf("the engine was told to create a device other than the one netcfg was configured for:\n%s", doc)
	}

	if err := w.svc.Start(context.Background(), startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if w.tl.indexOf("ip route add default dev casp0") < 0 {
		t.Fatalf("the routes name a different device from the one the engine was told to create\ntimeline:%s", w.tl)
	}
}

// TestNoCredentialReachesALogLine.
//
// docs/LAYOUT.md and design section 9 both require it, and the values this
// service handles are the worst ones: the pasted configuration and the WPA2
// passphrase. The logger is at Debug level so nothing is skipped for being
// verbose.
func TestNoCredentialReachesALogLine(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()

	// A successful start, a repeat, a change, a failure and a stop: every path
	// that logs anything.
	if err := w.svc.Start(ctx, startRequest(t)); err != nil {
		t.Fatalf("start: %v", err)
	}
	_ = w.svc.Start(ctx, startRequest(t))
	changed := startRequest(t)
	changed.Hotspot.SSID = "Caspian-Kitchen"
	_ = w.svc.Start(ctx, changed)
	bad := startRequest(t)
	bad.Hotspot.Passphrase = "SecurePass123"
	_ = w.svc.Start(ctx, bad)
	_ = w.svc.Stop(ctx)

	logged := w.logs.String()
	if logged == "" {
		t.Fatalf("nothing was logged at all, so this test would pass on a service that logs nothing")
	}
	for _, s := range secrets() {
		if strings.Contains(logged, s) {
			t.Fatalf("a credential reached a log line: %q\nlog:\n%s", s, logged)
		}
	}
	// And the whole configuration document, in case a future change logs the
	// request rather than a field of it.
	if strings.Contains(logged, string(startRequest(t).ConfigJSON)) {
		t.Fatalf("the whole configuration document reached a log line")
	}
}

// TestNoCredentialCrossesBackOverTheSocket.
//
// The request carries two credentials INTO the service. Nothing carries either
// of them back out, on any path, including the failure paths where an error
// would carry the most detail.
func TestNoCredentialCrossesBackOverTheSocket(t *testing.T) {
	w := newWorld(t)
	failHostapd(w)
	path := serving(t, w, ListenConfig{ServiceAccount: currentAccount(t)})

	var replies []string
	send := func(req wireRequest) {
		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close()
		req.Version = protocolVersion
		body, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		if err := writeFrame(conn, body); err != nil {
			t.Fatalf("write: %v", err)
		}
		_ = conn.(*net.UnixConn).SetReadDeadline(time.Now().Add(10 * time.Second))
		b, err := readFrame(conn)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		replies = append(replies, string(b))
	}

	req := startRequest(t)
	send(wireRequest{Action: panel.ActionStart, Start: &req})
	send(wireRequest{Action: panel.ActionStatus})
	send(wireRequest{Action: panel.ActionDetect})
	send(wireRequest{Action: panel.ActionEngineLog})
	send(wireRequest{Action: panel.ActionStop})

	all := strings.Join(replies, "\n")
	for _, s := range secrets() {
		if strings.Contains(all, s) {
			t.Fatalf("a credential came back over the socket: %q\nreplies:\n%s", s, all)
		}
	}
}

// TestTheWireVocabularyIsExactlyThePanelActionSet.
//
// The names on the wire are internal/panel's, taken from panel.Actions rather
// than listed here, so an action added there is carried without an edit. What
// this checks is that nothing ELSE is accepted.
func TestTheWireVocabularyIsExactlyThePanelActionSet(t *testing.T) {
	for _, a := range panel.Actions {
		if !knownAction(a) {
			t.Errorf("%q is a panel action and the wire does not accept it", a)
		}
	}
	for _, notAnAction := range []string{
		"", "run", "exec", "detect ", "DETECT", "engine_log", "shell", "/bin/sh", "start;stop",
	} {
		if knownAction(panel.Action(notAnAction)) {
			t.Errorf("%q is accepted on the wire and is not one of the five actions", notAnAction)
		}
	}
}

// TestTheWireCannotExpressACommand walks the request type looking for the
// shapes that would let a caller name something to run.
//
// internal/panel has the same test over its own interface. This is the other
// end of the same rule, on the type that is actually decoded from bytes a
// caller sent: "A privileged helper that takes a path and an argument list from
// its client is not a boundary; it is a way to run anything as root."
func TestTheWireCannotExpressACommand(t *testing.T) {
	banned := []string{"cmd", "command", "argv", "args", "arguments", "exec", "shell", "script", "path", "binary", "program"}

	var walk func(rt reflect.Type, where string, depth int)
	walk = func(rt reflect.Type, where string, depth int) {
		if depth > 6 {
			return
		}
		for rt.Kind() == reflect.Pointer {
			rt = rt.Elem()
		}
		if rt.Kind() == reflect.Slice && rt.Elem().Kind() == reflect.String {
			t.Errorf("%s is a []string, which is the shape of an argument vector", where)
			return
		}
		if rt.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			lower := strings.ToLower(f.Name)
			for _, b := range banned {
				if lower == b {
					t.Errorf("%s.%s is named like a command, a path or an argument list", where, f.Name)
				}
			}
			walk(f.Type, where+"."+f.Name, depth+1)
		}
	}
	walk(reflect.TypeOf(wireRequest{}), "wireRequest", 0)
}

// TestADeadlineFromACallerIsClamped.
//
// A caller cannot pin a root process to one operation for ever, and cannot cut
// one so short that it is guaranteed to be interrupted half way through
// reconfiguring the network, which is the state this service exists not to
// leave.
func TestADeadlineFromACallerIsClamped(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"no deadline at all", 0, defaultDeadline},
		{"one already in the past", -time.Hour, MinDeadline},
		{"one shorter than the floor", time.Second, MinDeadline},
		{"one inside the bounds", 30 * time.Second, 30 * time.Second},
		{"one longer than the ceiling", 24 * time.Hour, MaxDeadline},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var unixNano int64
			if tc.in != 0 {
				unixNano = now.Add(tc.in).UnixNano()
			}
			got := deadlineFrom(unixNano, now).Sub(now)
			if got != tc.want {
				t.Fatalf("a deadline of %v became %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestAllowedPermitsOnlyRootAndTheServiceAccount.
func TestAllowedPermitsOnlyRootAndTheServiceAccount(t *testing.T) {
	a, err := AllowedFor("")
	if err != nil {
		t.Fatalf("AllowedFor(\"\"): %v", err)
	}
	if !a.Permits(Peer{UID: 0}) {
		t.Fatalf("root is not permitted")
	}
	for _, uid := range []uint32{1, 500, 501, 1000, 65534} {
		if a.Permits(Peer{UID: uid}) {
			t.Fatalf("uid %d is permitted with no service account named", uid)
		}
	}

	a, err = AllowedFor("a-account-that-does-not-exist-here")
	if err == nil {
		t.Fatalf("a service account that does not exist was resolved without complaint")
	}
	if !a.Permits(Peer{UID: 0}) {
		t.Fatalf("root lost access because the service account could not be resolved")
	}
	if len(a.UIDs) != 1 {
		t.Fatalf("an unresolvable service account added %d ids to the permitted set", len(a.UIDs)-1)
	}
}

// TestFramingRoundTrip.
func TestFramingRoundTrip(t *testing.T) {
	var buf strings.Builder
	body := []byte(`{"v":1,"action":"status"}`)
	if err := writeFrame(&buf, body); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readFrame(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("round trip changed the message: %q", got)
	}

	if err := writeFrame(&buf, make([]byte, maxFrameBytes+1)); err == nil {
		t.Fatalf("a message larger than the limit was written")
	}
	if _, err := readFrame(strings.NewReader("\x00\x00\x00\x00")); err == nil {
		t.Fatalf("an empty message was accepted")
	}
}

// TestEverySourceFileCarriesTheLicenceHeader.
//
// The project is AGPL-3.0-or-later and every other package states it per file.
func TestEverySourceFileCarriesTheLicenceHeader(t *testing.T) {
	for _, name := range packageSourceFiles(t) {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if !strings.HasPrefix(string(b), "// SPDX-License-Identifier: AGPL-3.0-or-later") {
			t.Errorf("%s does not begin with the licence header", name)
		}
	}
}

// TestNoSourceFileCarriesAnEscapeCodeAnEmojiOrAnEmDash.
//
// The same check packaging/test-install.sh makes over the shipped scripts. An
// escape code in a message printed to a terminal is a control sequence that
// terminal will act on.
func TestNoSourceFileCarriesAnEscapeCodeAnEmojiOrAnEmDash(t *testing.T) {
	for _, name := range packageSourceFiles(t) {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		s := string(b)
		if strings.ContainsRune(s, 0x1b) {
			t.Errorf("%s contains an escape code", name)
		}
		if strings.Contains(s, "\u2014") || strings.Contains(s, "\u2013") {
			t.Errorf("%s contains an em dash", name)
		}
		for _, r := range s {
			if r >= 0x1F300 && r <= 0x1FAFF {
				t.Errorf("%s contains an emoji", name)
				break
			}
		}
	}
}

// TestEveryExportedTypeHasADocComment is a small discipline check: this package
// is the privilege boundary, and an exported name here with no explanation is
// one the next reader has to guess at.
func TestEveryExportedTypeHasADocComment(t *testing.T) {
	fset := token.NewFileSet()
	for _, name := range packageSourceFiles(t) {
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			gd, ok := n.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				return true
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !ts.Name.IsExported() {
					continue
				}
				if gd.Doc == nil && ts.Doc == nil {
					t.Errorf("%s: exported type %s has no doc comment", name, ts.Name.Name)
				}
			}
			return true
		})
	}
}
