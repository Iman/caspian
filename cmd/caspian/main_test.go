// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/privsvc"
)

// TestTheSubcommandsAreTheOnesLayoutFixes.
//
// docs/LAYOUT.md, "Two processes, one binary", names two of them by their exact
// command line, and the systemd units in packaging/ carry those lines. A binary
// that renamed one would install cleanly and fail to start with a usage message
// nobody sees.
func TestTheSubcommandsAreTheOnesLayoutFixes(t *testing.T) {
	units := map[string]string{
		"packaging/caspian.service":       "ExecStart=/usr/local/bin/caspian serve --privileged",
		"packaging/caspian-panel.service": "ExecStart=/usr/local/bin/caspian serve --panel",
	}
	for path, want := range units {
		b, err := os.ReadFile("../../" + path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		if !strings.Contains(string(b), want) {
			t.Fatalf("%s does not start this binary with %q", path, want)
		}
	}

	// And the usage this binary prints offers the same two, so somebody who
	// reads the unit and somebody who runs the binary are told the same thing.
	var usageText strings.Builder
	usage(&usageText)
	for _, want := range []string{"caspian serve --privileged", "caspian serve --panel", "caspian check", "caspian version"} {
		if !strings.Contains(usageText.String(), want) {
			t.Errorf("the usage does not offer %q", want)
		}
	}
}

// TestThePanelTimeoutIsOneThePrivilegedServiceWillHonour.
//
// The privileged service clamps whatever deadline it is given
// (internal/privsvc, MinDeadline and MaxDeadline). A panel timeout outside
// those bounds is one the two halves disagree about SILENTLY: the panel would
// wait for an operation the service had already abandoned, and the user would
// see a page that never comes back.
func TestThePanelTimeoutIsOneThePrivilegedServiceWillHonour(t *testing.T) {
	if privCallTimeout < privsvc.MinDeadline {
		t.Fatalf("the panel's timeout is %v and the privileged service raises anything below %v, "+
			"so a start would be given longer than the panel waits", privCallTimeout, privsvc.MinDeadline)
	}
	if privCallTimeout > privsvc.MaxDeadline {
		t.Fatalf("the panel's timeout is %v and the privileged service abandons an operation after %v, "+
			"so the panel would wait %v for an answer that is never coming",
			privCallTimeout, privsvc.MaxDeadline, privCallTimeout-privsvc.MaxDeadline)
	}
	// And it is generous enough to be worth overriding internal/panel's
	// default for at all.
	if privCallTimeout <= 20*time.Second {
		t.Fatalf("the panel's timeout is %v, which is no more than internal/panel's own default; "+
			"either raise it or stop overriding the default", privCallTimeout)
	}
}

// TestUsageErrorsAreRefusedAndSayWhy.
func TestUsageErrorsAreRefusedAndSayWhy(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no arguments at all", nil, "caspian serve --privileged"},
		{"a subcommand that does not exist", []string{"start"}, "is not a caspian command"},
		{"serve with neither role", []string{"serve"}, "needs --privileged or --panel"},
		{"serve with both roles", []string{"serve", "--privileged", "--panel"}, "two different accounts"},
		{"an option serve does not have", []string{"serve", "--panel", "--verbose"}, "is not an option"},
		{"listen with no address", []string{"serve", "--panel", "--listen"}, "needs an address"},
		{"listen on the privileged role", []string{"serve", "--privileged", "--listen", "127.0.0.1:8088"}, "belongs to"},
		{"an option check does not have", []string{"check", "--json"}, "is not an option"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := run(tc.args, &stdout, &stderr)
			if code != exitUsage {
				t.Fatalf("exit code %d, want %d (%s)", code, exitUsage, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("the message does not contain %q:\n%s", tc.want, stderr.String())
			}
		})
	}
}

// TestAWildcardOrPublicListenAddressIsRefused.
//
// design section 5.6: the hotspot interface always, the local network only if
// the user turns it on, never the uplink. The escape hatch cannot be a way
// round that, so it is validated by internal/panel's own rule.
func TestAWildcardOrPublicListenAddressIsRefused(t *testing.T) {
	for _, addr := range []string{
		"0.0.0.0:8088",
		"[::]:8088",
		":8088",
		"93.184.216.34:8088",
		"caspian.local:8088",
		"127.0.0.1:0",
	} {
		t.Run(addr, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := run([]string{"serve", "--panel", "--listen", addr}, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("exit code %d, want %d: %s", code, exitError, stderr.String())
			}
			if !strings.Contains(stderr.String(), "--listen "+addr+" was refused") {
				t.Fatalf("the refusal does not name the address:\n%s", stderr.String())
			}
		})
	}
}

// TestVersionPrintsWhatThisBinaryIs.
func TestVersionPrintsWhatThisBinaryIs(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run([]string{"version"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit code %d: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"caspian ", "engine:", "github.com/xtls/xray-core", "go:", "platform:"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output has no %q:\n%s", want, out)
		}
	}
}

// TestCheckChangesNothingAndSaysWhatItLookedAt.
//
// It is the command somebody reaches for when the box is not working, so it has
// to run on a machine where nothing is installed and still produce a report.
func TestCheckChangesNothingAndSaysWhatItLookedAt(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := runCheck(nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit code %d: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"This binary",
		"Programs this box needs",
		"Paths, against what docs/LAYOUT.md fixes",
		"Ports, from docs/LAYOUT.md",
		"The privileged service, asked over the socket",
		"This machine, measured directly by this command",
		"Stored settings",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report has no %q section:\n%s", want, out)
		}
	}
	// The placeholder used to make the planner run has to be labelled where a
	// reader will see it, or the report claims a route to an address nobody
	// owns.
	if strings.Contains(out, "192.0.2.1") && !strings.Contains(out, "PLACEHOLDER") {
		t.Errorf("the report uses a placeholder server address without saying so")
	}
	if strings.Contains(out, "Stored settings") && !strings.Contains(out, "proxy.raw=[redacted]") {
		t.Errorf("the stored-settings section is not the redacted rendering:\n%s", out)
	}
}

// TestCheckNeverPrintsTheStoredConfig.
//
// docs/LAYOUT.md: the user's proxy config is never printed or logged. This
// command reads the state file, so it is the one place in this binary that
// could.
func TestCheckNeverPrintsTheStoredConfig(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := runCheck(nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit code %d: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, scheme := range []string{"vless://", "vmess://", "trojan://", "ss://", "hysteria2://"} {
		if strings.Contains(out, scheme) {
			t.Errorf("the report contains what looks like a share link (%s)", scheme)
		}
	}
}

// ---------------------------------------------------------------------------
// docs/LAYOUT.md is binding. These read it.
// ---------------------------------------------------------------------------

// TestLayoutPortsMatchTheDocument reads the ports table out of docs/LAYOUT.md
// and compares it with the values this command passes in.
//
// LAYOUT.md says the values are fixed there "so no package has to learn them
// from another package's test fixture", and that "cmd/caspian reads these and
// passes them in". This is that reading, made checkable.
func TestLayoutPortsMatchTheDocument(t *testing.T) {
	doc := readLayout(t)

	want := map[int]string{
		dnsPort:      "dnsmasq",
		localDNSPort: "DNS listener",
		panelPort:    "panel",
		socksPort:    "SOCKS",
	}
	for port, what := range want {
		row := regexp.MustCompile(`(?m)^\|\s*` + strconv.Itoa(port) + `\s*\|`)
		if !row.MatchString(doc) {
			t.Errorf("docs/LAYOUT.md has no ports row for %d, which this command uses for the %s", port, what)
		}
	}

	// And nothing else is in that table, so a port added to the document
	// without being passed in here is a failing test rather than a service
	// listening somewhere nobody wrote down.
	section := between(t, doc, "## Ports", "## Two processes")
	rows := regexp.MustCompile(`(?m)^\|\s*(\d+)\s*\|`).FindAllStringSubmatch(section, -1)
	if len(rows) == 0 {
		t.Fatalf("no ports were found in the document's Ports table, so this test proves nothing")
	}
	for _, r := range rows {
		p, err := strconv.Atoi(r[1])
		if err != nil {
			continue
		}
		if _, ok := want[p]; !ok {
			t.Errorf("docs/LAYOUT.md fixes port %d and this command does not pass it in anywhere", p)
		}
	}
}

// TestLayoutPathsMatchTheDocument does the same for the paths this command
// names.
func TestLayoutPathsMatchTheDocument(t *testing.T) {
	doc := readLayout(t)
	for _, p := range []string{socketPath, stateDir, journalPath} {
		if !strings.Contains(doc, "`"+p+"`") {
			t.Errorf("docs/LAYOUT.md does not name %s, which this command uses", p)
		}
	}
	// The first-run password file is docs/INSTALL.md's, not LAYOUT.md's.
	install, err := os.ReadFile("../../docs/INSTALL.md")
	if err != nil {
		t.Fatalf("reading docs/INSTALL.md: %v", err)
	}
	if !strings.Contains(string(install), firstRunPasswordPath) {
		t.Errorf("docs/INSTALL.md does not name %s, which the panel role consumes", firstRunPasswordPath)
	}
}

// TestTheServiceAccountIsTheOneLayoutFixes.
func TestTheServiceAccountIsTheOneLayoutFixes(t *testing.T) {
	doc := readLayout(t)
	if !strings.Contains(doc, "| Service user and group | `"+serviceAccount+"`") {
		t.Errorf("docs/LAYOUT.md does not fix the service account as %q", serviceAccount)
	}
	if serviceGroup != serviceAccount {
		t.Errorf("the service user and group are one entry in docs/LAYOUT.md and differ here: %q and %q",
			serviceAccount, serviceGroup)
	}
}

func readLayout(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../docs/LAYOUT.md")
	if err != nil {
		t.Fatalf("reading docs/LAYOUT.md: %v", err)
	}
	return string(b)
}

func between(t *testing.T, doc, from, to string) string {
	t.Helper()
	i := strings.Index(doc, from)
	if i < 0 {
		t.Fatalf("docs/LAYOUT.md has no %q section", from)
	}
	rest := doc[i:]
	if j := strings.Index(rest, to); j > 0 {
		return rest[:j]
	}
	return rest
}
