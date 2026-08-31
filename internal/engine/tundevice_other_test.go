// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux

package engine

import "testing"

// TestTunDeviceLifecycleIsNotBuiltOnThisPlatform exists to make a developer
// machine SAY that it proved nothing about the tunnel device.
//
// tundevice_linux_test.go carries a linux build tag, so on macOS it is not
// compiled and its tests do not appear in the run at all. A suite that passes
// in silence reads like a pass. This turns that silence into a line naming
// what was not tested and where it is tested instead.
func TestTunDeviceLifecycleIsNotBuiltOnThisPlatform(t *testing.T) {
	t.Skip("SKIPPED and PROVES NOTHING about the TUN device: tundevice_linux_test.go is built only on linux, and its tests need root and /dev/net/tun. The device lifecycle is proven on the appliance, nowhere else.")
}
