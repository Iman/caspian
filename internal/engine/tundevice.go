// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"errors"
	"fmt"
	"time"

	xtun "github.com/xtls/xray-core/proxy/tun"
)

// The TUN device the engine creates, and why this package has to remove it.
//
// # What xray-core does and does not do
//
// core.New creates the TUN device, not Start: core/xray.go builds the inbound
// handlers inside initInstanceWithConfig, and proxy/tun's Handler.Init opens
// /dev/net/tun, brings the link up and hands the descriptor to a gVisor
// netstack. Nothing closes any of that again. The Handler keeps no reference
// to the device it made, it has NO Close method (TestTheTunInboundStillHasNoCloseMethod),
// and the inbound handler that owns it closes only its workers and its mux.
// The TUN inbound has no workers, because infra/conf/xray.go skips the port
// branch for protocol "tun" and leaves ReceiverConfig.PortList nil.
//
// So core.Instance.Close leaves behind: the device, up; one descriptor on
// /dev/net/tun; and a gVisor dispatch goroutine blocked reading that
// descriptor. Measured on the appliance on 2026-08-30.
//
// # Why that is not cosmetic
//
// The device is not persistent, so it can only be recreated when the last
// reference to it goes. The next Start therefore fails at TUNSETIFF with
// EBUSY, and the appliance cannot be switched on again until the process
// restarts. That happened three times on the appliance on 2026-08-30 between
// 13:02:48 and 13:02:54, each one logged as
// "the engine would not start" detail="start: device or resource busy".
//
// # Why the device is deleted rather than only the descriptor closed
//
// Closing the descriptor alone does work eventually, but not predictably:
// gVisor's dispatch goroutine sits in a ppoll with a NULL timeout
// (gvisor pkg/rawfile BlockingPollUntilStopped), and closing a descriptor does
// not wake a blocked poll, so the file description stays alive until that poll
// happens to return. Measured on the appliance over three runs: 4.445s, 5.2ms,
// 3.825s. A Stop that cannot say whether the device is gone is no use to a
// panel whose next action is Start.
//
// Deleting the link is deterministic. The kernel detaches the queue and wakes
// every waiter, so the device is gone at once and the gVisor goroutines exit
// by themselves. Measured on the appliance: gone 24.9us after the delete
// returned, with no gVisor dispatch goroutine left.

// tunDevice is one TUN device the engine asked xray-core to create.
//
// The index is what makes deleting it safe. The name comes from a
// configuration field the user can set, and deleting an interface because it
// has the name you expected is how you delete somebody else's interface.
type tunDevice struct {
	name  string
	index int
}

// tunHold is everything the engine has to give back that core.New took: the
// devices it created and the descriptors it opened for them.
//
// The zero value releases nothing, which is what a config with no TUN inbound
// should do.
type tunHold struct {
	devices []tunDevice
	fds     []int
}

// tunReleaseGrace bounds the wait for the kernel to remove a deleted device.
// The measured time is microseconds; this is a bound on being wrong, not an
// expected duration.
const tunReleaseGrace = 2 * time.Second

// tunInboundNames returns the device name of every TUN inbound in cfg.
//
// The name is already defaulted by the time it gets here: infra/conf/tun.go
// substitutes "xray0" for an empty name when it builds the protobuf, which
// TestTheLoaderDefaultsTheTunDeviceName pins. An empty name reaching this
// point would mean that stopped being true, and it is skipped rather than
// guessed at, because a wrong name here deletes the wrong interface.
func tunInboundNames(cfg *coreConfig) []string {
	if cfg == nil {
		return nil
	}
	var names []string
	for _, in := range cfg.Inbound {
		if in == nil || in.ProxySettings == nil {
			continue
		}
		settings, err := in.ProxySettings.GetInstance()
		if err != nil {
			continue
		}
		tc, ok := settings.(*xtun.Config)
		if !ok {
			continue
		}
		if tc.Name == "" {
			continue
		}
		names = append(names, tc.Name)
	}
	return names
}

// captureTunHold records what core.New just created.
//
// before is the set of descriptors this process had open on /dev/net/tun
// before core.New was called, so the difference is what the engine opened.
// That difference is attributed to the engine and to nothing else, which holds
// because this package documents one engine per process and nothing else in
// this repository opens /dev/net/tun.
func captureTunHold(cfg *coreConfig, before map[int]bool) *tunHold {
	h := &tunHold{}
	for _, name := range tunInboundNames(cfg) {
		index, err := lookupLinkIndex(name)
		if err != nil {
			// The device is not there, so either this platform does not make
			// one or it failed before it was named. Recording it with no index
			// would be recording a licence to delete by name alone.
			continue
		}
		h.devices = append(h.devices, tunDevice{name: name, index: index})
	}
	for fd := range tunDescriptors() {
		if !before[fd] {
			h.fds = append(h.fds, fd)
		}
	}
	return h
}

// release deletes the devices this engine created and closes the descriptors
// it opened for them.
//
// It keeps going past every failure, for the same reason internal/netcfg's
// Teardown does: one step failing says nothing about the rest, and stopping
// would leave the remainder in place.
func (h *tunHold) release() error {
	if h == nil {
		return nil
	}
	var errs []error
	for _, d := range h.devices {
		if err := removeTunDevice(d); err != nil {
			errs = append(errs, err)
		}
	}
	// The descriptors are closed after the devices are gone, and never before:
	// while the device exists, gVisor's dispatch goroutine is still reading
	// this descriptor, and closing a descriptor out from under a reader is how
	// a descriptor number gets reused while somebody is still using it.
	for _, fd := range h.fds {
		if err := closeDescriptor(fd); err != nil {
			errs = append(errs, fmt.Errorf("closing the tunnel descriptor: %w", err))
		}
	}
	h.devices, h.fds = nil, nil
	return errors.Join(errs...)
}

// removeTunDevice deletes one device, but only if it is still the one this
// engine created.
func removeTunDevice(d tunDevice) error {
	index, err := lookupLinkIndex(d.name)
	if err != nil {
		// Already gone. Nothing to do and nothing to report: a device removed
		// by something else is the state this function wanted.
		return nil
	}
	if index != d.index {
		// Same name, different interface. Somebody re-created it, or the name
		// was reused. Deleting it would be deleting somebody else's device.
		return fmt.Errorf("the tunnel device %s is now interface %d and not the %d this engine created, so it was left alone", d.name, index, d.index)
	}
	if err := deleteLink(d.name); err != nil {
		return fmt.Errorf("deleting the tunnel device %s: %w", d.name, err)
	}
	deadline := time.Now().Add(tunReleaseGrace)
	for time.Now().Before(deadline) {
		if _, err := lookupLinkIndex(d.name); err != nil {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("the tunnel device %s is still present %v after it was deleted", d.name, tunReleaseGrace)
}
