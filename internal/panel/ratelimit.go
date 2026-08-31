// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Rate limiting for failed password attempts.
//
// The threat is a device on the hotspot, or on the local network when the user
// has opened the panel there, guessing the panel password. The password is
// verified with Argon2id at 64 MiB and three passes (internal/state/password.go),
// so each guess is already expensive, and that is the point of the limit as much
// as the guessing is: an unlimited attempt rate against a memory-hard hash is a
// way to take the box's memory away from the engine and the access point. Both
// reasons want the same thing, which is a cap on attempts per unit time.
//
// Two buckets, because either alone is easy to get around. The per-address
// bucket stops one device grinding. The whole-panel bucket stops a device that
// can pick its own address from getting a fresh allowance per address, which on
// a network the attacker is already attached to is trivial.
const (
	// attemptBurst is how many wrong passwords one address may give before it
	// has to wait. Five is enough for a person mistyping a passphrase they
	// have written down.
	attemptBurst = 5

	// attemptRefill is how long one attempt takes to come back.
	attemptRefill = 60 * time.Second

	// globalBurst and globalRefill are the same idea across every address at
	// once. The burst is larger so that a household of devices does not lock
	// itself out, and the refill is faster so that a genuine user who waits is
	// not held behind an attacker's exhausted allowance for long.
	globalBurst  = 20
	globalRefill = 15 * time.Second

	// maxLimiterKeys bounds the per-address map.
	maxLimiterKeys = 4096
)

// bucket is a token bucket holding fractional tokens as a time.
//
// Rather than a count and a timestamp, it stores the instant at which the
// bucket would be full. Tokens available now is (full - now) turned into a
// count. That makes refill exact with no periodic sweep and no rounding drift.
type bucket struct {
	fullAt time.Time
}

// tokens reports how many whole attempts are available at now.
func (b bucket) tokens(now time.Time, burst int, refill time.Duration) int {
	if !now.Before(b.fullAt) {
		return burst
	}
	missing := int((b.fullAt.Sub(now) + refill - 1) / refill)
	if missing > burst {
		missing = burst
	}
	return burst - missing
}

// take consumes one attempt, and reports whether one was available and how long
// until the next one is.
func (b *bucket) take(now time.Time, burst int, refill time.Duration) (ok bool, retryAfter time.Duration) {
	if b.fullAt.Before(now) {
		b.fullAt = now
	}
	if b.tokens(now, burst, refill) <= 0 {
		return false, b.fullAt.Sub(now) - time.Duration(burst-1)*refill
	}
	b.fullAt = b.fullAt.Add(refill)
	return true, 0
}

// attemptLimiter is the pair of buckets.
type attemptLimiter struct {
	mu     sync.Mutex
	byAddr map[string]*bucket
	global bucket
	now    func() time.Time
}

func newAttemptLimiter(now func() time.Time) *attemptLimiter {
	return &attemptLimiter{byAddr: map[string]*bucket{}, now: now}
}

// allow reports whether an attempt from key may proceed, and consumes one.
//
// It is called BEFORE the password is verified rather than after a failure. A
// limiter that only counts failures still lets an attacker spend the box's
// memory on one Argon2id derivation per request, which on a Raspberry Pi that
// is also running an access point is a denial of service in itself. A
// successful attempt gives its token back through succeed.
func (l *attemptLimiter) allow(key string) (ok bool, retryAfter time.Duration) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if ok, after := l.global.take(now, globalBurst, globalRefill); !ok {
		return false, after
	}
	b, found := l.byAddr[key]
	if !found {
		l.pruneLocked(now)
		b = &bucket{fullAt: now}
		l.byAddr[key] = b
	}
	ok, after := b.take(now, attemptBurst, attemptRefill)
	if !ok {
		return false, after
	}
	return true, 0
}

// succeed returns the attempt a correct password consumed, so that somebody who
// logs in, logs out and logs in again is not walking towards a lockout.
func (l *attemptLimiter) succeed(key string) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if b, found := l.byAddr[key]; found {
		b.fullAt = b.fullAt.Add(-attemptRefill)
		if b.fullAt.Before(now) {
			b.fullAt = now
		}
	}
	l.global.fullAt = l.global.fullAt.Add(-globalRefill)
	if l.global.fullAt.Before(now) {
		l.global.fullAt = now
	}
}

// pruneLocked drops entries that have refilled, and if that is not enough,
// drops the entry with the most tokens left.
//
// Evicting the fullest entry rather than the oldest is deliberate. Eviction
// resets an address's allowance, so evicting the entry closest to a lockout
// would hand an attacker a way to clear their own limit by filling the map from
// other addresses. The fullest entry is the one that has been penalised least,
// so losing it costs the least.
func (l *attemptLimiter) pruneLocked(now time.Time) {
	if len(l.byAddr) < maxLimiterKeys {
		return
	}
	for k, b := range l.byAddr {
		if !now.Before(b.fullAt) {
			delete(l.byAddr, k)
		}
	}
	for len(l.byAddr) >= maxLimiterKeys {
		var fullestKey string
		var fullest time.Time
		for k, b := range l.byAddr {
			if fullestKey == "" || b.fullAt.Before(fullest) {
				fullestKey, fullest = k, b.fullAt
			}
		}
		if fullestKey == "" {
			return
		}
		delete(l.byAddr, fullestKey)
	}
}

// clientKey is what the limiter counts against: the client's IP address.
//
// X-Forwarded-For and every other proxy header are ignored, and that is a
// decision rather than an oversight. The panel binds to the hotspot interface
// and optionally the local network (design section 5.6) and is never behind a
// proxy, so any such header is attacker-supplied text. Honouring one would let
// a single device present a new identity per request and erase the per-address
// limit entirely.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// A RemoteAddr with no port is not something net/http produces, but a
		// custom listener could. Using it whole is better than using nothing,
		// which would put every client in one bucket.
		return r.RemoteAddr
	}
	return host
}
