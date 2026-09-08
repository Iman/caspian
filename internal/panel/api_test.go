package panel

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func decodeAPI(t *testing.T, res *http.Response, body string) (string, pageData) {
	t.Helper()
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("API returned %d %s, expected JSON", res.StatusCode, res.Header.Get("Content-Type"))
	}
	var value struct {
		View    string            `json:"view"`
		Page    pageData          `json:"page"`
		Strings map[string]string `json:"strings"`
	}
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatal(err)
	}
	if len(value.Strings) == 0 {
		t.Fatal("missing localized strings")
	}
	return value.View, value.Page
}

func TestAPICanonicalizationDoesNotReportSuccessfulMutation(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	for _, signedIn := range []bool{true, false} {
		if !signedIn {
			h.signedOut()
		}
		for _, path := range []string{"/api/v1//power", "/api/v1/./power"} {
			res, body := h.postForm(path, url.Values{"on": {"1"}})
			// Go versions use either 301 or 307 for ServeMux path cleanup.
			redirect := res.StatusCode == http.StatusMovedPermanently || res.StatusCode == http.StatusTemporaryRedirect
			if !redirect || res.Header.Get("Location") != "/api/v1/power" {
				t.Errorf("canonicalization reported mutation result: signedIn=%t path=%s status=%d location=%q", signedIn, path, res.StatusCode, res.Header.Get("Location"))
			}
			var result struct {
				OK    bool   `json:"ok"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal([]byte(body), &result); err != nil || result.OK || result.Error == "" {
				t.Errorf("canonicalization returned a success envelope: signedIn=%t path=%s", signedIn, path)
			}
		}
	}
	if len(h.priv.Starts()) != 0 {
		t.Fatal("canonicalization started the engine")
	}
}

func TestAPISetupDashboardAndLogout(t *testing.T) {
	h := newHarness(t)
	res, body := h.get("/api/v1/state?lang=en")
	view, page := decodeAPI(t, res, body)
	if res.StatusCode != 200 || view != "setup" || page.CSRF == "" || page.SignedIn || page.Passphrase != "" {
		t.Fatalf("invalid setup state: %s", view)
	}
	res, body = h.postForm("/api/v1/setup", url.Values{"csrf": {page.CSRF}, "password": {"api-password"}, "confirm": {"api-password"}})
	view, page = decodeAPI(t, res, body)
	if res.StatusCode != 200 || view != "dashboard" || !page.SignedIn || page.CSRF == "" {
		t.Fatalf("setup did not authenticate: %d %s", res.StatusCode, view)
	}
	res, body = h.postForm("/api/v1/hotspot", url.Values{"csrf": {page.CSRF}, "ssid": {"test-ap"}, "passphrase": {"test-wifi-password"}})
	_, page = decodeAPI(t, res, body)
	if res.StatusCode != 200 || string(page.SSID) != "test-ap" || page.QR == "" {
		t.Fatalf("hotspot not reflected: %d", res.StatusCode)
	}
	res, body = h.postForm("/api/v1/logout", url.Values{"csrf": {page.CSRF}})
	view, page = decodeAPI(t, res, body)
	if res.StatusCode != 200 || view != "login" || page.SignedIn || page.Passphrase != "" || page.QR != "" {
		t.Fatal("logout leaked dashboard")
	}
}

func TestAPIDeviceCountSupportsSetupChecklist(t *testing.T) {
	h := newHarness(t)
	h.ready()
	for _, lang := range []string{"en", "fa"} {
		for _, count := range []int{0, 3} {
			h.priv.SetHotspot(HotspotStatus{Devices: count})
			res, body := h.get("/api/v1/state?lang=" + lang)
			decodeAPI(t, res, body)
			var reply struct {
				Page map[string]any `json:"page"`
			}
			if err := json.Unmarshal([]byte(body), &reply); err != nil {
				t.Fatal(err)
			}
			if reply.Page["DeviceCount"] != float64(count) {
				t.Fatalf("%s API device count = %v, want %d", lang, reply.Page["DeviceCount"], count)
			}
		}
	}
	h.signedOut()
	res, body := h.get("/api/v1/state")
	_, page := decodeAPI(t, res, body)
	if page.SignedIn || strings.Contains(body, `"DeviceCount":3`) {
		t.Fatal("signed-out API exposed device count")
	}
}

func TestAPIRejectsUnauthenticatedAndInvalidMutations(t *testing.T) {
	h := newHarness(t)
	h.setup("api-password")
	res, body := h.postForm("/api/v1/config", url.Values{"csrf": {h.tokenOn("/")}, "config": {"not-a-config"}})
	_, page := decodeAPI(t, res, body)
	if res.StatusCode < 400 || !page.HasProblem {
		t.Fatalf("invalid config reported success: %d", res.StatusCode)
	}
	res, body = h.postForm("/api/v1/power", url.Values{"csrf": {"wrong"}, "on": {"1"}})
	decodeAPI(t, res, body)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("missing CSRF rejection: %d", res.StatusCode)
	}
	h.signedOut()
	res, body = h.postForm("/api/v1/power", url.Values{"on": {"1"}})
	if res.StatusCode != http.StatusUnauthorized || !strings.Contains(res.Header.Get("Content-Type"), "application/json") || !json.Valid([]byte(body)) {
		t.Fatalf("invalid unauthorized response: %d", res.StatusCode)
	}
}

func TestAPILoginRateLimitAndPasswordInvalidation(t *testing.T) {
	h := newHarness(t)
	h.setup(testPassword)
	h.signedOut()
	for i := 0; i <= attemptBurst; i++ {
		res, body := h.get("/api/v1/state")
		_, page := decodeAPI(t, res, body)
		res, body = h.postForm("/api/v1/login", url.Values{"csrf": {page.CSRF}, "password": {"incorrect"}})
		view, failure := decodeAPI(t, res, body)
		if view != "login" || !failure.HasProblem || (res.StatusCode != 401 && res.StatusCode != 429) {
			t.Fatalf("bad login failure: %d %s", res.StatusCode, view)
		}
		if i == attemptBurst && (res.StatusCode != 429 || res.Header.Get("Retry-After") == "") {
			t.Fatal("API login bypasses rate limit")
		}
	}
	h.clock.Advance(attemptRefill * time.Duration(attemptBurst+1))
	res, body := h.get("/api/v1/state")
	_, page := decodeAPI(t, res, body)
	res, body = h.postForm("/api/v1/login", url.Values{"csrf": {page.CSRF}, "password": {testPassword}})
	view, page := decodeAPI(t, res, body)
	if res.StatusCode != 200 || view != "dashboard" {
		t.Fatal("valid login did not recover")
	}
	res, body = h.postForm("/api/v1/password", url.Values{"csrf": {page.CSRF}, "current": {testPassword}, "password": {"replacement-password"}, "confirm": {"replacement-password"}})
	view, page = decodeAPI(t, res, body)
	if res.StatusCode != 200 || view != "login" || page.SignedIn {
		t.Fatal("password change did not sign out")
	}
	if valid, _ := h.store.VerifyPanelPassword(testPassword); valid {
		t.Fatal("old password still valid")
	}
}

func TestAPIOriginAndCredentialBoundaries(t *testing.T) {
	h := newHarness(t)
	h.ready()
	res, body := h.get("/api/v1/state?lang=fa&advanced=1")
	_, page := decodeAPI(t, res, body)
	if page.Lang != LangFA || page.Dir != "rtl" || !page.Advanced || page.Passphrase == "" {
		t.Fatal("authenticated localized dashboard incomplete")
	}
	if strings.Contains(body, testLink()) {
		t.Fatal("API exposed raw proxy configuration")
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("credential response can be cached")
	}
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+"/api/v1/power", strings.NewReader(url.Values{"csrf": {page.CSRF}, "on": {"1"}}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://example.invalid")
	res, body = h.do(req)
	decodeAPI(t, res, body)
	if res.StatusCode != 403 || len(h.priv.Starts()) != 0 {
		t.Fatal("cross-origin API mutation reached engine")
	}
	h.signedOut()
	res, body = h.get("/api/v1/state?advanced=1")
	view, page := decodeAPI(t, res, body)
	if view != "login" || page.Passphrase != "" || page.QR != "" || page.ConfigSummary != "" || len(page.ConfigFacts) != 0 || len(page.EngineLog) != 0 {
		t.Fatal("signed-out state contains private dashboard fields")
	}
}

func TestAPIUsesExistingEngineActions(t *testing.T) {
	h := newHarness(t)
	h.ready()
	res, body := h.get("/api/v1/state")
	_, page := decodeAPI(t, res, body)
	for _, action := range []struct {
		path string
		form url.Values
	}{
		{"power", url.Values{"on": {"1"}}},
		{"cut", url.Values{"cut": {"1"}}},
		{"cut", url.Values{"cut": {"0"}}},
		{"recover", url.Values{}},
		{"power", url.Values{"on": {"0"}}},
	} {
		action.form.Set("csrf", page.CSRF)
		res, body = h.postForm("/api/v1/"+action.path, action.form)
		_, page = decodeAPI(t, res, body)
		if res.StatusCode != 200 {
			t.Fatalf("%s failed: %d %s", action.path, res.StatusCode, page.ProblemHeadline)
		}
	}
	if len(h.priv.Starts()) == 0 || h.priv.Stops() == 0 {
		t.Fatal("API did not reach privileged actions")
	}
	h.priv.SetRecoverError(errors.New("backend unavailable"))
	res, body = h.postForm("/api/v1/recover", url.Values{"csrf": {page.CSRF}})
	_, page = decodeAPI(t, res, body)
	if res.StatusCode != 422 || !page.HasProblem {
		t.Fatal("failed recovery reported success")
	}
}
