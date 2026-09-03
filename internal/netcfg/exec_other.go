// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux

package netcfg

import "runtime"

// NewSystemRunner returns a runner that refuses everything.
//
// Linux and macOS have runners of their own. Everywhere else the whole pure
// half of this package compiles and is tested; only execution is refused, and
// it refuses rather than silently doing nothing. A no-op runner would make an
// apply report success, which is the kind of false green this project has a
// rule against. Windows gets its runner with the Windows port.
func NewSystemRunner() Runner {
	return FailingRunner{Err: ErrUnsupportedPlatform}
}

// SystemBackend is the backend for the machine this binary runs on, which on
// this platform is one that refuses.
func SystemBackend() Backend { return BackendFor(Platform(runtime.GOOS)) }
