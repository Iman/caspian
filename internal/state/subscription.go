// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Quota is what a provider reported about the account when the config was
// last refreshed, in bytes and unix seconds. Plain integers: usage figures
// identify nobody, and the panel shows them as they are.
//
// Zero throughout means the provider sent nothing, and the panel draws no
// usage line for it.
type Quota struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
	Expire   int64 `json:"expire"`
}

// maxSubscriptionURL bounds the address. Provider addresses are a name, a
// path and a token, and the longest seen in the wild are a few hundred bytes;
// 2048 is the bound most browsers apply and is far above any real one.
const maxSubscriptionURL = 2048

// The rules a subscription address has to meet. Each is its own sentinel so
// the panel can word each refusal differently, and none of the sentences
// carries the value: the address is a credential, and the error is the thing
// most likely to be printed.
var (
	// ErrSubscriptionNotHTTPS: the address does not start with https. Plain
	// http would carry the account token in the clear to whoever sits on the
	// path, tunnel or not.
	ErrSubscriptionNotHTTPS = errors.New("state: the subscription address must start with https")

	// ErrSubscriptionNoHost: there is nothing to connect to.
	ErrSubscriptionNoHost = errors.New("state: the subscription address has no host name")

	// ErrSubscriptionIPLiteral: the host is an IP address rather than a name.
	// The engine routes a private IP literal out of the direct outbound
	// (internal/xcfg, private-direct), and a refresh that leaves the box
	// directly is the one thing this feature promises never to do. A name
	// reaches the proxy outbound in every case; see
	// internal/xcfg/hostname_routing_test.go.
	ErrSubscriptionIPLiteral = errors.New("state: the subscription address must use a name, not an IP address")

	// ErrSubscriptionUserinfo: a user or password before the host. Tokens
	// belong in the path or the query, and an address in that shape is a
	// sign the person pasted something other than what the provider gave.
	ErrSubscriptionUserinfo = errors.New("state: the subscription address must not carry a user name or password before the host")

	// ErrSubscriptionTooLong: longer than maxSubscriptionURL bytes.
	ErrSubscriptionTooLong = errors.New("state: the subscription address is longer than this box stores")

	// ErrSubscriptionMalformed: not an address at all.
	ErrSubscriptionMalformed = errors.New("state: the subscription address could not be read as an address")
)

// HasSubscription reports whether a subscription address is stored, without
// revealing it.
func (p ProxyConfig) HasSubscription() bool { return !p.SubscriptionURL.IsZero() }

// validateSubscriptionURL applies the rules above. The value is never part of
// the error.
func validateSubscriptionURL(raw string) error {
	if len(raw) > maxSubscriptionURL {
		return ErrSubscriptionTooLong
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return ErrSubscriptionMalformed
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		// url.Parse quotes the input in its error; that error is dropped
		// here for the same reason the sentinel carries no value.
		return ErrSubscriptionMalformed
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return ErrSubscriptionNotHTTPS
	}
	if u.User != nil {
		return ErrSubscriptionUserinfo
	}
	host := u.Hostname()
	if host == "" {
		return ErrSubscriptionNoHost
	}
	// Hostname strips the brackets from an IPv6 literal, so one check covers
	// both families and the bracketed form. A zone suffix is stripped too,
	// because "fe80::1%eth0" is an address whichever way it is written.
	if zone := strings.IndexByte(host, '%'); zone >= 0 {
		host = host[:zone]
	}
	if net.ParseIP(host) != nil {
		return ErrSubscriptionIPLiteral
	}
	return nil
}

// SetSubscriptionURL stores the address a refresh is fetched from, after
// checking it against the rules above. It touches nothing else: the config,
// its selection and its figures all stay as they were, because a person can
// set the address before they have a config or long after.
func (s *Store) SetSubscriptionURL(u string) error {
	if err := validateSubscriptionURL(u); err != nil {
		return err
	}
	return s.Update(func(st *State) error {
		st.Proxy.SubscriptionURL = Secret(u)
		return nil
	})
}

// RecordRefresh stores a config that was fetched from the subscription
// address, with the figures the provider sent beside it.
//
// It has the semantics of SetProxyConfig, because a refreshed body is a new
// list: the selection goes back to the first entry and AddedAt is the refresh
// time. What it adds is the quota and the refresh time; what it keeps is the
// address, which is where the next refresh comes from. It refuses when no
// address is stored, because a refresh with no source is a paste wearing the
// wrong name, and it refuses negative figures, which no provider sends and
// which the panel would otherwise have to defend against.
func (s *Store) RecordRefresh(raw, scheme, label string, q Quota, at time.Time) error {
	return s.Update(func(st *State) error {
		if raw == "" {
			return errors.New("state: proxy config must not be empty")
		}
		if !st.Proxy.HasSubscription() {
			return errors.New("state: no subscription address is stored, so there is nothing to refresh from")
		}
		if q.Upload < 0 || q.Download < 0 || q.Total < 0 || q.Expire < 0 {
			return fmt.Errorf("state: the usage figures from a refresh cannot be negative")
		}
		st.Proxy.Raw = Secret(raw)
		st.Proxy.Scheme = scheme
		st.Proxy.Label = label
		st.Proxy.Selected = 0
		st.Proxy.AddedAt = at.UTC()
		st.Proxy.RefreshedAt = at.UTC()
		st.Proxy.Quota = q
		return nil
	})
}
