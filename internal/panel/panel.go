// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"caspianbyoc.org/caspian/internal/state"
)

// Config is everything a Panel needs. Every field with a sensible default has
// one; Store and Priv have none, because guessing either would be wrong.
type Config struct {
	// Store is the persistent state. Required.
	Store *state.Store

	// Priv is the privileged service. Required. Use NewFakePrivileged in a
	// test, or on a developer machine with no root and no radio.
	Priv Privileged

	// Logger receives the panel's own log lines. Defaults to a logger that
	// discards everything, so a caller that has not thought about where logs
	// go does not get them on stdout, which on an appliance is a hole rather
	// than an output.
	//
	// Nothing written through it ever contains the pasted config or the
	// hotspot passphrase; see the note on logging below.
	Logger *slog.Logger

	// ListenAddrs is where to serve. Each must pass ValidateBindAddr, so a
	// wildcard is refused at construction and not at bind time. Empty is
	// allowed, for a Panel used as an http.Handler; use BindAddrs to derive
	// the default, which is the hotspot interface only.
	ListenAddrs []string

	// TLS says the panel is served over HTTPS, which decides whether the
	// session cookie is marked Secure. See newSessionCookie for why this
	// cannot simply be true.
	TLS bool

	// PrivTimeout bounds every call to the privileged service, so a wedged
	// service cannot hold an HTTP handler open. Defaults to 20 seconds, which
	// is long enough for a real start on a Raspberry Pi.
	PrivTimeout time.Duration

	// Now is the clock, overridable for tests.
	Now func() time.Time
}

// Panel is the web interface. It is an http.Handler.
//
// # Logging
//
// The panel logs to Config.Logger and the rule is absolute: no log line ever
// carries the pasted config, the config document built from it, the hotspot
// passphrase, the panel password, or a session token. Design section 9 lists
// log redaction as a known hazard, and the state and link packages both hold
// the same rule from the other side. The tests hold it too:
// TestPastedConfigNeverAppearsInAResponseOrALog submits a config and then greps
// every response body and every log line for it.
//
// That name is on one line on purpose. A test name wrapped across two comment
// lines cannot be found by the grep that would tell a future reader whether the
// test still exists, which is how a citation rots while still reading as
// authoritative.
//
// The way that rule is kept is that the values never reach a formatting verb.
// state.Secret, link.Link, HotspotSpec and StartRequest all redact themselves
// through String and GoString, so even a %v on a whole struct is safe.
type Panel struct {
	store *state.Store
	priv  Privileged
	log   *slog.Logger

	mux      *http.ServeMux
	sessions *sessionStore
	limiter  *attemptLimiter
	events   *eventLog

	now         func() time.Time
	privTimeout time.Duration

	secureCookies bool
	cookieName    string
	listenAddrs   []string
}

// maxBodyBytes bounds a request body. A pasted config is a link or a small JSON
// document; 256 KiB is far more than any of them and small enough that a
// hostile client cannot spend the box's memory through this door.
const maxBodyBytes = 256 << 10

// routeSpec is one route. The table below is the single list of what the panel
// serves, and it exists so that "is every route authenticated" is a question
// with an answer rather than a review.
type routeSpec struct {
	Method string
	Path   string

	// Public routes are reachable without a session, because each one has to
	// work before anyone can log in: the two forms, and the three static files
	// those forms are rendered with.
	//
	// The exact number is asserted in TestThePublicRoutesAreTheExpectedSeven
	// rather than written here, because a count in a comment is wrong the first
	// time somebody adds a route and nothing notices.
	Public bool

	// JSON routes answer with a status code rather than a redirect when there
	// is no session, because a redirect to an HTML login page is not something
	// a fetch() can do anything with.
	JSON bool
}

// routes is every route the panel serves.
//
// TestEveryRouteRefusesWithoutASession walks this table, so a route added
// without a thought about authentication is a failing test rather than a hole.
// New also refuses to start if this table and the handler map disagree in
// either direction, so a route cannot be served without appearing here and an
// entry here cannot be left without a handler.
var routes = []routeSpec{
	{Method: "GET", Path: "/"},
	{Method: "POST", Path: "/power"},
	{Method: "POST", Path: "/cut"},
	{Method: "GET", Path: "/help"},
	{Method: "POST", Path: "/config"},
	{Method: "POST", Path: "/hotspot"},
	{Method: "POST", Path: "/advanced"},
	{Method: "POST", Path: "/recover"},
	{Method: "POST", Path: "/logout"},
	{Method: "GET", Path: "/status.json", JSON: true},

	// The public routes.
	{Method: "GET", Path: "/login", Public: true},
	{Method: "POST", Path: "/login", Public: true},
	{Method: "GET", Path: "/setup", Public: true},
	{Method: "POST", Path: "/setup", Public: true},
	{Method: "GET", Path: "/assets/panel.css", Public: true},
	{Method: "GET", Path: "/assets/panel.js", Public: true},
	{Method: "GET", Path: "/favicon.svg", Public: true},
}

// New builds a Panel.
func New(cfg Config) (*Panel, error) {
	if cfg.Store == nil {
		return nil, errors.New("panel: Config.Store is required")
	}
	if cfg.Priv == nil {
		return nil, errors.New("panel: Config.Priv is required")
	}
	for _, addr := range cfg.ListenAddrs {
		if err := ValidateBindAddr(addr); err != nil {
			return nil, err
		}
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	timeout := cfg.PrivTimeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}

	p := &Panel{
		store:         cfg.Store,
		priv:          cfg.Priv,
		log:           logger,
		mux:           http.NewServeMux(),
		sessions:      newSessionStore(now),
		limiter:       newAttemptLimiter(now),
		events:        newEventLog(now),
		now:           now,
		privTimeout:   timeout,
		secureCookies: cfg.TLS,
		cookieName:    cookieName(cfg.TLS),
		listenAddrs:   append([]string(nil), cfg.ListenAddrs...),
	}

	handlers := map[string]http.HandlerFunc{
		"GET /":                 p.handleIndex,
		"GET /help":             p.handleHelp,
		"POST /power":           p.handlePower,
		"POST /cut":             p.handleCut,
		"POST /recover":         p.handleRecover,
		"POST /config":          p.handleConfig,
		"POST /hotspot":         p.handleHotspot,
		"POST /advanced":        p.handleAdvanced,
		"POST /logout":          p.handleLogout,
		"GET /status.json":      p.handleStatusJSON,
		"GET /login":            p.handleLoginForm,
		"POST /login":           p.handleLogin,
		"GET /setup":            p.handleSetupForm,
		"POST /setup":           p.handleSetup,
		"GET /assets/panel.css": p.serveAsset("panel.css", "text/css; charset=utf-8"),
		"GET /assets/panel.js":  p.serveAsset("panel.js", "text/javascript; charset=utf-8"),
		"GET /favicon.svg":      p.serveAsset("favicon.svg", "image/svg+xml"),
	}

	seen := map[string]bool{}
	for _, rt := range routes {
		key := rt.Method + " " + rt.Path
		h, ok := handlers[key]
		if !ok {
			return nil, fmt.Errorf("panel: route %q has no handler", key)
		}
		seen[key] = true
		if !rt.Public {
			h = p.requireSession(h, rt.JSON)
		}
		p.mux.HandleFunc(key, h)
	}
	for key := range handlers {
		if !seen[key] {
			return nil, fmt.Errorf("panel: handler %q is not in the route table, so nothing decided whether it needs a session", key)
		}
	}
	return p, nil
}

// ListenAddrs is where this panel was configured to serve.
func (p *Panel) ListenAddrs() []string { return append([]string(nil), p.listenAddrs...) }

// ServeHTTP applies the whole-panel rules and dispatches.
func (p *Panel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	p.setSecurityHeaders(w)

	// The language is resolved once, here, and carried on the context. Doing it
	// per handler would mean a handler could forget, and the one that forgot
	// would serve Persian to somebody who had chosen English with no error
	// anywhere.
	r = r.WithContext(context.WithValue(r.Context(), langCtxKey, p.langFor(w, r)))

	// Refuse an unsafe request that came from another origin before any
	// handler runs, so that no route can forget to ask. The per-form tokens
	// are the first lock; this is the second. See sameOriginPost.
	if r.Method != http.MethodGet && r.Method != http.MethodHead && !sameOriginPost(r) {
		p.log.Warn("refused a cross-site request", "path", r.URL.Path)
		p.renderProblem(w, r, http.StatusForbidden, Problem{
			Headline: MsgCrossSite, Advice: MsgCrossSiteAdvice,
		})
		return
	}
	p.mux.ServeHTTP(w, r)
}

// setSecurityHeaders writes the headers that apply to every response.
//
// The Content-Security-Policy is the enforcement half of design section 5.7,
// "The panel fetches nothing". The Go tests check that no asset and no rendered
// page names an external URL; this makes the browser refuse one that got past
// them. default-src 'none' means anything not listed below is blocked outright,
// and every source that is listed is 'self'.
//
// form-action 'self' stops a submission being redirected off the box, which is
// the shape a stolen-credential bug takes on a login form. base-uri 'none'
// stops an injected <base> from re-pointing every relative URL on the page,
// which would defeat the 'self' sources. frame-ancestors 'none' stops the panel
// being framed, which is what makes a clickjacked switch impossible.
//
// Cache-Control is no-store on everything rather than only on the pages that
// need it. The main page shows the hotspot passphrase in the clear, because
// somebody standing at the box has to be able to read it, and a cached copy of
// that page is a copy of the passphrase in a place nobody thinks to look. The
// assets could be cached, and are not, because one rule with no exceptions is
// easier to keep right than a rule with a list.
func (p *Panel) setSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Security-Policy",
		"default-src 'none'; "+
			"script-src 'self'; "+
			"style-src 'self'; "+
			"img-src 'self'; "+
			"connect-src 'self'; "+
			"form-action 'self'; "+
			"base-uri 'none'; "+
			"frame-ancestors 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")
	h.Set("Pragma", "no-cache")
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

type ctxKey int

const (
	sessionCtxKey ctxKey = iota
	langCtxKey
)

// langFrom returns the language chosen for this request. ServeHTTP resolves it
// once and puts it here, so no handler can forget to and quietly serve the
// default to somebody who asked for the other one.
func langFrom(r *http.Request) Lang {
	if l, ok := r.Context().Value(langCtxKey).(Lang); ok && l.Valid() {
		return l
	}
	return DefaultLang
}

// requireSession is the gate in front of every route that is not in the public
// five.
//
// Order matters here. The first question is not "is this browser logged in" but
// "has a password been chosen at all", because design section 3 requires the
// first run to show setup rather than a login form: a login form on a box with
// no password is a dead end with no way out of it.
func (p *Panel) requireSession(h http.HandlerFunc, isJSON bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.store.Snapshot().Panel.IsSet() {
			if isJSON {
				writeJSONError(w, http.StatusUnauthorized, "setup is not finished")
				return
			}
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		sess, ok := p.currentSession(r)
		if !ok {
			if isJSON {
				writeJSONError(w, http.StatusUnauthorized, "not signed in")
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Cross-site request forgery. SameSite=Strict on the cookie is the
		// first lock and this is the second; see the note on session.csrf.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if err := r.ParseForm(); err != nil {
				p.renderProblem(w, r, http.StatusBadRequest, Problem{
					Headline: MsgBadForm, Advice: MsgBadFormAdvice,
				})
				return
			}
			if !sess.checkCSRF(r.PostFormValue("csrf")) {
				// Deliberately not a redirect. A redirect would look like it
				// worked, and this is the case where something other than the
				// panel's own page submitted the form.
				p.renderProblem(w, r, http.StatusForbidden, Problem{
					Headline: MsgBadForm, Advice: MsgBadFormAdvice,
				})
				return
			}
		}
		h(w, r.WithContext(context.WithValue(r.Context(), sessionCtxKey, sess)))
	}
}

func (p *Panel) currentSession(r *http.Request) (*session, bool) {
	c, err := r.Cookie(p.cookieName)
	if err != nil {
		return nil, false
	}
	return p.sessions.lookup(c.Value)
}

// sessionFrom returns the session a gated handler is running under. It is never
// nil in such a handler, because requireSession put it there.
func sessionFrom(r *http.Request) *session {
	s, _ := r.Context().Value(sessionCtxKey).(*session)
	return s
}

func csrfFrom(r *http.Request) string {
	if s := sessionFrom(r); s != nil {
		return s.csrf
	}
	return ""
}

// ---------------------------------------------------------------------------
// Calling the privileged service
// ---------------------------------------------------------------------------

// privCtx bounds one call to the privileged service.
func (p *Panel) privCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), p.privTimeout)
}

// status polls the privileged service, turning an unreachable service into a
// zero status with a fault rather than an error, so that the panel still draws
// when the service is down. A panel that will not render when the box is broken
// is a panel that is missing exactly when it is needed, which is the same
// argument design section 5.7 makes about remote assets.
func (p *Panel) status(r *http.Request) (SystemStatus, Fault) {
	ctx, cancel := p.privCtx(r)
	defer cancel()
	st, err := p.priv.Status(ctx)
	if err != nil {
		f := FaultOf(err)
		if f == FaultUnknown {
			f = FaultUnavailable
		}
		p.log.Warn("privileged status call failed", "fault", string(f))
		return SystemStatus{}, f
	}
	return st, FaultNone
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	// Written by hand rather than through encoding/json: the message is a
	// constant from this file in every call site, so there is nothing to
	// escape, and this cannot fail halfway through and leave a half-written
	// body after the header has gone.
	_, _ = io.WriteString(w, `{"error":`+quoteJSON(msg)+`}`)
}

// quoteJSON quotes a plain ASCII string for JSON. It is used only on constants
// declared in this package.
func quoteJSON(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\\':
			out = append(out, '\\', c)
		case '\n':
			out = append(out, '\\', 'n')
		default:
			if c < 0x20 {
				out = append(out, ' ')
				continue
			}
			out = append(out, c)
		}
	}
	return string(append(out, '"'))
}
