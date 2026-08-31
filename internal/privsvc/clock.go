// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import "time"

// DefaultClockFloor is the earliest wall-clock time this build will attempt a
// connection at.
//
// docs/2026-08-29-design.md section 9 gives two independent reasons, and the
// second is the one that makes this a check rather than a nicety:
//
//   - REALITY writes the wall clock into the handshake and some servers check
//     it, and a Raspberry Pi has no battery clock, so a box that comes up with
//     a clock taken from a file timestamp fails authentication and the panel
//     blames the config;
//   - WHICH CONFIGS THE ENGINE ACCEPTS DEPENDS ON THE DATE. xray-core
//     v1.260327.0 removes the allowInsecure option gated on
//     time.Now().After(2026-06-01) (infra/conf/transport_internet.go:709-716),
//     so the same binary accepts a config before that date and refuses it
//     after. Config validity is not a property of the config alone.
//
// The value is the date this package was written. A clock behind it is a clock
// that was never set: the software did not exist then. It is deliberately not
// derived from the build timestamp, because that is not reproducible and a
// reproducible build is a property this project keeps
// (docs/LAYOUT.md, release artefacts and checksums).
//
// There is no upper bound. A clock in the future is not a failure mode either
// mechanism above has: REALITY's skew check refuses it at the server, which is
// a server answer and not a local refusal to try, and the engine's gate only
// ever moves from "accepted" to "refused" as time passes.
var DefaultClockFloor = time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)

// clockPlausible reports whether now is late enough to attempt a connection.
func clockPlausible(now, floor time.Time) bool { return !now.Before(floor) }
