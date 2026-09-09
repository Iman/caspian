// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//go:build !windows && !linux && !darwin

package snispoof

import "net/netip"

func openPackets(netip.AddrPort, string) (packetIO, error) { return nil, ErrUnsupported }
