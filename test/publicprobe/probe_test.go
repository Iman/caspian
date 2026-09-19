// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

// Package publicprobe drives share links from public lists through the real
// engine, end to end, and records which ones carry a request today.
//
// It is opt-in and off by default. It needs the network, a directory of link
// lists nobody commits (local/configs/public-configs, filled by the survey), and
// an environment variable, because every probe sends one small request through
// a stranger's server from this machine's own connection. A list that is
// downloaded, parsed and built proves the parser and the document builder; only
// this probe proves the engine connects, and only for the servers that answer at
// the moment it runs. Free servers churn daily, so the output is a snapshot with
// a date on it, never a claim about tomorrow.
//
//	CASPIAN_PUBLIC_PROBE_DIR=local/configs/public-configs go test ./test/publicprobe -run TestPublicConfigsConnect -v -count=1 -timeout 30m
//
// Optional: CASPIAN_PUBLIC_PROBE_MAX (default 200 links), CASPIAN_PUBLIC_PROBE_PARALLEL
// (default 4 engines at once), CASPIAN_PUBLIC_PROBE_TIMEOUT (default 12s per link).
//
// Origin mode, for a server of our own on the same LAN, where the exit address
// would be this household's and proves nothing: set CASPIAN_PUBLIC_PROBE_ORIGIN
// to a URL only the server's side can reach, its own loopback, and
// CASPIAN_PUBLIC_PROBE_TOKEN to the text that origin serves. A link connected
// when the token came back through the tunnel. Added 2026-09-13 for the
// Raspberry Pi used as a temporary server.
//
// What it writes, all under <dir>/working/, which lives in local/ and is
// gitignored: one file per protocol shape holding the links that connected, a
// results table, and a README with the date and the counts. It never writes or
// logs this machine's own address; the direct exit is read once, compared, and
// forgotten.
package publicprobe

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/proxy"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// echoURL is the same source A the hardware harness uses (test/hardware/lib/
// exitip.sh): Cloudflare's trace endpoint, pinned by address, answering a
// plain "ip=" line. Pinned so the probe does not depend on a resolver.
const echoURL = "https://1.1.1.1/cdn-cgi/trace"

var ipLine = regexp.MustCompile(`(?m)^ip=(\S+)$`)

type result struct {
	line  string
	shape string
	class string // "connected", or a failure class
	exit  string // the exit address the echo reported, empty on failure
}

func TestPublicConfigsConnect(t *testing.T) {
	dir := os.Getenv("CASPIAN_PUBLIC_PROBE_DIR")
	if dir == "" {
		t.Skip("set CASPIAN_PUBLIC_PROBE_DIR to a directory of public link lists; without it this " +
			"probe sends nothing anywhere and proves nothing")
	}
	dir = againstModuleRoot(dir)
	max := envInt("CASPIAN_PUBLIC_PROBE_MAX", 200)
	parallel := envInt("CASPIAN_PUBLIC_PROBE_PARALLEL", 4)
	timeout := time.Duration(envInt("CASPIAN_PUBLIC_PROBE_TIMEOUT", 12)) * time.Second
	// Origin mode, for a server we run ourselves on the same LAN. The exit
	// through such a server is this household's own address, so the exit
	// comparison cannot prove anything there. Instead the probe fetches an
	// origin that only the server's side of the tunnel can reach, the
	// server's own loopback, and looks for a token only that origin serves:
	// the same shape of proof test/tunnel uses on loopback.
	origin := os.Getenv("CASPIAN_PUBLIC_PROBE_ORIGIN")
	token := os.Getenv("CASPIAN_PUBLIC_PROBE_TOKEN")
	if (origin == "") != (token == "") {
		t.Fatal("CASPIAN_PUBLIC_PROBE_ORIGIN and CASPIAN_PUBLIC_PROBE_TOKEN go together")
	}

	lines := collectLinks(t, dir, max)
	if len(lines) == 0 {
		t.Fatalf("%s holds no share links", dir)
	}

	// The direct baseline: what this connection's exit looks like with no
	// tunnel. Held in memory for the comparison and never written anywhere.
	var direct string
	if origin == "" {
		var err error
		direct, err = echoDirect(timeout)
		if err != nil {
			t.Fatalf("the echo endpoint does not answer directly, so no exit can be compared: %v", err)
		}
		t.Logf("baseline read from %s; probing %d links, %d at a time, %s each", echoURL, len(lines), parallel, timeout)
	} else {
		t.Logf("origin mode: fetching %s through each tunnel and looking for its token; probing %d links, %d at a time, %s each", origin, len(lines), parallel, timeout)
	}

	results := make([]result, len(lines))
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	for i, ln := range lines {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, ln string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = probeOne(ln, direct, origin, token, timeout)
		}(i, ln)
	}
	wg.Wait()

	// Report by shape: counts only, no addresses.
	byShape := map[string]map[string]int{}
	connected := map[string][]string{}
	total := map[string]int{}
	for _, r := range results {
		if byShape[r.shape] == nil {
			byShape[r.shape] = map[string]int{}
		}
		byShape[r.shape][r.class]++
		total[r.class]++
		if r.class == "connected" {
			connected[r.shape] = append(connected[r.shape], r.line)
		}
	}
	var shapes []string
	for s := range byShape {
		shapes = append(shapes, s)
	}
	sort.Strings(shapes)
	var b strings.Builder
	fmt.Fprintf(&b, "\n%-32s %9s %5s  %s\n", "shape", "connected", "of", "failures")
	for _, s := range shapes {
		n := 0
		var fails []string
		for class, c := range byShape[s] {
			n += c
			if class != "connected" {
				fails = append(fails, fmt.Sprintf("%s=%d", class, c))
			}
		}
		sort.Strings(fails)
		fmt.Fprintf(&b, "%-32s %9d %5d  %s\n", s, byShape[s]["connected"], n, strings.Join(fails, " "))
	}
	var totals []string
	for class, c := range total {
		totals = append(totals, fmt.Sprintf("%s=%d", class, c))
	}
	sort.Strings(totals)
	fmt.Fprintf(&b, "total: %s of %d links; %s", "connected="+strconv.Itoa(total["connected"]), len(results), strings.Join(totals, " "))
	t.Log(b.String())

	writeWorking(t, dir, results, connected, b.String())
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// collectLinks reads every share line from the *.txt files directly in dir,
// skipping the working/ output of a previous run, up to max, in file order.
func collectLinks(t *testing.T, dir string, max int) []string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, "*.txt"))
	sort.Strings(files)
	scheme := regexp.MustCompile(`^[a-z0-9]+://`)
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, ln := range strings.Split(string(raw), "\n") {
			ln = strings.TrimSpace(ln)
			if !scheme.MatchString(ln) || seen[ln] {
				continue
			}
			seen[ln] = true
			out = append(out, ln)
			if len(out) >= max {
				return out
			}
		}
	}
	return out
}

// probeOne runs one link the way the appliance does, link.Parse, xcfg.Build,
// engine.Start, then one GET through the SOCKS inbound, and classifies what
// came back. The engine is stopped before it returns.
func probeOne(ln string, direct, origin, token string, timeout time.Duration) result {
	l, err := link.Parse(ln)
	if err != nil {
		return result{line: ln, shape: "unparsed", class: "parse"}
	}
	shape := l.Protocol + "/" + l.Network + "/" + string(l.Security)
	port, err := freePort()
	if err != nil {
		return result{line: ln, shape: shape, class: "no-port"}
	}
	o := xcfg.Defaults()
	o.Link = l
	o.TUN.Disabled = true
	o.SOCKS.Listen = "127.0.0.1"
	o.SOCKS.Port = uint16(port)
	doc, err := xcfg.Build(o)
	if err != nil {
		return result{line: ln, shape: shape, class: "build"}
	}
	e := engine.New()
	if err := e.Start(context.Background(), doc); err != nil {
		return result{line: ln, shape: shape, class: "engine-start"}
	}
	defer func() { _ = e.Stop() }()
	if !acceptsWithin(port, 5*time.Second) {
		return result{line: ln, shape: shape, class: "no-listener"}
	}
	if origin != "" {
		body, err := fetchThrough(port, origin, timeout)
		if err != nil {
			return result{line: ln, shape: shape, class: classify(err) + "/" + reachability(l, timeout)}
		}
		if !strings.Contains(body, token) {
			// Something answered through the tunnel, and it was not the origin.
			return result{line: ln, shape: shape, class: "wrong-origin"}
		}
		return result{line: ln, shape: shape, class: "connected", exit: "origin-token"}
	}
	exit, err := echoThrough(port, timeout)
	if err != nil {
		// Tell a dead server from a live one that would not carry us. A plain
		// TCP connect to the server's own address and port needs no
		// credential and no handshake; if that is refused or silent, the node
		// is gone and nothing on this side could have connected. If it accepts
		// and the tunnel still failed, the fault is behind the handshake:
		// rotated credentials, a name the server no longer serves, or a
		// client bug, and only a second client could tell those apart.
		return result{line: ln, shape: shape, class: classify(err) + "/" + reachability(l, timeout)}
	}
	if exit == direct {
		// The request came back with this machine's own address: the engine
		// answered but the server did not carry it, or the "proxy" is a
		// transparent hop. Not a working tunnel by this project's standard.
		return result{line: ln, shape: shape, class: "exit-unchanged"}
	}
	return result{line: ln, shape: shape, class: "connected", exit: exit}
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func acceptsWithin(port int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if err == nil {
			_ = c.Close()
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// echoThrough fetches the echo through the SOCKS inbound. The destination name
// travels inside the SOCKS request, so the engine resolves it through the
// tunnel, the way client traffic would.
func echoThrough(port int, timeout time.Duration) (string, error) {
	return echoWith(socksClient(port, timeout))
}

// fetchThrough performs one GET of url through the SOCKS inbound and returns
// the body. The destination name travels inside the SOCKS request, so the
// engine resolves it on the far side, the way client traffic would.
func fetchThrough(port int, url string, timeout time.Duration) (string, error) {
	resp, err := socksClient(port, timeout).Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}
	return string(body), nil
}

func socksClient(port int, timeout time.Duration) *http.Client {
	dialer, _ := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", port), nil, &net.Dialer{Timeout: timeout})
	cd, _ := dialer.(proxy.ContextDialer)
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           cd.DialContext,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
			DisableKeepAlives:     true,
		},
	}
}

func echoDirect(timeout time.Duration) (string, error) {
	return echoWith(&http.Client{Timeout: timeout})
}

func echoWith(client *http.Client) (string, error) {
	resp, err := client.Get(echoURL + "?caspian=" + strconv.FormatInt(time.Now().UnixNano(), 36))
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}
	m := ipLine.FindStringSubmatch(string(body))
	if m == nil {
		return "", fmt.Errorf("no ip= line in the echo body")
	}
	return m[1], nil
}

// classify reduces a transport error to a short class. The text is never
// kept: an error can name the server.
func classify(err error) string {
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "http "):
		return "http-status"
	case strings.Contains(s, "timeout") || strings.Contains(s, "deadline"):
		return "timeout"
	case strings.Contains(s, "refused"):
		return "refused"
	case strings.Contains(s, "eof") || strings.Contains(s, "reset"):
		return "closed"
	case strings.Contains(s, "tls") || strings.Contains(s, "certificate") || strings.Contains(s, "x509"):
		return "tls"
	case strings.Contains(s, "socks"):
		return "socks-refused"
	}
	return "other"
}

// writeWorking records the snapshot under dir/working: the links that connected
// by shape, a results table, and a dated README. Nothing about the machine that
// ran it is written.
func writeWorking(t *testing.T, dir string, results []result, connected map[string][]string, summary string) {
	t.Helper()
	out := filepath.Join(dir, "working")
	if err := os.MkdirAll(out, 0o700); err != nil {
		t.Fatal(err)
	}
	// Start clean: a previous snapshot's files must not survive into this one.
	old, _ := filepath.Glob(filepath.Join(out, "*.txt"))
	for _, f := range old {
		_ = os.Remove(f)
	}
	var shapes []string
	for s := range connected {
		shapes = append(shapes, s)
	}
	sort.Strings(shapes)
	for _, s := range shapes {
		name := strings.ReplaceAll(s, "/", "-") + ".txt"
		if err := os.WriteFile(filepath.Join(out, name), []byte(strings.Join(connected[s], "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var tsv strings.Builder
	tsv.WriteString("shape\tclass\texit\tlink\n")
	for _, r := range results {
		fmt.Fprintf(&tsv, "%s\t%s\t%s\t%s\n", r.shape, r.class, r.exit, r.line)
	}
	if err := os.WriteFile(filepath.Join(out, "probe-results.tsv"), []byte(tsv.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	readme := "# Public configs that connected\n\n" +
		"Snapshot taken " + time.Now().UTC().Format("2006-01-02 15:04 UTC") + " by test/publicprobe from a\n" +
		"developer machine, with the real engine, TUN off, one GET to " + echoURL + "\n" +
		"through each link's SOCKS inbound. \"Connected\" means the echo answered through the\n" +
		"tunnel with an exit address different from this machine's direct one. Free servers\n" +
		"churn daily; this is what answered at that moment, not a promise.\n\n" +
		"One file per protocol shape holds the links that connected. probe-results.tsv holds\n" +
		"every probed link with its class and the exit the echo reported. Everything here\n" +
		"stays in local/, which is gitignored.\n\n```" + summary + "\n```\n"
	if err := os.WriteFile(filepath.Join(out, "README.md"), []byte(readme), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d shape files, probe-results.tsv and README.md under %s", len(shapes), out)
}

// againstModuleRoot resolves a relative directory against the module root
// rather than the package directory, which is where go test runs. Without this,
// "local/configs/public-configs" silently points at test/publicprobe/local and
// the probe reports an empty directory, which is what happened on the first run.
func againstModuleRoot(dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	wd, err := os.Getwd()
	if err != nil {
		return dir
	}
	for d := wd; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return filepath.Join(d, dir)
		}
		if filepath.Dir(d) == d {
			return dir
		}
	}
}

// reachability reports "server-down" when the server's address and port do not
// accept a plain TCP connection, "server-up" when they do, and "udp" for
// Hysteria2, whose transport is QUIC and cannot be probed this way.
func reachability(l *link.Link, timeout time.Duration) string {
	if l.Protocol == "hysteria" {
		return "udp"
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort(l.Address, strconv.Itoa(int(l.Port))), timeout/2)
	if err != nil {
		return "server-down"
	}
	_ = c.Close()
	return "server-up"
}
