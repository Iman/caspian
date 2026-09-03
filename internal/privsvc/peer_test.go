// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import "testing"

// The Windows identity is a SID, and administrators are admitted by membership
// rather than by name. The logic is untagged so it is checked here on every
// platform; only the reading of the SID is Windows code.
func TestPermitsKnowsBothIdentities(t *testing.T) {
	a := Allowed{UIDs: map[uint32]bool{0: true, 501: true}, SIDs: map[string]bool{"S-1-5-80-1": true}}
	for _, tc := range []struct {
		peer Peer
		want bool
	}{
		{Peer{UID: 0}, true},
		{Peer{UID: 501}, true},
		{Peer{UID: 502}, false},
		{Peer{SID: "S-1-5-80-1"}, true},
		{Peer{SID: "S-1-5-21-9"}, false},
		{Peer{SID: "S-1-5-21-9", Admin: true}, true},
	} {
		if got := a.Permits(tc.peer); got != tc.want {
			t.Errorf("Permits(%v) = %t, want %t", tc.peer, got, tc.want)
		}
	}
	if (Peer{SID: "S-1-5-18"}).String() != "sid S-1-5-18" || (Peer{UID: 7}).String() != "uid 7" {
		t.Fatal("String must name the identity the platform has")
	}
}
