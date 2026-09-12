// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package link

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/xtls/xray-core/infra/conf"
)

// TestPublicListsParseAndBuild runs every share link in a directory of public
// lists through Parse, XrayConfig and the engine's own conf.Build, and reports
// what was accepted, what was refused and why. It is the survey that on
// 2026-09-12 measured 8,600 links from four public repositories: the parser
// accepted 96 to 99 percent, and the engine's refusals were almost all one of
// two removed features, allowInsecure and the pre-AEAD Shadowsocks ciphers.
//
// It is opt-in. It needs files nobody commits, because a public list is a
// list of strangers' servers and this repository never carries a server
// address. Fetch them with scripts/fetch-public-lists.sh, then:
//
//	CASPIAN_PUBLIC_LISTS_DIR=local/public-lists go test ./internal/link/ -run TestPublicLists -v -count=1
//
// Nothing is connected to. A link that parses and builds is a link this box
// would accept, and says nothing about whether the server answers.
//
// What it asserts is structural: every share line either parses or is refused
// with a sentence that names no value, and no document that parsed makes the
// engine panic. The counts are printed, not asserted, because the lists churn
// daily and a floor on them would fail for reasons that are not this code's.
func TestPublicListsParseAndBuild(t *testing.T) {
	dir := os.Getenv("CASPIAN_PUBLIC_LISTS_DIR")
	if dir == "" {
		t.Skip("set CASPIAN_PUBLIC_LISTS_DIR to a directory filled by scripts/fetch-public-lists.sh; " +
			"without it this survey of public share links is not run and proves nothing")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(files) == 0 {
		t.Fatalf("%s holds no files; run scripts/fetch-public-lists.sh first", dir)
	}
	scheme := regexp.MustCompile(`^[a-z0-9]+://`)
	ipv4 := regexp.MustCompile(`\b\d{1,3}(\.\d{1,3}){3}\b`)
	redact := func(s string) string { return ipv4.ReplaceAllString(s, "<addr>") }

	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		name := filepath.Base(f)
		var report strings.Builder
		fmt.Fprintf(&report, "\n== %s (%d bytes)", name, len(raw))

		// The whole-document path, which is what a paste of the file is.
		list, err := ParseAll(text)
		if err != nil {
			fmt.Fprintf(&report, "\n   ParseAll refused the document: %s", redact(err.Error()))
		} else {
			fmt.Fprintf(&report, "\n   ParseAll: entries=%d dropped=%d", len(list.Entries), list.Dropped)
		}

		// The per-link path, one Parse per share line, for lists of links; the
		// per-entry path through Select for documents without share lines.
		var links []*Link
		refused := map[string]int{}
		var lines []string
		for _, ln := range strings.Split(text, "\n") {
			ln = strings.TrimSpace(ln)
			if scheme.MatchString(ln) {
				lines = append(lines, ln)
			}
		}
		switch {
		case len(lines) > 0:
			for _, ln := range lines {
				l, err := Parse(ln)
				if err != nil {
					if strings.Contains(err.Error(), "://") {
						t.Errorf("%s: a parse error carries a link: %s", name, redact(err.Error()))
					}
					refused[redact(err.Error())]++
					continue
				}
				links = append(links, l)
			}
			fmt.Fprintf(&report, "\n   per link: share lines=%d parsed=%d refused=%d%s",
				len(lines), len(links), len(lines)-len(links), topCounts(refused, 8))
		case list != nil:
			for _, e := range list.Entries {
				l, _, err := Select(text, e.Index)
				if err != nil {
					refused[redact(err.Error())]++
					continue
				}
				links = append(links, l)
			}
			fmt.Fprintf(&report, "\n   per entry: parsed=%d refused=%d%s", len(links), len(refused), topCounts(refused, 8))
		}

		// The engine's verdict on each document, and the shape of what it took.
		built := 0
		shapes := map[string]int{}
		buildFail := map[string]int{}
		for _, l := range links {
			shape := fmt.Sprintf("%s/%s/%s", l.Protocol, l.Network, l.Security)
			b, err := l.XrayConfig()
			if err == nil {
				var c conf.Config
				if err = json.Unmarshal(b, &c); err == nil {
					_, err = c.Build()
				}
			}
			if err != nil {
				buildFail[shape+": "+lastSegment(redact(err.Error()))]++
				continue
			}
			built++
			shapes[shape]++
		}
		fmt.Fprintf(&report, "\n   built by the engine: %d of %d parsed%s\n   shapes that build:%s",
			built, len(links), topCounts(buildFail, 10), topCounts(shapes, 30))
		t.Log(report.String())
	}
}

// lastSegment keeps the part of an xray-core error after the last " > ", which
// is the cause; the prefix is the same wrapping on every line.
func lastSegment(s string) string {
	if i := strings.LastIndex(s, " > "); i >= 0 {
		return s[i+3:]
	}
	return s
}

// topCounts renders a count map, largest first, at most n rows.
func topCounts(m map[string]int, n int) string {
	type kv struct {
		k string
		v int
	}
	var kvs []kv
	for k, v := range m {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].v > kvs[j].v || (kvs[i].v == kvs[j].v && kvs[i].k < kvs[j].k)
	})
	var b strings.Builder
	for i, e := range kvs {
		if i >= n {
			fmt.Fprintf(&b, "\n      ... %d more", len(kvs)-n)
			break
		}
		fmt.Fprintf(&b, "\n      %5d  %s", e.v, e.k)
	}
	return b.String()
}
