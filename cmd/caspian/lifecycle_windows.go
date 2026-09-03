// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows

package main

import (
	"context"
	"os"
	"os/signal"

	"golang.org/x/sys/windows/svc"
)

// serviceContext returns the context a role runs under.
//
// A Windows service receives no signal from the Service Control Manager. It
// receives SERVICE_CONTROL_STOP and SHUTDOWN through a handler registered with
// svc.Run, so when this process was started by the SCM the handler cancels
// the context and the role's own shutdown code runs unchanged. Started from a
// console it behaves as on unix: Ctrl-C cancels.
func serviceContext(role string) (context.Context, context.CancelFunc) {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return signal.NotifyContext(context.Background(), os.Interrupt)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &scmHandler{cancel: cancel, done: make(chan struct{})}
	go func() {
		// svc.Run blocks until the handler's Execute returns, which it does
		// once the role has exited (Wait is called from main via cancel).
		_ = svc.Run(role, h)
	}()
	return ctx, func() {
		cancel()
		close(h.done)
	}
}

type scmHandler struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func (h *scmHandler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.Running, Accepts: accepted}
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending}
				h.cancel()
				<-h.done
				s <- svc.Status{State: svc.Stopped}
				return false, 0
			}
		case <-h.done:
			s <- svc.Status{State: svc.Stopped}
			return false, 0
		}
	}
}
