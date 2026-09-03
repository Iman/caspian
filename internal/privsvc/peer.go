// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"errors"
	"fmt"
	"os/user"
	"strconv"
)

// Peer is who is on the other end of a connection.
type Peer struct {
	UID uint32
	// PID is 0 where the platform does not report one. It is used for a log
	// line and never for a decision: a pid can be reused between the moment it
	// is read and the moment anything is done with it.
	PID int

	// SID is the account on Windows, where a UID means nothing; Admin is set
	// when that account is LocalSystem or a member of Administrators. Both
	// are empty on unix. See transport_windows.go.
	SID   string
	Admin bool
}

func (p Peer) String() string {
	if p.SID != "" {
		return "sid " + p.SID
	}
	return "uid " + strconv.FormatUint(uint64(p.UID), 10)
}

// ErrPeerNotAllowed is returned for a connection from an account this service
// does not answer.
var ErrPeerNotAllowed = errors.New("privsvc: the account on the other end of the socket is not allowed to talk to this service")

// ErrPeerUnknown is returned when the platform could not report who connected.
//
// It is a refusal and not a shrug. A boundary that answers anybody it cannot
// identify is not a boundary.
var ErrPeerUnknown = errors.New("privsvc: this platform cannot report who is on the other end of the socket")

// Allowed is the set of accounts permitted to drive this service.
//
// docs/LAYOUT.md fixes the socket at 0660 root:caspian inside a 0750
// root:caspian directory, so the file modes already keep everybody else out.
// This check is the second lock and it is stricter than the first in a way that
// matters: the mode admits anyone in the caspian GROUP, and this admits only
// the caspian ACCOUNT and root. Adding a user to a group is a thing an
// administrator does for unrelated reasons; it must not hand them the
// appliance's root helper.
type Allowed struct {
	// UIDs is the set of user ids that may connect.
	UIDs map[uint32]bool

	// SIDs are the Windows accounts permitted besides administrators, filled
	// by AllowedFor from the service account's name where the platform
	// resolves names to SIDs.
	SIDs map[string]bool
}

// AllowedFor builds the permitted set: root, plus the named service account if
// this machine has one.
//
// A machine with no such account still gets root, and the caller is told, so
// that "the panel cannot talk to the service" is reported as "there is no
// caspian account on this machine" rather than as a silent refusal of every
// connection.
func AllowedFor(serviceAccount string) (Allowed, error) {
	a := Allowed{UIDs: map[uint32]bool{0: true}, SIDs: map[string]bool{}}
	if serviceAccount == "" {
		return a, nil
	}
	if sid, ok := lookupAccountSID(serviceAccount); ok {
		a.SIDs[sid] = true
		return a, nil
	}
	u, err := user.Lookup(serviceAccount)
	if err != nil {
		return a, fmt.Errorf("privsvc: this machine has no %q account, so only root can reach the privileged service: %w", serviceAccount, err)
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return a, fmt.Errorf("privsvc: the %q account has a user id this program cannot read: %w", serviceAccount, err)
	}
	a.UIDs[uint32(uid)] = true
	return a, nil
}

// Permits reports whether this peer may drive the service.
func (a Allowed) Permits(p Peer) bool {
	if p.SID != "" {
		return p.Admin || a.SIDs[p.SID]
	}
	return a.UIDs[p.UID]
}

// The credential itself is read by peerCredential, which is platform specific:
// SO_PEERCRED on Linux, LOCAL_PEERCRED on Darwin, and a refusal everywhere
// else. There is deliberately no way to substitute it, not even for a test.
// An authorisation check with a seam in it is an authorisation check with a way
// past it, so the tests exercise the real syscall instead, by choosing which
// account the listener is told to permit.
