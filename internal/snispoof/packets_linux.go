// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import (
	"encoding/binary"
	"net"
	"net/netip"
	"sync"

	"golang.org/x/sys/unix"
)

type rawPackets struct {
	mu        sync.Mutex
	fd, index int
}

func networkOrder(v uint16) int {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	return int(binary.NativeEndian.Uint16(b[:]))
}

func openPackets(_ netip.AddrPort, iface string) (packetIO, error) {
	device, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, ErrUnavailable
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, networkOrder(unix.ETH_P_IP))
	if err != nil {
		return nil, ErrUnavailable
	}
	if err = unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: device.Index, Protocol: uint16(networkOrder(unix.ETH_P_IP))}); err != nil {
		unix.Close(fd)
		return nil, ErrUnavailable
	}
	return &rawPackets{fd: fd, index: device.Index}, nil
}

func (r *rawPackets) Receive() (frame, error) {
	buf := make([]byte, 65535+64)
	for {
		r.mu.Lock()
		if r.fd < 0 {
			r.mu.Unlock()
			return frame{}, ErrUnavailable
		}
		poll := []unix.PollFd{{Fd: int32(r.fd), Events: unix.POLLIN}}
		n, err := unix.Poll(poll, 100)
		if err == unix.EINTR || n == 0 {
			r.mu.Unlock()
			continue
		}
		if err != nil {
			r.mu.Unlock()
			return frame{}, ErrUnavailable
		}
		n, _, err = unix.Recvfrom(r.fd, buf, 0)
		r.mu.Unlock()
		if err == unix.EAGAIN || err == unix.EINTR {
			continue
		}
		if err != nil {
			return frame{}, ErrUnavailable
		}
		if f, ok := ethernetFrame(buf[:n]); ok {
			return f, nil
		}
	}
}

func (r *rawPackets) Send(f frame) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fd < 0 || len(f.prefix) < 14 {
		return ErrUnavailable
	}
	frame := append(append([]byte(nil), f.prefix...), f.ip...)
	sa := &unix.SockaddrLinklayer{Ifindex: r.index, Protocol: uint16(networkOrder(unix.ETH_P_IP)), Halen: 6}
	copy(sa.Addr[:], frame[:6])
	if err := unix.Sendto(r.fd, frame, 0, sa); err != nil {
		return ErrConfirmation
	}
	return nil
}

func (r *rawPackets) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fd >= 0 {
		unix.Close(r.fd)
		r.fd = -1
	}
	return nil
}
