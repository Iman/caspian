// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package hotspot

import "context"

// AccessPoint is what the privileged service holds instead of a concrete
// Supervisor: something that can bring a rendered Plan on the air, take it
// down again, and say what is running.
//
// On Linux it is the Supervisor over hostapd and dnsmasq. On macOS it is
// Apple's Internet Sharing driven through its preferences; on Windows it is
// Mobile Hotspot. Each is fed the same validated Plan and answers in the same
// Status vocabulary, which is what lets internal/privsvc and the panel stay
// unchanged across the three.
//
// The contract is the Supervisor's. Start is idempotent: calling it with the
// plan that is already on the air restarts nothing, because a restart
// disconnects every joined device. Stop removes what Start wrote and puts back
// what it changed. Status reports without changing anything.
type AccessPoint interface {
	Start(ctx context.Context, plan Plan) (Status, error)
	Stop(ctx context.Context) error
	Status(ctx context.Context, iface string) (Status, error)
}

var _ AccessPoint = (*Supervisor)(nil)
