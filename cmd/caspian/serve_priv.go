// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"caspianbyoc.org/caspian/internal/privsvc"
)

// shutdownGrace is how long a clean stop is given.
//
// packaging/caspian.service sets TimeoutStopSec=20s, so this has to finish
// inside that or systemd sends SIGKILL in the middle of a teardown. Fifteen
// seconds leaves five for the process to exit after the teardown returns.
const shutdownGrace = 15 * time.Second

// servePrivileged runs the half that holds root.
func servePrivileged(ctx context.Context, log *slog.Logger) error {
	// ---------------------------------------------------------------------
	// Refuse early, and say what is wrong.
	// ---------------------------------------------------------------------
	if ok, who := runningPrivileged(); !ok {
		return fmt.Errorf("the privileged service has to run with the operating system's full privileges, "+
			"and this process is running as %s. It is started by %s, which does that; running it by hand "+
			"needs sudo or an elevated prompt", who, layout.ServiceManager)
	}

	plat, err := newPlatformPrivileged()
	if err != nil {
		// The refusal already says what this machine lacks. "caspian check"
		// runs the read-only half of the same code and is the thing to run
		// instead.
		return fmt.Errorf("%w. Use \"caspian check\" on this machine instead", err)
	}

	svc, err := privsvc.New(privsvc.Config{
		Runner:       plat.runner,
		Backend:      plat.backend,
		System:       plat.system,
		AccessPoint:  plat.ap,
		HotspotPaths: hotspotPaths(),
		TunName:      plat.tunName,
		Country:      plat.country,
		JournalPath:  journalPath,
		SocksPort:    socksPort,
		LocalDNSPort: localDNSPort,
		DNSPort:      dnsPort,
		PanelPort:    panelPort,
		Logger:       log,
	})
	if err != nil {
		return err
	}
	defer svc.Close()

	// ---------------------------------------------------------------------
	// Recovery, BEFORE the socket exists.
	//
	// A journal on disk means a previous process was killed between writing an
	// inverse and running the change, or after running it and before undoing
	// it. Replaying it is not tidying: everything this service does afterwards
	// plans against a machine it has just measured, and a leftover policy rule
	// or half default route makes that measurement a description of somebody
	// else's box. Doing it before the socket is opened means no caller can ask
	// for anything while the machine is in that state.
	// ---------------------------------------------------------------------
	rep, err := svc.ReplayJournal(ctx)
	if err != nil {
		// Not fatal, and the reasoning is worth writing down. An error here is
		// the journal FILE being unreadable or unwritable, not an inverse
		// failing; failed inverses stay in the journal and are retried. If the
		// service refused to start, the panel could never reach it to say what
		// was wrong, which is the failure docs/2026-08-29-design.md section 5.6
		// says to avoid above all others.
		log.Error("the teardown journal from a previous run could not be replayed; "+
			"the box may still be carrying changes from it",
			"journal", journalPath, "error", err.Error())
	} else if len(rep.Results) > 0 {
		log.Warn("put back changes left by a previous run that was killed",
			"inverses_run", len(rep.Results), "inverses_failed", rep.Failed)
	}

	// ---------------------------------------------------------------------
	// The socket.
	// ---------------------------------------------------------------------
	ln, err := privsvc.Listen(svc, privsvc.ListenConfig{
		Path:           socketPath,
		Group:          serviceGroup,
		ServiceAccount: serviceAccount,
		Logger:         log,
	})
	if err != nil {
		if errors.Is(err, privsvc.ErrAlreadyRunning) {
			return fmt.Errorf("cannot open the socket at %s because another copy of the privileged service "+
				"is already listening on it. Stop it first: %s", socketPath, layout.StopPrivilegedAdvice)
		}
		return err
	}

	log.Info("privileged service listening",
		"socket", ln.Addr(),
		"journal", journalPath,
		"tunnel_dns_port", localDNSPort,
		"panel_port", panelPort)

	serveErr := ln.Serve(ctx)

	// ---------------------------------------------------------------------
	// Shutdown.
	//
	// A clean stop returns the machine to how it was found, which is what
	// design section 5.5 promises. It is done here rather than left to the
	// next start's recovery because "left fail-closed until some later process
	// runs" is a worse state to leave a box in than "as we found it", and
	// because the journal is still the safety net if this does not finish:
	// systemd's KillMode takes hostapd and dnsmasq with the cgroup either way.
	// ---------------------------------------------------------------------
	stopCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := svc.Stop(stopCtx); err != nil {
		log.Error("the box could not be fully returned to how it was found; "+
			"the journal holds what is left and the next start will replay it",
			"journal", journalPath, "error", err.Error())
	}
	return serveErr
}
