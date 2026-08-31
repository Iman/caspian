// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package engine

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// The four kernel operations tundevice.go needs, and nothing else lives here.
//
// netlink rather than an "ip" subprocess: this package runs no commands, and
// the device being removed was created in this process by code in this binary.
// A netlink call is in-process, has an error that can be tested, and does not
// depend on ip being on the path or on a locale. Running commands against the
// machine is internal/netcfg's job, and this is not that: netcfg never created
// this device and has no inverse recorded for it.

// tunDescriptors returns the descriptors this process has open on
// /dev/net/tun.
//
// /proc is the only place that answers this. A descriptor whose link cannot be
// read is skipped rather than guessed at, so the result can undercount but
// never name a descriptor that is not a tunnel.
func tunDescriptors() map[int]bool {
	out := map[int]bool{}
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return out
	}
	for _, e := range ents {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
		if err != nil || target != "/dev/net/tun" {
			continue
		}
		fd, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		out[fd] = true
	}
	return out
}

// lookupLinkIndex returns the interface index of a device by name. The error
// means "no such device" for every caller here, which is why none of them
// inspects it.
func lookupLinkIndex(name string) (int, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return 0, err
	}
	return link.Attrs().Index, nil
}

// deleteLink removes a device by name.
func deleteLink(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	return netlink.LinkDel(link)
}

// closeDescriptor closes one descriptor.
func closeDescriptor(fd int) error { return unix.Close(fd) }
