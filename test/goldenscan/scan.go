// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

// Package goldenscan is the leak scan over every committed fixture in this
// repository.
//
// # Why this exists at all
//
// A golden file is committed. It is therefore permanent: once a credential
// reaches git history, removing it from the working tree removes nothing. The
// panel renders the hotspot passphrase; the state file holds the pasted proxy
// config with its user id, its REALITY keys and its server address; the engine
// configuration document is BUILT out of those credentials. Every one of those
// is one careless -update away from a committed golden.
//
// This package is the after-the-fact sweep. It is modelled on hw_leakscan in
// test/hardware/lib/common.sh, which does the same job for the hardware
// harness's output directories, and it keeps that file's two best properties:
// it re-reads what was actually written rather than trusting the writer, and it
// checks FILE NAMES as well as file bodies, because a fixture named after a
// config label is exactly the shape that carries an address into a path.
//
// # Two checks, and why one of them is not enough
//
// SENTINELS (Check returns class "sentinel"). Every credential the golden
// layers feed to the product is a value that occurs nowhere else in the
// repository. A hit is unambiguous: no false positive is possible, and no
// allowlist can suppress one. This is the strong check.
//
// SHAPES (every other class). Regular expressions for the things a credential
// LOOKS like: a UUID, a 43-character base64url key, a wpa_passphrase line, a
// share link, a PEM private key, an argon2 hash, a routable IPv4 address. This
// is the weak check, because it cannot tell an invented fixture from a real
// credential, and it therefore needs an allowlist.
//
// The sentinel check alone would miss a credential nobody thought to register.
// The shape check alone would drown in the invented fixtures that legitimately
// fill internal/xcfg/testdata. Together they cover both directions, and the
// allowlist is what makes the second one usable. Every allowlist entry names
// the file, the class and the REASON, and TestAllowlistEntriesAreAllUsed fails
// on an entry that no longer matches anything, so the list cannot rot into a
// blanket permission.
//
// # Open findings
//
// An allowlist entry says "this is not a secret". An open finding says "this IS
// or may be a secret, somebody who is not this package owns the file, and here
// is the record". Open findings do not fail the scan, and they are printed on
// every run with their owner and their date, so they cannot become quiet.
// TestOpenFindingsAreDatedAndOwned refuses an entry that does not say who owns
// it and when it was recorded.
package goldenscan

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Finding is one hit: a file, a line, and what matched.
//
// It deliberately does NOT carry the matched text. Printing the match would put
// the credential in the test output, in CI logs and in a terminal scrollback,
// which is the same disclosure the scan exists to prevent. hw_guard in
// test/hardware/lib/common.sh makes the same choice for the same reason.
type Finding struct {
	Path  string
	Line  int // 0 for a filename hit
	Class string
	// Name is set for a sentinel hit: which registered secret matched.
	Name string
}

func (f Finding) String() string {
	where := f.Path
	if f.Line > 0 {
		where = fmt.Sprintf("%s:%d", f.Path, f.Line)
	}
	if f.Name != "" {
		return fmt.Sprintf("%s  class=%s  secret=%s", where, f.Class, f.Name)
	}
	return fmt.Sprintf("%s  class=%s", where, f.Class)
}

// ---------------------------------------------------------------------------
// The shape classes
// ---------------------------------------------------------------------------

// shapeClass is one credential shape.
type shapeClass struct {
	name string
	re   *regexp.Regexp
	// why explains what a hit means, printed in the failure.
	why string
}

var shapeClasses = []shapeClass{
	{
		name: "uuid",
		re:   regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`),
		why:  "a UUID is the user id of a vless or vmess account",
	},
	{
		name: "base64-32byte-key",
		// 43 characters of base64url is exactly 32 bytes, which is the length
		// of a REALITY public key and of most symmetric keys this product
		// touches. The boundary assertions stop it matching a slice of a
		// longer blob.
		re:  regexp.MustCompile(`(^|[^A-Za-z0-9_-])[A-Za-z0-9_-]{43}([^A-Za-z0-9_-]|$)`),
		why: "43 characters of base64url decode to 32 bytes, the length of a REALITY public key",
	},
	{
		name: "wpa-passphrase",
		re:   regexp.MustCompile(`(?m)^wpa_passphrase=.+$`),
		why:  "a hostapd wpa_passphrase line is the WiFi key in the clear",
	},
	{
		name: "share-link",
		re:   regexp.MustCompile(`\b(vless|vmess|trojan|hysteria2|hy2|ss|socks)://\S`),
		why:  "a share link carries the user id, the keys and the server address together",
	},
	{
		name: "pem-private-key",
		re:   regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
		why:  "a PEM private key block",
	},
	{
		name: "argon2-hash",
		re:   regexp.MustCompile(`\$argon2(id|i|d)\$`),
		why:  "an argon2 hash of the panel password is an offline cracking target",
	},
	{
		name: "routable-ipv4",
		re:   regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`),
		why: "a routable IPv4 address may be a proxy server address, which this repository's " +
			"own .gitignore names as a live credential alongside user ids and keys",
	},
}

// nonRoutable reports whether an IPv4 literal is one that cannot identify
// anybody's server: private, loopback, link-local, multicast, unspecified,
// broadcast, or one of the ranges reserved for documentation and benchmarking.
//
// The routable-ipv4 class would otherwise fire on every DHCP range and every
// gateway in the hotspot and netcfg fixtures, which is hundreds of lines of
// noise for no signal.
func nonRoutable(s string) bool {
	var a, b, c, d int
	if n, err := fmt.Sscanf(s, "%d.%d.%d.%d", &a, &b, &c, &d); n != 4 || err != nil {
		return true
	}
	for _, v := range []int{a, b, c, d} {
		if v < 0 || v > 255 {
			return true // not an address at all
		}
	}
	switch {
	case a == 0: // unspecified and "this network"
		return true
	case a == 10: // RFC 1918
		return true
	case a == 127: // loopback
		return true
	case a == 169 && b == 254: // link-local
		return true
	case a == 172 && b >= 16 && b <= 31: // RFC 1918
		return true
	case a == 192 && b == 168: // RFC 1918
		return true
	case a == 192 && b == 0 && c == 0: // IETF protocol assignments
		return true
	case a == 192 && b == 0 && c == 2: // TEST-NET-1
		return true
	case a == 198 && b == 51 && c == 100: // TEST-NET-2
		return true
	case a == 203 && b == 0 && c == 113: // TEST-NET-3
		return true
	case a == 198 && (b == 18 || b == 19): // benchmarking
		return true
	case a == 100 && b >= 64 && b <= 127: // carrier-grade NAT
		return true
	case a >= 224: // multicast, reserved, broadcast
		return true
	case a == 255:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Scanning
// ---------------------------------------------------------------------------

// skipExt are file types a line scan cannot read usefully.
var skipExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".zip": true, ".gz": true, ".pdf": true,
}

// Roots returns every directory this scan covers: every testdata directory in
// the module, minus vendored dependency trees.
//
// Vendored trees are excluded because they are not this project's fixtures and
// their contents are decided by npm, not by anybody here. Excluding them is a
// scope decision and is stated so it does not read as an oversight.
func Roots(moduleRoot string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == ".git" || name == "node_modules" || name == ".codegraph" || name == ".idea" {
			return filepath.SkipDir
		}
		// /local/ is gitignored and holds real configs by design. It is never
		// committed, so it is out of scope, and reading it would put live
		// credentials into this process for no benefit.
		if path == filepath.Join(moduleRoot, "local") {
			return filepath.SkipDir
		}
		if name == "testdata" {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// Scan walks the given directories and returns every finding, allowlisted
// entries removed.
//
// Order is stable: directories in the order given, files sorted by path, lines
// in order, classes in declaration order. A scan whose output moved between
// runs would be unusable in a gate.
func Scan(dirs []string, moduleRoot string) ([]Finding, error) {
	var found []Finding
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, rerr := filepath.Rel(moduleRoot, path)
			if rerr != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)

			// File and directory NAMES first. A fixture named after a config
			// label is how an address reaches a path, which is the case
			// hw_guard_name exists for on the hardware side.
			found = append(found, scanName(rel)...)
			if d.IsDir() {
				return nil
			}
			if skipExt[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			body, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			found = append(found, scanBody(rel, string(body))...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return filterAllowed(found), nil
}

func scanName(rel string) []Finding {
	base := filepath.Base(rel)
	var out []Finding
	for _, s := range Sentinels() {
		if strings.Contains(base, s.Value) {
			out = append(out, Finding{Path: rel, Class: "sentinel", Name: s.Name})
		}
	}
	for _, c := range shapeClasses {
		if c.name == "routable-ipv4" {
			for _, m := range c.re.FindAllString(base, -1) {
				if !nonRoutable(m) {
					out = append(out, Finding{Path: rel, Class: c.name})
					break
				}
			}
			continue
		}
		if c.re.MatchString(base) {
			out = append(out, Finding{Path: rel, Class: c.name})
		}
	}
	return out
}

func scanBody(rel, body string) []Finding {
	var out []Finding
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		n := i + 1
		for _, s := range Sentinels() {
			if strings.Contains(line, s.Value) {
				out = append(out, Finding{Path: rel, Line: n, Class: "sentinel", Name: s.Name})
			}
		}
		for _, c := range shapeClasses {
			if c.name == "routable-ipv4" {
				for _, m := range c.re.FindAllString(line, -1) {
					if !nonRoutable(m) {
						out = append(out, Finding{Path: rel, Line: n, Class: c.name})
						break
					}
				}
				continue
			}
			if c.name == "wpa-passphrase" {
				// A line whose value is a redaction placeholder is the
				// correct state, not a finding. Reporting it would train
				// people to allowlist the class, and the allowlist entry would
				// then also cover the day somebody stopped redacting.
				if v, ok := strings.CutPrefix(line, "wpa_passphrase="); ok &&
					v != "" && !strings.HasPrefix(v, "<redacted:") {
					out = append(out, Finding{Path: rel, Line: n, Class: c.name})
				}
				continue
			}
			if c.re.MatchString(line) {
				out = append(out, Finding{Path: rel, Line: n, Class: c.name})
			}
		}
	}
	return out
}

// ClassWhy returns the explanation for a class name, for a failure message that
// says what the hit means rather than only that there was one.
func ClassWhy(class string) string {
	if class == "sentinel" {
		return "a value this repository's golden layers feed to the product as a credential; " +
			"it can only be here because something wrote a credential into a fixture"
	}
	for _, c := range shapeClasses {
		if c.name == class {
			return c.why
		}
	}
	return "unknown class"
}

// ShapeClassNames returns every shape class, for tests that must cover them all.
func ShapeClassNames() []string {
	out := make([]string, 0, len(shapeClasses))
	for _, c := range shapeClasses {
		out = append(out, c.name)
	}
	return out
}
