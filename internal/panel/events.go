// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"sync"
	"time"
)

// The events area is the only place a person can see what the box has been
// doing, and it is built out of sentences rather than log lines.
//
// # Why it does not render the engine log
//
// The obvious source is the engine's own log, which internal/engine already
// redacts. It is the wrong source for this area anyway, for two reasons that
// are not about secrets. It is English, produced by a Go library, and there is
// no honest way to translate a line like "failed to dial tcp" into Persian at
// render time; a machine-translated log is worse than an untranslated one
// because it reads as though somebody wrote it. And it is engine vocabulary,
// which is the "no AP-capable phy" mistake in another costume.
//
// So the events area records what the PANEL knows: what the user did, and what
// the panel observed while polling. Every entry is a closed vocabulary value
// rendered through the message catalogue, which makes it a sentence in both
// languages by construction and makes it impossible for one to carry a
// credential: there is nowhere in an Event to put a string.
//
// The engine's own log is still available, unchanged and already redacted, in
// advanced mode. That is the split: plain sentences here, the engine's words
// underneath for whoever wants them.
//
// # What it does not do
//
// It is memory only and it is per process. A restart empties it, and the panel
// says so on screen rather than letting somebody believe they are looking at a
// full history. Persisting it was rejected: the state file holds credentials
// and is written through a privileged path, and a UI activity log is not worth
// putting on that path. It is also not telemetry and never leaves the box
// (design section 6).

// EventKind is one thing that can appear in the events area. The set is closed;
// each value has exactly one message key.
type EventKind string

const (
	EventSignedIn        EventKind = "signed-in"
	EventSwitchedOn      EventKind = "switched-on"
	EventSwitchedOff     EventKind = "switched-off"
	EventConnected       EventKind = "connected"
	EventDisconnected    EventKind = "disconnected"
	EventStartFailed     EventKind = "start-failed"
	EventConfigAdded     EventKind = "config-added"
	EventConfigChanged   EventKind = "config-changed"
	EventHotspotNamed    EventKind = "hotspot-named"
	EventAdvancedSaved   EventKind = "advanced-saved"
	EventTrafficCut      EventKind = "traffic-cut"
	EventTrafficRestored EventKind = "traffic-restored"
	EventRecovered       EventKind = "recovered"
	EventWrongPassword   EventKind = "wrong-password"
)

// eventKinds is every kind, so a test can check each one has a message in both
// languages rather than discovering a missing one on a user's screen.
var eventKinds = []EventKind{
	EventSignedIn, EventSwitchedOn, EventSwitchedOff, EventConnected,
	EventDisconnected, EventStartFailed, EventConfigAdded, EventConfigChanged,
	EventHotspotNamed, EventAdvancedSaved, EventTrafficCut, EventTrafficRestored, EventRecovered,
	EventWrongPassword,
}

// Key is the message for this kind.
func (k EventKind) Key() Key {
	switch k {
	case EventSignedIn:
		return MsgEventSignedIn
	case EventSwitchedOn:
		return MsgEventSwitchedOn
	case EventSwitchedOff:
		return MsgEventSwitchedOff
	case EventConnected:
		return MsgEventConnected
	case EventDisconnected:
		return MsgEventDisconnected
	case EventStartFailed:
		return MsgEventStartFailed
	case EventConfigAdded:
		return MsgEventConfigAdded
	case EventConfigChanged:
		return MsgEventConfigChanged
	case EventHotspotNamed:
		return MsgEventHotspotNamed
	case EventAdvancedSaved:
		return MsgEventAdvancedSaved
	case EventTrafficCut:
		return MsgEventTrafficCut
	case EventTrafficRestored:
		return MsgEventTrafficRestored
	case EventWrongPassword:
		return MsgEventWrongPassword
	default:
		return MsgFaultUnrecognised
	}
}

// Event is one entry. It holds no free-form text, which is what makes it
// impossible for one to carry a credential.
type Event struct {
	Kind EventKind
	At   time.Time

	// Fault is set only for EventStartFailed, and adds the reason to the
	// sentence. It is a closed vocabulary value, not a message.
	Fault Fault
}

// Sentence renders the event in a language.
func (e Event) Sentence(l Lang) string {
	s := T(l, e.Kind.Key())
	if e.Fault != FaultNone {
		s += " " + T(l, e.Fault.Key())
	}
	return s
}

// eventCapacity is how many entries are kept. Enough to cover a session of
// somebody fixing something, small enough that the area stays readable and the
// memory is irrelevant.
const eventCapacity = 40

// eventLog is a bounded ring of events, newest first on the way out.
type eventLog struct {
	mu   sync.Mutex
	ring []Event
	next int
	full bool
	now  func() time.Time

	// observed tracks the last connection state seen by a status poll, so a
	// transition is recorded once rather than once per poll. seen guards the
	// first observation, which must not report a drop on a box that was simply
	// never on.
	seen      bool
	connected bool
}

func newEventLog(now func() time.Time) *eventLog {
	return &eventLog{ring: make([]Event, eventCapacity), now: now}
}

// add records an event.
func (l *eventLog) add(kind EventKind, fault Fault) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.addLocked(kind, fault)
}

func (l *eventLog) addLocked(kind EventKind, fault Fault) {
	l.ring[l.next] = Event{Kind: kind, At: l.now(), Fault: fault}
	l.next = (l.next + 1) % len(l.ring)
	if l.next == 0 {
		l.full = true
	}
}

// observe records a change in the connection state seen by a status poll.
//
// It is called from GET handlers, which means several browsers polling at once
// all reach it. The lock plus the "only on a change" test makes that safe and
// idempotent: ten concurrent pollers seeing the same transition record one
// event between them, not ten.
func (l *eventLog) observe(st SystemStatus, reachable bool) {
	if !reachable {
		// The privileged service is not answering, so nothing is known about
		// the tunnel. Recording a drop here would invent an event out of the
		// panel's own blindness.
		return
	}
	now := st.Connected()

	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.seen {
		l.seen, l.connected = true, now
		return
	}
	if now == l.connected {
		return
	}
	l.connected = now
	if now {
		l.addLocked(EventConnected, FaultNone)
	} else {
		l.addLocked(EventDisconnected, FaultNone)
	}
}

// entries returns the events, newest first.
func (l *eventLog) entries() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	n := l.next
	if l.full {
		n = len(l.ring)
	}
	out := make([]Event, 0, n)
	for i := 0; i < n; i++ {
		idx := (l.next - 1 - i + len(l.ring)*2) % len(l.ring)
		out = append(out, l.ring[idx])
	}
	return out
}
