package panel

import (
	"net/http"
	"net/url"
	"testing"
)

func TestPasswordChangeRevokesSessionsAndPreservesConfiguration(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	if err := h.store.SetHotspot("KeepThisSSID", "keep-this-passphrase"); err != nil {
		t.Fatal(err)
	}
	before := h.store.Snapshot().Hotspot
	res, _ := h.get("/")
	oldCookies := h.client.Jar.Cookies(res.Request.URL)
	res, _ = h.postForm("/password", url.Values{"csrf": {h.tokenOn("/")}, "current": {testPassword}, "password": {"a-new-panel-password"}, "confirm": {"a-new-panel-password"}})
	if res.StatusCode != 303 || res.Header.Get("Location") != "/login" {
		t.Fatalf("change failed: %d", res.StatusCode)
	}
	if h.store.Snapshot().Hotspot != before {
		t.Fatal("hotspot settings changed")
	}
	h.client.Jar.SetCookies(res.Request.URL, oldCookies)
	res, _ = h.get("/status.json")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatal("old session survived password change")
	}
	h.signedOut()
	if h.signIn(testPassword).StatusCode != http.StatusUnauthorized {
		t.Fatal("old password still works")
	}
	if h.signIn("a-new-panel-password").StatusCode != 303 {
		t.Fatal("new password does not work")
	}
}

func TestPasswordChangeRejectsBadInputs(t *testing.T) {
	for _, tc := range []struct{ name, current, password, confirm string }{
		{"wrong current", "wrong", "long-new-password", "long-new-password"},
		{"short", testPassword, "short", "short"},
		{"mismatch", testPassword, "long-new-password", "different-password"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.setup(testPassword)
			h.postForm("/password", url.Values{"csrf": {h.tokenOn("/")}, "current": {tc.current}, "password": {tc.password}, "confirm": {tc.confirm}})
			if ok, err := h.store.VerifyPanelPassword(testPassword); err != nil || !ok {
				t.Fatal("invalid input changed password")
			}
		})
	}
}
