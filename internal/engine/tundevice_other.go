// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux

package engine

import "errors"

// On anything that is not Linux this package releases no TUN device itself.
// Since xray-core v26.4.15 the engine's own Close releases it on every
// platform (proxy/tun.Handler.Close closes the stack and the device, reached
// from the inbound manager; TestTheTunInboundReleasesItselfOnClose pins
// both halves), and the Linux release path in tundevice_linux.go is kept only
// as the measured safety net for the appliance. Here the path is a no-op:
// captureTunHold records no device, because lookupLinkIndex fails for every
// name, and no descriptors, because tunDescriptors is empty.
//
// This file exists so that tundevice.go has no platform branch in it. The
// alternative, a runtime.GOOS check, would compile netlink into a macOS build
// that cannot use it.

var errNotLinux = errors.New("tunnel devices are only managed on linux")

func tunDescriptors() map[int]bool { return map[int]bool{} }

func lookupLinkIndex(string) (int, error) { return 0, errNotLinux }

func deleteLink(string) error { return errNotLinux }

func closeDescriptor(int) error { return nil }
