// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/privsvc"
	"caspianbyoc.org/caspian/internal/state"
)

// bindRefreshInterval is how often the panel looks again for somewhere to
// listen.
//
// It exists because of the hazard docs/2026-08-29-design.md section 5.6 names
// and does not solve: "the hotspot interface does not exist until the access
// point starts". So the address the panel is meant to serve on appears LATER,
// after somebody has already used the panel to switch the hotspot on. Binding
// once at startup would mean the panel never appears on the hotspot until the
// service is restarted.
const bindRefreshInterval = 15 * time.Second

// loopbackHost is bound in addition to whatever is detected.
//
// This is an ADDITION to design section 5.6 and worth being explicit about,
// because that section is a list of where the panel listens. Its three clauses
// are "the hotspot interface, always", "the local network only if the user
// turns that on" and "never the uplink", and loopback is none of the three: it
// is not a network anybody else is on. Reaching it requires already having an
// account on the box, which is a strictly stronger position than being on the
// hotspot, so it grants nothing that was not already held.
//
// What it buys is the one thing the section leaves open. On a fresh box the
// hotspot does not exist, the local network is off by default, and there is
// therefore nowhere at all to serve; the password the installer printed opens
// nothing. With loopback bound, an operator with shell access can reach the
// panel over an SSH port forward and finish the setup. It does not fix the
// hazard for a user who has no shell; nothing here can.
const loopbackHost = "127.0.0.1"

// privCallTimeout bounds every call the panel makes to the privileged service.
//
// internal/panel's own default is 20 seconds, documented there as "long enough
// for a real start on a Raspberry Pi". That is a claim nobody in this tree has
// measured, and getting it wrong is not a slow screen. The panel's context is
// what the privileged side derives its own deadline from, so a start that takes
// longer than this is CUT OFF PART WAY THROUGH and rolled back, every time, and
// the user has no way to get past it.
//
// What a start does: about twenty netcfg commands, a pinned host route, an
// engine start, and two daemons each polled for a pid file up to twenty times
// at 100ms (internal/hotspot/supervisor.go, StartTries and StartSettle).
// Ninety seconds is generous against that and still bounded. The cost of the
// larger number is only that a wedged privileged service holds one HTTP
// handler open for longer, which is what internal/panel's field exists to
// bound and not what it exists to minimise.
//
// It is set here rather than left to the default because docs/LAYOUT.md makes
// cmd/caspian the place values are passed in, and because a number chosen
// against an unmeasured claim belongs where somebody can see and change it.
// TestThePanelTimeoutIsOneThePrivilegedServiceWillHonour checks it against the
// bounds that service clamps to, so it cannot quietly become a number the two
// halves disagree about.
const privCallTimeout = 90 * time.Second

// servePanel runs the half that holds no privilege.
func servePanel(ctx context.Context, log *slog.Logger, extraListen []string) error {
	for _, a := range extraListen {
		if err := panel.ValidateBindAddr(a); err != nil {
			return fmt.Errorf("--listen %s was refused: %w", a, err)
		}
	}

	store, err := state.Load(stateDir)
	if err != nil {
		// internal/state refuses a corrupt file rather than replacing it with
		// defaults, because silently resetting throws away the user's proxy
		// config with no way to tell that happened. Its message says what to do.
		return err
	}

	if err := consumeFirstRunPassword(store, firstRunPasswordPath, log); err != nil {
		// Not fatal. A panel that will not start is a panel that cannot say
		// what is wrong, which design section 5.6 identifies as the worst
		// outcome for this product. The setup screen is still a way in.
		log.Error("the first-run password handoff did not complete", "error", err.Error())
	}
	if !store.Snapshot().Panel.IsSet() {
		log.Info("no panel password is set, so the panel will show its setup screen")
	}

	priv := privsvc.NewClient(socketPath)

	initial := desiredAddrs(ctx, priv, store, extraListen, log)
	p, err := panel.New(panel.Config{
		Store:       store,
		Priv:        priv,
		Logger:      log,
		ListenAddrs: initial,
		PrivTimeout: privCallTimeout,
	})
	if err != nil {
		return err
	}

	srv := &panelListeners{handler: p, log: log}
	defer srv.closeAll()

	srv.reconcile(initial)
	if len(srv.addrs()) == 0 {
		// Said once, at startup, in the words somebody looking for the panel
		// needs. It is not fatal: the loop below keeps looking, and the moment
		// the hotspot exists the panel appears on it.
		log.Warn("the panel has nowhere to listen yet: the hotspot has not been started, " +
			"so its interface does not exist. Reach the panel over an SSH forward to " +
			loopbackHost + ":" + strconv.Itoa(panelPort) + ", or pass --listen with an address on this box")
	}

	t := time.NewTicker(bindRefreshInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			srv.reconcile(desiredAddrs(ctx, priv, store, extraListen, log))
		}
	}
}

// desiredAddrs is where the panel should be serving right now.
//
// The rule is design section 5.6, with loopback added for the reason on
// loopbackHost: the hotspot interface always, the local network only when the
// user has turned it on AND the address there is private, never the uplink.
// internal/panel.BindAddrs applies the last two of those; this function adds
// the addresses that do not come from detection.
//
// The privileged service being unreachable is not an error here. The panel unit
// is ordered after it with Wants= and never Requires= precisely so the panel
// comes up and can say what is wrong, so a failed detection means "nothing
// detected yet" and the next tick asks again.
func desiredAddrs(ctx context.Context, priv panel.Privileged, store *state.Store, extra []string, log *slog.Logger) []string {
	set := map[string]bool{}
	add := func(a string) {
		if a == "" {
			return
		}
		if err := panel.ValidateBindAddr(a); err != nil {
			log.Warn("not serving on an address that was refused", "address", a, "reason", err.Error())
			return
		}
		set[a] = true
	}

	add(net.JoinHostPort(loopbackHost, strconv.Itoa(panelPort)))
	for _, a := range extra {
		add(a)
	}

	dctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	d, err := priv.Detect(dctx)
	if err != nil {
		if f := panel.FaultOf(err); f == panel.FaultUnavailable {
			log.Warn("the privileged service is not answering, so the panel does not know " +
				"where the hotspot is yet")
		} else {
			log.Warn("detection failed", "fault", string(panel.FaultOf(err)))
		}
	} else {
		onLAN := store.Advanced().PanelOnLAN
		addrs, err := panel.BindAddrs(d, panelPort, onLAN)
		if err != nil && !errors.Is(err, panel.ErrNoBindAddress) {
			log.Warn("the detected addresses were refused", "reason", err.Error())
		}
		for _, a := range addrs {
			add(a)
		}
	}

	out := make([]string, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	// Sorted so that the list a log line prints is the same on every tick for
	// the same set, which is what makes a change in it worth reading.
	sort.Strings(out)
	return out
}

// panelListeners keeps one HTTP server per address, adding and removing them as
// the set changes.
type panelListeners struct {
	handler http.Handler
	log     *slog.Logger

	mu     sync.Mutex
	active map[string]*panelListener
}

type panelListener struct {
	srv *http.Server
	ln  net.Listener
}

// reconcile opens what is new and closes what has gone.
func (s *panelListeners) reconcile(want []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		s.active = map[string]*panelListener{}
	}

	wanted := map[string]bool{}
	for _, a := range want {
		wanted[a] = true
	}

	for addr, pl := range s.active {
		if wanted[addr] {
			continue
		}
		// The address has gone: the hotspot was switched off, or the user
		// turned the local network off. Closing is what makes "never the
		// uplink" hold over time and not only at startup.
		s.log.Info("no longer serving the panel here", "address", addr)
		_ = pl.srv.Close()
		delete(s.active, addr)
	}

	for _, addr := range want {
		if s.active[addr] != nil {
			continue
		}
		// panel.Listen validates the address a second time, at the last point
		// before a socket exists. That repetition is internal/panel's decision
		// and is kept.
		ln, err := panel.Listen(addr)
		if err != nil {
			s.log.Warn("could not serve the panel here", "address", addr, "reason", err.Error())
			continue
		}
		srv := &http.Server{
			Handler: s.handler,
			// A slow or absent client must not hold a connection open on a
			// box with four cores and one user.
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		s.active[addr] = &panelListener{srv: srv, ln: ln}
		s.log.Info("panel available", "url", "http://"+addr+"/")
		go func(addr string, srv *http.Server, ln net.Listener) {
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.log.Warn("the panel stopped serving on an address", "address", addr, "error", err.Error())
			}
		}(addr, srv, ln)
	}
}

func (s *panelListeners) addrs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.active))
	for a := range s.active {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

func (s *panelListeners) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for addr, pl := range s.active {
		_ = pl.srv.Close()
		delete(s.active, addr)
	}
}
