// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"fmt"
	"net"
	"strconv"

	"caspianbyoc.org/caspian/internal/xcfg"
)

// ---------------------------------------------------------------------------
// The loopback SOCKS port a run actually binds
//
// Config.SocksPort is the PREFERRED port, docs/LAYOUT.md's 10808. Until
// 2026-09-12 it was also the only port: the engine document named it, the
// macOS system proxy steps named it, and the subscription refresh dialled it.
// A Windows 11 report that day (GitHub issue #2) showed what that costs: another
// proxy client on the box already held 127.0.0.1:10808, and the engine refused
// to start with "bind: Only one usage of each socket address". The config was
// fine; the port was taken.
//
// So a start now asks the operating system, at the moment it decides, whether
// the preferred port is free, and moves to a free loopback port when it is not.
// The chosen port is decided ONCE per run, before the engine document and the
// network plan are composed, and every consumer reads that one value:
//
//   - engineDocument (plans.go) writes it into the SOCKS inbound;
//   - netOptionsFor (plans.go) writes it into the macOS SystemSOCKS options,
//     so the networksetup steps point the Mac at the port the engine bound;
//   - refreshClient (refresh.go) dials it;
//   - Status reports it as SystemStatus.LocalProxy, which the panel shows.
//
// What the probe proves and does not prove. Binding and releasing the port
// shows that nothing held it at that instant. Another program can take it in
// the window between the probe and the engine's own bind; when that happens the
// engine still fails to listen and engineFault (faults.go) still classifies it
// as panel.FaultPortInUse, exactly as before. The probe narrows that failure
// to a race; it does not remove the classification.
// ---------------------------------------------------------------------------

// chooseSocksPort returns the loopback port this run's SOCKS inbound will
// bind: preferred when it is free, otherwise a free port the operating system
// picks. moved reports which. The error is the one from net.Listen when not
// even an ephemeral loopback port could be bound, which is a box whose
// loopback interface is broken rather than one with a busy port.
func chooseSocksPort(preferred uint16) (port uint16, moved bool, err error) {
	if ln, lerr := net.Listen("tcp", localProxyAddr(preferred)); lerr == nil {
		_ = ln.Close()
		return preferred, false, nil
	}
	ln, err := net.Listen("tcp", localProxyAddr(0))
	if err != nil {
		return 0, false, fmt.Errorf("privsvc: no free loopback port for the local proxy: %w", err)
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 || addr.Port > 65535 {
		return 0, false, fmt.Errorf("privsvc: the loopback listener reported no usable port (%v)", ln.Addr())
	}
	return uint16(addr.Port), true, nil
}

// localProxyAddr renders the loopback SOCKS inbound as host:port, on the
// listen address internal/xcfg pins the inbound to. It is the one spelling of
// that address in this package, so the engine document, the refresh dialler
// and the panel cannot disagree about it.
func localProxyAddr(port uint16) string {
	return net.JoinHostPort(xcfg.DefaultSocksListen, strconv.Itoa(int(port)))
}

// socksPortInForce is the port this run's SOCKS inbound binds: the one
// chooseSocksPort picked at start, or the preferred port while no run has
// chosen one. The fallback keeps every path that never goes through Start,
// such as a refresh test that flips the engine phase directly, on the port it
// was configured with.
func (s *Service) socksPortInForce() uint16 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.socksPort != 0 {
		return s.socksPort
	}
	return s.cfg.SocksPort
}
