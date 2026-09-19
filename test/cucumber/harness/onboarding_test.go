// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"caspianbyoc.org/caspian/internal/state"
)

func TestFirstRunControlClearsSetupAndPersistsWithoutChangingAdvanced(t *testing.T) {
	h := &harness{}
	if err := h.reset(""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, dir := range h.dirs {
			_ = os.RemoveAll(dir)
		}
	})
	if err := h.cur.store.Update(func(st *state.State) error {
		st.Advanced.InternetInterface = "test-uplink"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	before := h.cur.store.Snapshot()
	if !before.Panel.IsSet() || !before.Proxy.IsConfigured() || before.Hotspot.SSID == "" {
		t.Fatal("fixture did not start configured")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/__control/first-run", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("first-run status=%d, want200", w.Code)
	}
	reloaded, err := state.Load(h.cur.store.Dir())
	if err != nil {
		t.Fatal(err)
	}
	after := reloaded.Snapshot()
	if after.Panel.IsSet() || after.Proxy.IsConfigured() || after.Hotspot.SSID != "" || !after.Hotspot.Passphrase.IsZero() {
		t.Fatal("first-run fixture retained configured credentials")
	}
	if after.Advanced != before.Advanced {
		t.Fatal("first-run fixture changed advanced settings")
	}
}
