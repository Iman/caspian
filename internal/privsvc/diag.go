// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/netcfg"
)

// diagCapacity is how many diagnostic lines this service keeps.
//
// Small, because this is a recent-events view for one screen and not an audit
// log, and because the interesting event is always the most recent failure.
const diagCapacity = 64

// diagRing holds this service's own diagnostic lines, for the advanced view.
//
// # Why this exists at all
//
// A real start failed on the target on 2026-08-30 and the single most useful
// fact in the whole event existed NOWHERE. The command was
// "iw phy phy0 interface add ap0 type __ap" and the error was
// "Input/output error (-5)". What reached the panel was "Something went wrong
// and Caspian could not work out what", and what reached the log was two lines
// saying "start failed" and "fault=unknown". The command and the error were
// recovered by reading the leftover journal entry by hand and running the
// command in a terminal. On a box whose user cannot read a log and cannot
// reach a terminal, that is the difference between a product and a puzzle.
//
// panel.EngineLog is the only typed channel from this service to the advanced
// view, and internal/panel renders it under a heading that reads "Connection
// software log" rather than "engine log" (internal/panel/i18n_messages.go,
// "advanced.log.heading"), so this service's own lines about setting the
// connection up belong there. Service.EngineLog merges the two.
//
// # Redaction
//
// Text is redacted on the way IN and never on the way out, which is the same
// rule internal/engine's own ring keeps and for the same reason: there is then
// no path by which an unredacted line exists in memory to be read back out.
// Two things are removed. internal/engine.Redact takes out the key material
// the engine prints, and addResolved takes out the user's server addresses,
// which internal/netcfg puts in the argument vector of the pinned host route
// and which docs/LAYOUT.md says is never printed or logged.
type diagRing struct {
	mu      sync.Mutex
	buf     []engine.LogEntry
	next    int
	filled  bool
	dropped uint64
	now     func() time.Time
}

func newDiagRing(capacity int, now func() time.Time) *diagRing {
	if capacity <= 0 {
		capacity = diagCapacity
	}
	if now == nil {
		now = time.Now
	}
	return &diagRing{buf: make([]engine.LogEntry, capacity), now: now}
}

// add appends one line, redacting it first.
func (r *diagRing) add(text string) {
	if r == nil {
		return
	}
	entry := engine.LogEntry{At: r.now(), Text: engine.Redact(text)}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.filled {
		r.dropped++
	}
	r.buf[r.next] = entry
	r.next = (r.next + 1) % len(r.buf)
	if r.next == 0 {
		r.filled = true
	}
}

func (r *diagRing) addf(format string, args ...any) { r.add(fmt.Sprintf(format, args...)) }

// entries returns a copy of the retained lines, oldest first.
func (r *diagRing) entries() []engine.LogEntry {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	n := r.next
	if r.filled {
		n = len(r.buf)
	}
	out := make([]engine.LogEntry, 0, n)
	if r.filled {
		out = append(out, r.buf[r.next:]...)
	}
	return append(out, r.buf[:r.next]...)
}

func (r *diagRing) droppedCount() uint64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}

// redactedCommand renders a command for a person, with the user's server
// addresses taken out.
//
// The address is a credential on this path. internal/netcfg puts it in the
// argument vector of the pinned host route, docs/LAYOUT.md says the config is
// never printed or logged, and docs/INSTALL.md makes the same point about the
// uninstaller's journal replay: "One inverse removes the pinned host route to
// the user's proxy server, so its argument vector contains that server's
// address".
//
// Everything else in the vector is a fact about this machine rather than about
// the user: an interface name, a local gateway, a subnet this program chose. So
// the command is printed in full with one substitution, rather than reduced to
// an operation name, because "iw phy phy0 interface add ap0 type __ap" is the
// fact that made the failure diagnosable and "iface" is not.
//
// Command.String never prints stdin, so the generated nftables ruleset is
// reported as a byte count and not as text.
func redactedCommand(c netcfg.Command, servers []netip.Addr) string {
	return redactedText(c.String(), servers)
}

// redactedText removes the user's server addresses and then whatever
// internal/engine.Redact removes.
//
// It is applied to the ERROR as well as to the command, and that is not
// belt-and-braces: internal/netcfg builds its failure as
// fmt.Errorf("netcfg: step %q failed (%s): %w", s.Op, s.Do, runErr), so the
// error string embeds the whole command a second time. Redacting only the
// command would have printed the address anyway, one field to the right.
// TestTheServerAddressNeverAppearsInADiagnosticLine caught exactly that.
func redactedText(text string, servers []netip.Addr) string {
	for _, a := range servers {
		if !a.IsValid() {
			continue
		}
		text = strings.ReplaceAll(text, a.String(), "[server]")
	}
	return engine.Redact(text)
}
