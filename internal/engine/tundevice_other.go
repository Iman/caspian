// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux

package engine

import "errors"

// On anything that is not Linux there is no TUN device to release, because
// xray-core's TUN inbound is only reached on a machine that has one and this
// appliance is Linux. The engine still builds and runs everywhere else, with
// its release path a no-op: captureTunHold records no device, because
// lookupLinkIndex fails for every name, and no descriptors, because
// tunDescriptors is empty.
//
// This file exists so that tundevice.go has no platform branch in it. The
// alternative, a runtime.GOOS check, would compile netlink into a macOS build
// that cannot use it.

var errNotLinux = errors.New("tunnel devices are only managed on linux")

func tunDescriptors() map[int]bool { return map[int]bool{} }

func lookupLinkIndex(string) (int, error) { return 0, errNotLinux }

func deleteLink(string) error { return errNotLinux }

func closeDescriptor(int) error { return nil }
