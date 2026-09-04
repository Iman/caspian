// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package privsvc

import "testing"

func TestWindowsPeerIsAuthorisedByThePipeDACL(t *testing.T) {
	if !(Allowed{}).Permits(Peer{SID: "pipe-dacl", Admin: true}) {
		t.Fatal("a peer admitted by the Windows pipe DACL was rejected")
	}
}
