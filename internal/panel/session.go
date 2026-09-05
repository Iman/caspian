// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// Session lifetimes. A panel session is a person standing at a box on their own
// network, not a bank login, so the absolute lifetime is generous and the idle
// timeout is what actually ends most sessions.
const (
	sessionLifetime = 12 * time.Hour
	sessionIdle     = 2 * time.Hour

	// maxSessions bounds memory. Nothing legitimate needs more than a handful;
	// the number is high enough that a household never meets it and low enough
	// that the map cannot grow without limit.
	maxSessions = 64
)

// sessionCookie is the cookie name when the panel is served over plain HTTP.
// See newSessionCookie for the __Host- variant and for why plain HTTP is the
// normal case here.
const sessionCookie = "caspian_session"

// session is one logged-in browser.
//
// It holds no password, no password hash and nothing derived from either. The
// cookie value is a bearer token and this is what it refers to.
type session struct {
	// csrf is a per-session token that every state-changing form must carry.
	// SameSite=Strict already stops a cross-site form post in every browser
	// that honours it, so this is the second lock rather than the first: it
	// also covers a browser that does not, and it makes the requirement
	// visible in the templates instead of resting on a cookie attribute
	// nobody can see.
	//
	// It is written once at creation and never again, so it is read without
	// the mutex below.
	csrf string

	created time.Time

	// mu guards everything from here down. The session store's own lock
	// guards the map; this one guards one session's contents, so a slow
	// handler holding a session does not block every other request.
	mu       sync.Mutex
	lastSeen time.Time

	// flashProblem and flashNotice carry one message from a POST to the GET
	// that follows it.
	//
	// They live in the session rather than in the URL, and that is the whole
	// reason they exist. Every POST here ends in a redirect, so the result of
	// adding a config has to reach the next page somehow, and the two other
	// ways of doing it are both wrong for this panel: a query parameter would
	// put a message about the user's config into a URL, which lands in browser
	// history and in any log a future version might grow, and rendering the
	// result directly from the POST would leave a page that re-submits the
	// config when it is reloaded.
	flashProblem Problem
	flashNotice  Key
}

// touch records that the session was just used.
func (s *session) touch(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSeen = now
}

func (s *session) seenAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSeen
}

// setFlash stores the message the next page should show, replacing any
// message that has not been collected.
func (s *session) setFlash(p Problem, notice Key) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flashProblem, s.flashNotice = p, notice
}

// flash returns the pending message WITHOUT clearing it.
//
// Not clearing on read is deliberate, and it was a defect before it was a
// decision. The message after a rejected config says "Advanced mode shows what
// the software said"; the user then clicks the advanced link, which is a plain
// GET, and with a read-once flash the whole banner vanished on the way. The
// panel promised something and then took it away, which is worse than not
// having offered.
//
// So a message stays until the next action replaces it: every POST that can
// fail ends in setFlash, so the banner always describes the last thing the user
// did rather than accumulating. clearFlash covers signing out.
//
// The cost is that a message survives a page reload. That is the right side of
// the trade for this audience: somebody who has just been told something went
// wrong is more likely to reload the page than to accept that the explanation
// was a one-time offer.
func (s *session) flash() (Problem, Key) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flashProblem, s.flashNotice
}

// clearFlash drops the pending message.
func (s *session) clearFlash() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flashProblem, s.flashNotice = Problem{}, ""
}

// sessionStore holds the live sessions.
//
// The map is keyed by the SHA-256 of the token rather than by the token itself.
// The token is what the browser holds; the store holds only a value derived
// from it. So a memory disclosure, a dump, or anything that renders the store
// yields nothing that can be replayed as a cookie. It also means the map lookup
// is over a fixed-width digest rather than over the secret.
type sessionStore struct {
	mu   sync.Mutex
	byID map[string]*session
	now  func() time.Time
}

func newSessionStore(now func() time.Time) *sessionStore {
	return &sessionStore{byID: map[string]*session{}, now: now}
}

func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newToken returns 256 bits of randomness in base64url.
//
// crypto/rand.Read is documented never to return a short read without an
// error, and on every platform this targets it cannot fail; the error is
// checked anyway, because a silent fallback to a weak token would be a session
// anyone could guess.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// create makes a new session and returns its token.
func (s *sessionStore) create() (token string, sess *session, err error) {
	token, err = newToken()
	if err != nil {
		return "", nil, err
	}
	csrf, err := newToken()
	if err != nil {
		return "", nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	if len(s.byID) >= maxSessions {
		// Evict the least recently used rather than refusing to log in. A
		// panel that cannot be logged into is a box the user cannot fix, and
		// the design already names being locked out of the panel as a hazard
		// (section 5.6).
		s.evictOldestLocked()
	}
	now := s.now()
	sess = &session{csrf: csrf, created: now, lastSeen: now}
	s.byID[sessionKey(token)] = sess
	return token, sess, nil
}

// lookup returns the session for a token, and marks it as just used.
func (s *sessionStore) lookup(token string) (*session, bool) {
	if token == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	sess, ok := s.byID[sessionKey(token)]
	if !ok {
		return nil, false
	}
	sess.touch(s.now())
	return sess, true
}

// destroy ends one session.
func (s *sessionStore) destroy(token string) {
	if token == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byID, sessionKey(token))
}

// destroyAll invalidates every browser session after a password change.
func (s *sessionStore) destroyAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID = make(map[string]*session)
}

func (s *sessionStore) pruneLocked() {
	now := s.now()
	for k, v := range s.byID {
		if now.Sub(v.created) > sessionLifetime || now.Sub(v.seenAt()) > sessionIdle {
			delete(s.byID, k)
		}
	}
}

func (s *sessionStore) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	for k, v := range s.byID {
		seen := v.seenAt()
		if oldestKey == "" || seen.Before(oldest) {
			oldestKey, oldest = k, seen
		}
	}
	if oldestKey != "" {
		delete(s.byID, oldestKey)
	}
}

// checkCSRF compares a submitted token against the session's, in constant time.
func (s *session) checkCSRF(got string) bool {
	if s == nil || s.csrf == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(s.csrf), []byte(got)) == 1
}

// newSessionCookie builds the cookie for a token.
//
// About Secure. The panel is normally reached at http://<hotspot address>,
// because there is no way to obtain a certificate for a private address on a
// box with no name, and design section 5.6 puts the panel on the hotspot
// interface where the only listeners are the user's own devices. A Secure
// cookie is never sent over plain HTTP, so setting it unconditionally would
// produce a panel that accepts a password and then behaves as though nobody had
// logged in. So Secure follows the deployment: Config.TLS says the panel is
// served over HTTPS, and only then is the cookie marked Secure and given the
// __Host- prefix, which additionally pins it to this exact origin with no
// Domain attribute and Path=/.
//
// What holds in both cases: HttpOnly, so script cannot read it; SameSite=Strict,
// so it is not sent on any cross-site navigation; no Expires and no Max-Age, so
// it is a session cookie the browser drops when it closes; and a value that is
// a bearer token for a server-side record rather than anything derived from the
// password.
func newSessionCookie(name, token string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
}

// cookieName is the session cookie's name for this deployment.
func cookieName(secure bool) string {
	if secure {
		return "__Host-" + sessionCookie
	}
	return sessionCookie
}

// clearSessionCookie is the cookie that removes the session cookie. The
// attributes have to match the ones it was set with or some browsers keep the
// original.
func clearSessionCookie(name string, secure bool) *http.Cookie {
	c := newSessionCookie(name, "", secure)
	c.MaxAge = -1
	return c
}
