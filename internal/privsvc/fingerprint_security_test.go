// SPDX-License-Identifier: AGPL-3.0-or-later
package privsvc

import (
	"context"
	"testing"
)

func TestRequestFingerprintIsScopedToService(t *testing.T) {
	a, b := newWorld(t), newWorld(t)
	req := startRequest(t)
	if err := a.svc.Start(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := b.svc.Start(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	first := a.svc.Applied()
	if first == "" || first == b.svc.Applied() {
		t.Fatal("the same secret-bearing request has a reusable fingerprint across services")
	}
	if err := a.svc.Start(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if a.svc.Applied() != first {
		t.Fatal("identical request changed fingerprint within one service")
	}
}
