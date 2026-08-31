// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build !linux && !darwin

package privsvc

import "net"

// peerCredential refuses on a platform where the account on the other end
// cannot be established.
//
// It refuses rather than returning a permissive answer. A helper that runs as
// root and answers anybody it cannot identify is not a privilege boundary, and
// this project has a standing rule against a stub that reports success while
// doing nothing (internal/netcfg/exec_other.go makes the same choice for the
// same reason).
func peerCredential(*net.UnixConn) (Peer, error) { return Peer{}, ErrPeerUnknown }
