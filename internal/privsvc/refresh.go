// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// ---------------------------------------------------------------------------
// The subscription refresh
//
// This is the one request Caspian makes for content, and it is made HERE, in
// the privileged process, for one reason: the request has to travel through
// the tunnel, and the tunnel's loopback SOCKS inbound (internal/xcfg, TagSOCKSIn
// at 127.0.0.1:SocksPort) exists only while the engine this process owns is
// running. The panel keeps no HTTP client and gains none.
//
// Three properties make "through the tunnel" a fact rather than a hope, and
// each is held by a test in refresh_test.go:
//
//   - No engine, no request. The engine phase is checked before any socket
//     opens, and a refresh on a stopped box is refused with FaultNotRunning.
//   - The NAME goes to the proxy. The address must have a host name, never an
//     IP literal, because the engine routes a private literal out of the
//     direct outbound (internal/xcfg/hostname_routing_test.go pins that a
//     hostname on the SOCKS inbound can reach only the proxy). And the SOCKS
//     client hands that name through unresolved (golang.org/x/net/proxy sends
//     it as a domain-typed destination), so this machine's resolver is never
//     asked and the provider's name never reaches the ISP's resolver.
//   - Plain http is refused at the address and at every redirect, so the
//     account token in the address is never sent in the clear.
//
// What the privileged side keeps between presses: nothing. The address arrives
// in the request, the body leaves in the reply, and no field of Service holds
// either. The panel is the one place the address is stored.
//
// What crosses back: the status, the body for a 2xx answer only, and four
// headers by name. A provider's error page, cookies and any other header stay
// on this side and are dropped.
//
// What never reaches an error or a log line: the address, or the host name
// inside it. Every error this file returns wraps a fixed sentinel, and the
// url.Error the client produces, which quotes the address, is classified and
// dropped here.
// ---------------------------------------------------------------------------

// maxRefreshBodyBytes bounds the body, and it is the same bound the panel
// applies to a pasted config (internal/panel, maxBodyBytes): a body over it
// could not have been pasted either.
const maxRefreshBodyBytes = 256 << 10

// maxRefreshHeaderBytes bounds each header value that crosses back. The four
// headers are a usage line, a title, a file name and a number; anything longer
// is not one of those and is dropped rather than cut, because a cut title is
// a wrong title.
const maxRefreshHeaderBytes = 1024

// refreshTimeout bounds the whole fetch, redirects included.
const refreshTimeout = 30 * time.Second

// maxRefreshRedirects is how many redirects are followed. A fourth is refused
// as no answer.
const maxRefreshRedirects = 3

// refreshHeaderAllowList is every header the reply carries, keyed in lower
// case. The panel reads these and no others; see panel.RefreshReply.
var refreshHeaderAllowList = []string{
	"subscription-userinfo",
	"profile-title",
	"content-disposition",
	"profile-update-interval",
}

// The fixed causes. Each is a sentence with no value in it, and each is what
// a faultError wraps, so an in-process caller printing the error sees these
// and never the url.Error that quotes the address.
var (
	errRefreshNotRunning       = errors.New("the tunnel is not running")
	errRefreshBadAddress       = errors.New("the subscription address is not one this box will fetch from")
	errRefreshNoAnswer         = errors.New("the provider did not answer")
	errRefreshTooLarge         = errors.New("the provider's answer is larger than a config can be")
	errRefreshNotHTTPS         = errors.New("the provider redirected to an address that is not https")
	errRefreshTooManyRedirects = errors.New("the provider redirected too many times")
)

// Refresh implements panel.Privileged.
//
// It takes neither lock. opMu would make a slow provider block a Stop for up
// to refreshTimeout, and there is nothing to protect: this method reads the
// engine phase and touches no field of Service. If the engine stops half way
// through, the inbound goes with it and the fetch fails as no answer.
func (s *Service) Refresh(ctx context.Context, req panel.RefreshRequest) (panel.RefreshReply, error) {
	if s.cfg.Engine.State().Phase != engine.PhaseRunning {
		s.cfg.Logger.Warn("subscription refresh refused", "fault", string(panel.FaultNotRunning))
		return panel.RefreshReply{}, fail("refresh", panel.FaultNotRunning, errRefreshNotRunning)
	}
	target, err := url.Parse(req.URL)
	if err != nil || checkRefreshAddress(target) != nil {
		// Whatever internal/state let through, or whatever a caller that is
		// not the panel sent. The parse error quotes the input and is dropped.
		s.cfg.Logger.Warn("subscription refresh refused", "fault", string(panel.FaultRefreshBadAddress))
		return panel.RefreshReply{}, fail("refresh", panel.FaultRefreshBadAddress, errRefreshBadAddress)
	}

	client, err := s.refreshClient()
	if err != nil {
		return panel.RefreshReply{}, fail("refresh", panel.FaultUnknown, err)
	}
	defer client.Transport.(*http.Transport).CloseIdleConnections()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return panel.RefreshReply{}, fail("refresh", panel.FaultRefreshBadAddress, errRefreshBadAddress)
	}
	httpReq.Header.Set("User-Agent", "Caspian/"+panel.Version)

	resp, err := client.Do(httpReq)
	if err != nil {
		f, cause := classifyRefreshError(err)
		s.cfg.Logger.Warn("subscription refresh failed", "fault", string(f))
		return panel.RefreshReply{}, fail("refresh", f, cause)
	}
	defer resp.Body.Close()

	reply := panel.RefreshReply{Status: resp.StatusCode, Headers: map[string]string{}}
	for _, name := range refreshHeaderAllowList {
		v := resp.Header.Get(name)
		if v == "" || len(v) > maxRefreshHeaderBytes {
			continue
		}
		reply.Headers[name] = v
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// The number is the answer. The body is an error page and is not read.
		s.cfg.Logger.Warn("subscription refresh answered with an error status", "status", resp.StatusCode)
		return reply, nil
	}

	// One byte past the cap is read so that "exactly the cap" and "over the
	// cap" are told apart without reading the whole of an oversized answer.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRefreshBodyBytes+1))
	if err != nil {
		s.cfg.Logger.Warn("subscription refresh failed", "fault", string(panel.FaultRefreshNoAnswer))
		return panel.RefreshReply{}, fail("refresh", panel.FaultRefreshNoAnswer, errRefreshNoAnswer)
	}
	if len(body) > maxRefreshBodyBytes {
		s.cfg.Logger.Warn("subscription refresh failed", "fault", string(panel.FaultRefreshTooLarge))
		return panel.RefreshReply{}, fail("refresh", panel.FaultRefreshTooLarge, errRefreshTooLarge)
	}
	reply.Body = body
	s.cfg.Logger.Info("subscription refreshed", "status", resp.StatusCode, "bytes", len(body))
	return reply, nil
}

// checkRefreshAddress is the rule set internal/state applies on the way in,
// applied again here to every address this process is about to connect to:
// the one in the request and every redirect target. The error is one of two
// sentinels and carries no part of the address.
func checkRefreshAddress(u *url.URL) error {
	if u == nil {
		return errRefreshBadAddress
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return errRefreshNotHTTPS
	}
	if u.User != nil {
		return errRefreshBadAddress
	}
	host := u.Hostname()
	if host == "" {
		return errRefreshBadAddress
	}
	if zone := strings.IndexByte(host, '%'); zone >= 0 {
		host = host[:zone]
	}
	if net.ParseIP(host) != nil {
		return errRefreshBadAddress
	}
	return nil
}

// refreshClient builds the client for one press.
//
// Built per press and thrown away, with keep-alives off, so nothing is held
// between presses. Every connection it makes goes to the engine's loopback
// SOCKS inbound: the SOCKS dialer's forward dialer is given an IP literal and
// a port, so it resolves nothing, and the destination name is written into
// the SOCKS request for the engine to resolve through the tunnel.
func (s *Service) refreshClient() (*http.Client, error) {
	inbound := net.JoinHostPort(xcfg.DefaultSocksListen, strconv.Itoa(int(s.cfg.SocksPort)))
	forward := &net.Dialer{Timeout: 5 * time.Second}
	socks, err := proxy.SOCKS5("tcp", inbound, nil, forward)
	if err != nil {
		return nil, err
	}
	dialer, ok := socks.(proxy.ContextDialer)
	if !ok {
		return nil, errors.New("privsvc: the SOCKS dialer does not take a context")
	}

	transport := &http.Transport{
		// nil, explicitly: no environment proxy is consulted. The only
		// proxy is the one DialContext goes through.
		Proxy:       nil,
		DialContext: dialer.DialContext,
		// Verification on, against the system roots unless a test supplied
		// a pool. There is no InsecureSkipVerify and there will not be.
		TLSClientConfig:        &tls.Config{RootCAs: s.cfg.RefreshRootCAs},
		DisableKeepAlives:      true,
		ForceAttemptHTTP2:      false,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  refreshTimeout,
		MaxResponseHeaderBytes: 64 << 10,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   refreshTimeout,
		// No cookie jar: nothing a provider sets is kept or sent back.
		Jar: nil,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > maxRefreshRedirects {
				return errRefreshTooManyRedirects
			}
			// The same rules as the address itself, on every hop, so a
			// redirect cannot reach a plain address or an IP literal.
			return checkRefreshAddress(req.URL)
		},
	}, nil
}

// classifyRefreshError reduces whatever the client returned to a fault and
// a fixed cause. The client's own error is a url.Error carrying the address,
// and it is dropped here on purpose.
func classifyRefreshError(err error) (panel.Fault, error) {
	switch {
	case errors.Is(err, errRefreshNotHTTPS):
		return panel.FaultRefreshNotHTTPS, errRefreshNotHTTPS
	case errors.Is(err, errRefreshBadAddress):
		return panel.FaultRefreshBadAddress, errRefreshBadAddress
	case errors.Is(err, errRefreshTooManyRedirects):
		return panel.FaultRefreshNoAnswer, errRefreshTooManyRedirects
	default:
		// A dial that failed, a certificate that did not verify, a
		// connection that dropped, a deadline: nothing complete came back.
		return panel.FaultRefreshNoAnswer, errRefreshNoAnswer
	}
}
