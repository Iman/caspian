// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

type bpfPackets struct {
	mu      sync.Mutex
	fd      int
	buf     []byte
	pending []frame
}

func bpfSet(fd int, request uint, value unsafe.Pointer) error {
	_, _, err := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(request), uintptr(value))
	if err != 0 {
		return ErrUnavailable
	}
	return nil
}

func openPackets(_ netip.AddrPort, iface string) (packetIO, error) {
	if len(iface) == 0 || len(iface) >= unix.IFNAMSIZ {
		return nil, ErrUnavailable
	}
	fd := -1
	for i := 0; i < 256; i++ {
		candidate, err := unix.Open(fmt.Sprintf("/dev/bpf%d", i), unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err == nil {
			fd = candidate
			break
		}
	}
	if fd < 0 {
		return nil, ErrUnavailable
	}
	fail := func() (packetIO, error) { unix.Close(fd); return nil, ErrUnavailable }
	var ifr [32]byte
	copy(ifr[:unix.IFNAMSIZ], iface)
	if bpfSet(fd, unix.BIOCSETIF, unsafe.Pointer(&ifr[0])) != nil {
		return fail()
	}
	one := uint32(1)
	for _, op := range []uint{unix.BIOCIMMEDIATE, unix.BIOCSHDRCMPLT, unix.BIOCSSEESENT} {
		if bpfSet(fd, op, unsafe.Pointer(&one)) != nil {
			return fail()
		}
	}
	var linkType uint32
	if bpfSet(fd, unix.BIOCGDLT, unsafe.Pointer(&linkType)) != nil || linkType != unix.DLT_EN10MB {
		return fail()
	}
	var size uint32
	if bpfSet(fd, unix.BIOCGBLEN, unsafe.Pointer(&size)) != nil || size < 64 || size > 16<<20 {
		return fail()
	}
	return &bpfPackets{fd: fd, buf: make([]byte, size)}, nil
}

func (b *bpfPackets) Receive() (frame, error) {
	for {
		b.mu.Lock()
		if b.fd < 0 {
			b.mu.Unlock()
			return frame{}, ErrUnavailable
		}
		if len(b.pending) > 0 {
			f := b.pending[0]
			b.pending = b.pending[1:]
			b.mu.Unlock()
			return f, nil
		}
		poll := []unix.PollFd{{Fd: int32(b.fd), Events: unix.POLLIN}}
		n, err := unix.Poll(poll, 100)
		if err == unix.EINTR || n == 0 {
			b.mu.Unlock()
			continue
		}
		if err != nil {
			b.mu.Unlock()
			return frame{}, ErrUnavailable
		}
		n, err = unix.Read(b.fd, b.buf)
		if err == unix.EAGAIN || err == unix.EINTR {
			b.mu.Unlock()
			continue
		}
		if err != nil || n == 0 {
			b.mu.Unlock()
			return frame{}, ErrUnavailable
		}
		var header unix.BpfHdr
		capOffset := int(unsafe.Offsetof(header.Caplen))
		hdrOffset := int(unsafe.Offsetof(header.Hdrlen))
		for pos := 0; pos+hdrOffset+2 <= n; {
			hdrLen := int(binary.NativeEndian.Uint16(b.buf[pos+hdrOffset:]))
			capLen := int(binary.NativeEndian.Uint32(b.buf[pos+capOffset:]))
			if hdrLen < hdrOffset+2 || hdrLen > n-pos || capLen > n-pos-hdrLen {
				break
			}
			end := pos + hdrLen + capLen
			if f, ok := ethernetFrame(b.buf[pos+hdrLen : end]); ok {
				b.pending = append(b.pending, f)
			}
			pos = (end + 3) &^ 3
		}
		b.mu.Unlock()
	}
}

func (b *bpfPackets) Send(f frame) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fd < 0 || len(f.prefix) < 14 {
		return ErrUnavailable
	}
	raw := append(append([]byte(nil), f.prefix...), f.ip...)
	n, err := unix.Write(b.fd, raw)
	if err != nil || n != len(raw) {
		return ErrConfirmation
	}
	return nil
}

func (b *bpfPackets) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fd >= 0 {
		unix.Close(b.fd)
		b.fd = -1
	}
	return nil
}
