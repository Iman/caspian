// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"caspianbyoc.org/caspian/internal/panel"
)

// ---------------------------------------------------------------------------
// The wire
//
// One request, one response, one connection. There is no session, no
// multiplexing and no second message: a connection carries a single named
// action and is then closed. That is what makes "a malformed message is refused
// without reaching any privileged code" a property of the shape rather than of
// somebody remembering to check.
//
// A message is a 4-byte big-endian length followed by that many bytes of JSON:
//
//	+--------+----------------+
//	| uint32 | JSON, n bytes  |
//	+--------+----------------+
//
// Length prefixed rather than newline delimited, for one reason: the length is
// checked against maxFrameBytes BEFORE anything is allocated or parsed, so an
// oversized message costs four bytes and a refusal rather than a buffer.
//
// # The vocabulary
//
// The only verbs on this wire are panel.Actions. There is no field anywhere in
// a request that names a command, a path, a binary or an argument list, and no
// field whose value is used to build one. internal/panel/priv.go states the
// rule the whole split exists for: "A privileged helper that takes a path and
// an argument list from its client is not a boundary; it is a way to run
// anything as root." TestWireVocabularyIsTheClosedActionSet holds it by walking
// the request type the same way internal/panel's own test walks the interface.
//
// # What comes back, and what deliberately does not
//
// On the FAILURE path, one word: a panel.Fault, which is a closed set declared
// in internal/panel and visible in one screen, or a Refusal, which is a closed
// set declared below. Nothing else. An error inside this service can carry the
// engine's own text, which embeds the user's private key, seed, short id and
// UUID (internal/engine/redact.go); a daemon's stderr; or a value the caller
// sent. None of that crosses back: it is logged on the privileged side and
// dropped. There is no field on wireResponse it could travel in.
//
// On the SUCCESS path, three result types do cross, and it is worth being
// precise rather than claiming nothing does. panel.Detection carries interface
// names, a channel and a subnet, none of which is a credential.
// panel.SystemStatus carries the hotspot's SSID, which is broadcast, and
// engine.State.Reason, which internal/engine has already put through Redact.
// panel.EngineLog carries engine log lines, which internal/engine redacts on
// the way into its ring and never on the way out, so there is no path by which
// an unredacted one exists to be sent. The hotspot passphrase is in none of
// them; it travels one way only, from the panel in a StartRequest.
// ---------------------------------------------------------------------------

// maxFrameBytes bounds one message.
//
// The largest thing on this wire is a start request carrying a configuration
// document, which configFromRequest independently bounds at maxConfigBytes.
// This is the outer bound, on the socket, before any of it is JSON.
const maxFrameBytes = 256 << 10

// protocolVersion is sent on every request and checked on every one.
//
// It exists because the two halves are one binary and are meant to be upgraded
// together, and the failure when they are not has to be loud. A panel from one
// release talking to a privileged service from another gets a refusal naming
// the mismatch, not a field silently decoded as its zero value.
const protocolVersion = 1

// Refusal is a protocol-level rejection. It is not a panel.Fault: a Fault
// describes the machine, and these describe the message.
//
// The set is closed and every value is a constant in this file. Nothing a
// caller sent is ever echoed in one.
type Refusal string

const (
	RefusalBadFrame      Refusal = "bad-frame"
	RefusalTooLarge      Refusal = "too-large"
	RefusalBadJSON       Refusal = "bad-json"
	RefusalBadVersion    Refusal = "bad-version"
	RefusalUnknownAction Refusal = "unknown-action"
	RefusalMissingArg    Refusal = "missing-argument"
	RefusalUnexpectedArg Refusal = "unexpected-argument"
)

// Refusals is every protocol-level rejection this service can send.
var Refusals = []Refusal{
	RefusalBadFrame, RefusalTooLarge, RefusalBadJSON, RefusalBadVersion,
	RefusalUnknownAction, RefusalMissingArg, RefusalUnexpectedArg,
}

// wireRequest is one call.
//
// Note what is not here: no command, no path, no argument vector, no interface
// name that is not also a typed field of panel.StartRequest, and no free-form
// string that reaches an exec. Action is compared against panel.Actions by
// equality, never used to look anything up by name.
type wireRequest struct {
	Version int          `json:"v"`
	Action  panel.Action `json:"action"`

	// Start is present for, and only for, panel.ActionStart.
	Start *panel.StartRequest `json:"start,omitempty"`

	// DeadlineUnixNano is when the caller stops waiting. The privileged side
	// derives its own context from it, clamped to [minDeadline, maxDeadline],
	// so a caller cannot ask for an unbounded operation and cannot cut one so
	// short that it is guaranteed to fail half way. Zero means "use the
	// default", which is what a caller with no deadline gets.
	//
	// A deadline rather than a timeout because internal/panel/priv.go says the
	// panel "always gives it a deadline", and a deadline survives the time
	// spent getting here.
	DeadlineUnixNano int64 `json:"deadline_unix_nano,omitempty"`
}

// MinDeadline and MaxDeadline are the bounds a caller's deadline is clamped to.
//
// The floor exists because a start reconfigures routes, starts an engine and
// starts two daemons, and being cut off in the middle of that is exactly the
// half-configured box this service exists not to leave. The ceiling exists so
// that a caller cannot pin a root process to one operation for ever.
//
// They are exported because a client that asks for a deadline outside them gets
// something other than what it asked for, silently: a caller that waits twenty
// minutes for an operation this service abandoned after ten has no way to tell
// that from a slow box. cmd/caspian checks its own timeout against them.
const (
	MinDeadline = 5 * time.Second
	MaxDeadline = 10 * time.Minute

	defaultDeadline = 60 * time.Second
)

// wireResponse is one answer.
//
// Exactly one of the three shapes is meaningful, decided by the action. Fault
// empty and Refusal empty means the call succeeded.
type wireResponse struct {
	Fault   panel.Fault `json:"fault,omitempty"`
	Refusal Refusal     `json:"refusal,omitempty"`

	Detect *panel.Detection    `json:"detect,omitempty"`
	Status *panel.SystemStatus `json:"status,omitempty"`
	Log    *panel.EngineLog    `json:"log,omitempty"`
}

// errFrameTooLarge is returned by readFrame for a length above maxFrameBytes.
var errFrameTooLarge = errors.New("privsvc: message is larger than this service accepts")

// writeFrame writes one length-prefixed message.
func writeFrame(w io.Writer, payload []byte) error {
	if len(payload) > maxFrameBytes {
		return errFrameTooLarge
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// readFrame reads one length-prefixed message.
//
// The length is checked before the body is allocated, which is the whole reason
// the frame carries one.
func readFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > maxFrameBytes {
		return nil, errFrameTooLarge
	}
	if n == 0 {
		return nil, errors.New("privsvc: empty message")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// decodeRequest turns bytes into a request, or into the refusal to send back.
//
// EVERY check here happens before the service is touched. The order is: is it
// JSON at all, is it this protocol, is the verb a known one, and does the
// verb carry exactly the argument it should. A message that fails any of them
// reaches no privileged code.
func decodeRequest(payload []byte) (wireRequest, Refusal) {
	var req wireRequest
	dec := json.NewDecoder(bytes.NewReader(payload))
	// Unknown fields are refused rather than ignored. A field this build does
	// not know about means the two halves disagree about the vocabulary, and
	// the failure has to be loud: silently dropping it is how a request means
	// one thing to the sender and another to the root process.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return wireRequest{}, RefusalBadJSON
	}
	if dec.More() {
		// Trailing bytes after the object. One message per frame, exactly.
		return wireRequest{}, RefusalBadJSON
	}
	if req.Version != protocolVersion {
		return wireRequest{}, RefusalBadVersion
	}
	if !knownAction(req.Action) {
		return wireRequest{}, RefusalUnknownAction
	}
	// Recover ends in a start, so it carries the same argument start does.
	if req.Action == panel.ActionStart || req.Action == panel.ActionRecover {
		if req.Start == nil {
			return wireRequest{}, RefusalMissingArg
		}
	} else if req.Start != nil {
		// An argument on an action that takes none. It would be ignored, and
		// an ignored argument is a caller and a service that disagree about
		// what was asked for.
		return wireRequest{}, RefusalUnexpectedArg
	}
	return req, ""
}

// knownAction reports whether a is one of the actions panel.Actions names.
//
// It walks panel.Actions rather than a switch, so that an action added to
// internal/panel is carried here without an edit and internal/panel's own
// TestActionVocabularyMatchesTheInterface stays the single guard on that list.
func knownAction(a panel.Action) bool {
	for _, k := range panel.Actions {
		if a == k {
			return true
		}
	}
	return false
}

// deadlineFrom turns the caller's deadline into one this service will honour.
func deadlineFrom(unixNano int64, now time.Time) time.Time {
	if unixNano == 0 {
		return now.Add(defaultDeadline)
	}
	d := time.Unix(0, unixNano).Sub(now)
	switch {
	case d < MinDeadline:
		d = MinDeadline
	case d > MaxDeadline:
		d = MaxDeadline
	}
	return now.Add(d)
}

// refusalError is what a client turns a Refusal into.
//
// It is deliberately NOT a panel.FaultError. A refusal is not a statement about
// the machine, and panel.FaultOf will therefore report it as
// panel.FaultUnknown, which is the honest answer: the privileged side did not
// classify anything, it declined to read the message.
func refusalError(r Refusal) error {
	return fmt.Errorf("privsvc: the privileged service refused the request: %s", string(r))
}
