// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type divertPackets struct {
	handle                      uintptr
	recv, send, shutdown, close *windows.LazyProc
	once                        sync.Once
}

func openPackets(remote netip.AddrPort, _ string) (packetIO, error) {
	if runtime.GOARCH != "amd64" {
		return nil, ErrUnsupported
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, ErrUnavailable
	}
	// Never load a driver DLL from the current directory or a user's PATH.
	dll := windows.NewLazyDLL(filepath.Join(filepath.Dir(exe), "WinDivert.dll"))
	if err := dll.Load(); err != nil {
		return nil, ErrUnavailable
	}
	open := dll.NewProc("WinDivertOpen")
	d := &divertPackets{recv: dll.NewProc("WinDivertRecv"), send: dll.NewProc("WinDivertSend"), shutdown: dll.NewProc("WinDivertShutdown"), close: dll.NewProc("WinDivertClose")}
	for _, proc := range []*windows.LazyProc{open, d.recv, d.send, d.shutdown, d.close} {
		if err := proc.Find(); err != nil {
			return nil, ErrUnavailable
		}
	}
	filter, _ := windows.BytePtrFromString(fmt.Sprintf("ip and tcp and ((ip.DstAddr == %s and tcp.DstPort == %d) or (ip.SrcAddr == %s and tcp.SrcPort == %d))", remote.Addr(), remote.Port(), remote.Addr(), remote.Port()))
	// NETWORK=0, priority=0, SNIFF=1: ordinary traffic is never diverted.
	h, _, _ := open.Call(uintptr(unsafe.Pointer(filter)), 0, 0, 1)
	if h == ^uintptr(0) {
		return nil, ErrUnavailable
	}
	d.handle = h
	return d, nil
}

func (d *divertPackets) Receive() (frame, error) {
	f := frame{ip: make([]byte, 65535)}
	var n uint32
	ok, _, _ := d.recv.Call(d.handle, uintptr(unsafe.Pointer(&f.ip[0])), uintptr(len(f.ip)), uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&f.address[0])))
	if ok == 0 || n > uint32(len(f.ip)) {
		return frame{}, ErrUnavailable
	}
	f.ip = f.ip[:n]
	return f, nil
}

func (d *divertPackets) Send(f frame) error {
	if len(f.ip) == 0 {
		return ErrConfirmation
	}
	flags := binary.LittleEndian.Uint32(f.address[8:12])
	flags |= 1<<21 | 1<<22 // The generated IPv4 and TCP checksums are complete.
	binary.LittleEndian.PutUint32(f.address[8:12], flags)
	var n uint32
	ok, _, _ := d.send.Call(d.handle, uintptr(unsafe.Pointer(&f.ip[0])), uintptr(len(f.ip)), uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&f.address[0])))
	if ok == 0 || n != uint32(len(f.ip)) {
		return ErrConfirmation
	}
	return nil
}

func (d *divertPackets) Close() error {
	d.once.Do(func() { d.shutdown.Call(d.handle, 3); d.close.Call(d.handle) })
	return nil
}
