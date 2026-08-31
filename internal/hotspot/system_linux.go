// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package hotspot

// NewSystemRunner returns the System the appliance runs on.
func NewSystemRunner() (System, error) {
	return NewExecSystem(), nil
}
