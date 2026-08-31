// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux

package netcfg

// NewSystemRunner returns a runner that refuses everything.
//
// Development happens on macOS and the target is Raspberry Pi OS arm64. The
// whole pure half of this package compiles and is tested on the development
// machine; only execution is Linux-only, so this file keeps the build green
// there without pretending that "ip" and "nft" exist.
//
// It refuses rather than silently doing nothing. A no-op runner would make an
// apply on a Mac report success, which is the kind of false green this project
// has a rule against.
func NewSystemRunner() Runner {
	return FailingRunner{Err: ErrUnsupportedPlatform}
}
