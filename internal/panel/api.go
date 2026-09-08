// SPDX-License-Identifier: AGPL-3.0-or-later

package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// apiResponse captures typed page data before template rendering. The existing
// form handlers remain the sole implementation of authentication, validation
// and mutations; their redirects become updated UI state for native clients.
type apiResponse struct {
	http.ResponseWriter
	status int
	view   string
	page   *pageData
	body   bytes.Buffer
}

func (w *apiResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *apiResponse) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}

func (w *apiResponse) capture(status int, name string, data any) {
	w.status = status
	w.view = name
	if name == "index" {
		w.view = "dashboard"
	}
	page, ok := data.(pageData)
	if !ok {
		w.status = http.StatusInternalServerError
		return
	}
	w.page = &page
}

// handleAPIState exposes only the sign-in/setup page until a current session
// exists. The authenticated branch uses the same dashboard builder as HTML.
func (p *Panel) handleAPIState(w http.ResponseWriter, r *http.Request) {
	if !p.store.Snapshot().Panel.IsSet() {
		p.handleSetupForm(w, r)
		return
	}
	sess, ok := p.currentSession(r)
	if !ok {
		p.handleLoginForm(w, r)
		return
	}
	r = r.Clone(context.WithValue(r.Context(), sessionCtxKey, sess))
	r.URL.Path = "/"
	p.handleIndex(w, r)
}

func (p *Panel) finishAPI(w *apiResponse, r *http.Request) {
	// Only form actions issue a 303 after processing a mutation. In particular,
	// ServeMux's path cleanup runs before authentication or the handler and
	// must retain its redirect status rather than claim an action succeeded.
	if w.status == http.StatusSeeOther && r.Method == http.MethodPost {
		// A new session or a cleared session is already in Set-Cookie, but the
		// request still contains the old cookie. Apply that response locally to
		// build the state the client will see on its next request.
		next := r.Clone(r.Context())
		cookies := map[string]*http.Cookie{}
		for _, cookie := range next.Cookies() {
			cookies[cookie.Name] = cookie
		}
		for _, cookie := range (&http.Response{Header: w.Header()}).Cookies() {
			if cookie.MaxAge < 0 {
				delete(cookies, cookie.Name)
			} else {
				cookies[cookie.Name] = cookie
			}
		}
		next.Header.Del("Cookie")
		for _, cookie := range cookies {
			next.AddCookie(cookie)
		}
		next.Method = http.MethodGet
		w.Header().Del("Location")
		w.status, w.page = 0, nil
		w.body.Reset()
		p.handleAPIState(w, next)
		// A rejected mutation is a failure even though the HTML handler uses
		// a redirect to show its flash. A status problem discovered by polling
		// alone does not turn a successful mutation into a failure.
		if r.Method == http.MethodPost && r.URL.Path != "/api/v1/login" && r.URL.Path != "/api/v1/setup" {
			if sess, ok := p.currentSession(r); ok {
				if problem, _ := sess.flash(); !problem.Empty() {
					w.status = http.StatusUnprocessableEntity
				}
			}
		}
	}
	if w.status == 0 {
		w.status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Del("Content-Length")
	if w.page == nil {
		w.ResponseWriter.WriteHeader(w.status)
		if json.Valid(w.body.Bytes()) {
			_, _ = w.ResponseWriter.Write(w.body.Bytes())
		} else {
			_ = json.NewEncoder(w.ResponseWriter).Encode(map[string]string{"error": http.StatusText(w.status)})
		}
		return
	}
	type language struct {
		Code Lang   `json:"code"`
		Name string `json:"name"`
		Dir  string `json:"dir"`
	}
	languages := make([]language, 0, len(Langs))
	for _, lang := range Langs {
		languages = append(languages, language{lang, lang.NativeName(), lang.Dir()})
	}
	strings := make(map[string]string, len(messages[w.page.Lang]))
	for key, value := range messages[w.page.Lang] {
		strings[string(key)] = value
	}
	w.ResponseWriter.WriteHeader(w.status)
	if err := json.NewEncoder(w.ResponseWriter).Encode(struct {
		View      string            `json:"view"`
		OK        bool              `json:"ok"`
		Page      *pageData         `json:"page"`
		Strings   map[string]string `json:"strings"`
		Languages []language        `json:"languages"`
	}{w.view, w.status < 400, w.page, strings, languages}); err != nil {
		p.log.Warn("writing API state failed", "error", err.Error())
	}
}
