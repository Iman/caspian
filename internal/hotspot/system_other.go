// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux

package hotspot

import "errors"

// ErrUnsupportedPlatform is returned by NewSystemRunner off Linux.
//
// The pure half of this package (the two renderers, the lease parser, the
// validation) builds and runs everywhere, which is the point of the split: a
// developer on darwin can change what the appliance broadcasts and see the
// golden diff. Only the part that starts hostapd and dnsmasq is Linux.
var ErrUnsupportedPlatform = errors.New(
	"hotspot: the hotspot can only be run on Linux; on this machine the configuration " +
		"can be generated and checked but not started")

// NewSystemRunner reports that this platform cannot run the hotspot.
func NewSystemRunner() (System, error) {
	return nil, ErrUnsupportedPlatform
}
