// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import (
	"context"
	"io"
	"net"
	"net/netip"
	"sync"
	"time"
)

const maxConnections = 128

// Forwarder owns a loopback listener, packet capture, and its remote connections.
type Forwarder struct {
	remote   netip.AddrPort
	local    netip.Addr
	name     string
	options  Options
	packets  packetIO
	listener net.Listener
	ctx      context.Context
	cancel   context.CancelFunc
	once     sync.Once
	wg       sync.WaitGroup
	mu       sync.Mutex
	flows    map[flowKey]*flow
	clients  map[net.Conn]struct{}
	limit    chan struct{}
}

// Start opens a loopback-only forwarder. The caller owns Close, including on
// startup rollback. Only IPv4 is supported by the reference packet algorithm.
func Start(remote netip.AddrPort, iface, name string) (*Forwarder, error) {
	normalized, err := NormalizeName(name)
	if err != nil || normalized == "" {
		return nil, ErrName
	}
	return StartWithOptions(remote, iface, Options{FakeSNI: name})
}

func StartWithOptions(remote netip.AddrPort, iface string, options Options) (*Forwarder, error) {
	name, err := NormalizeName(options.FakeSNI)
	if err != nil {
		return nil, ErrName
	}
	if !remote.Addr().Is4() || remote.Port() == 0 {
		return nil, ErrUnsupported
	}
	// This UDP connect selects a source address without sending a datagram.
	c, err := net.DialUDP("udp4", nil, net.UDPAddrFromAddrPort(remote))
	if err != nil {
		return nil, ErrUnavailable
	}
	local := c.LocalAddr().(*net.UDPAddr).AddrPort().Addr().Unmap()
	c.Close()
	var packets packetIO
	if name != "" {
		packets, err = openPackets(remote, iface)
	}
	if err != nil {
		return nil, err
	}
	options.FakeSNI = name
	f, err := startWithOptions(remote, local, options, packets)
	if err != nil && packets != nil {
		packets.Close()
	}
	return f, err
}

func startWithPackets(remote netip.AddrPort, local netip.Addr, name string, packets packetIO) (*Forwarder, error) {
	return startWithOptions(remote, local, Options{FakeSNI: name}, packets)
}

func startWithOptions(remote netip.AddrPort, local netip.Addr, options Options, packets packetIO) (*Forwarder, error) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &Forwarder{remote: remote, local: local, name: options.FakeSNI, options: options, packets: packets, listener: ln, ctx: ctx, cancel: cancel,
		flows: make(map[flowKey]*flow), clients: make(map[net.Conn]struct{}), limit: make(chan struct{}, maxConnections)}
	f.wg.Add(1)
	if packets != nil {
		f.wg.Add(1)
		go f.capture()
	}
	go f.accept()
	return f, nil
}

func (f *Forwarder) Addr() netip.AddrPort { return f.listener.Addr().(*net.TCPAddr).AddrPort() }

func (f *Forwarder) shutdown() {
	f.once.Do(func() {
		f.cancel()
		f.listener.Close()
		if f.packets != nil {
			f.packets.Close()
		}
		f.mu.Lock()
		for client := range f.clients {
			client.Close()
		}
		f.mu.Unlock()
	})
}

func (f *Forwarder) Close() error { f.shutdown(); f.wg.Wait(); return nil }

func (f *Forwarder) accept() {
	defer f.wg.Done()
	for {
		client, err := f.listener.Accept()
		if err != nil {
			f.shutdown()
			return
		}
		select {
		case f.limit <- struct{}{}:
		default:
			client.Close()
			continue
		}
		f.mu.Lock()
		if f.ctx.Err() != nil {
			f.mu.Unlock()
			client.Close()
			<-f.limit
			return
		}
		f.clients[client] = struct{}{}
		f.mu.Unlock()
		f.wg.Add(1)
		go f.handle(client)
	}
}

func (f *Forwarder) handle(client net.Conn) {
	defer f.wg.Done()
	defer func() { client.Close(); f.mu.Lock(); delete(f.clients, client); f.mu.Unlock(); <-f.limit }()
	server, err := f.connect()
	if err != nil {
		return
	}
	defer server.Close()
	stopClose := context.AfterFunc(f.ctx, func() { server.Close() })
	defer stopClose()
	if f.options.TCPSplit || f.options.TLSRecordSplit {
		client.SetReadDeadline(time.Now().Add(5 * time.Second))
		server.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := forwardHello(server, client, f.options); err != nil {
			return
		}
		client.SetReadDeadline(time.Time{})
		server.SetWriteDeadline(time.Time{})
	}
	// Both streams remain untouched until the packet-level confirmation.
	done := make(chan struct{}, 2)
	copyStream := func(dst, src net.Conn) {
		_, err := io.Copy(dst, src)
		if err != nil {
			client.Close()
			server.Close()
		} else if tcp, ok := dst.(interface{ CloseWrite() error }); ok {
			tcp.CloseWrite()
		}
		done <- struct{}{}
	}
	go copyStream(server, client)
	go copyStream(client, server)
	<-done
	<-done
}

func (f *Forwarder) capture() {
	defer f.wg.Done()
	for {
		frame, err := f.packets.Receive()
		if err != nil {
			f.shutdown()
			return
		}
		p, ok := parsePacket(frame.ip)
		if !ok {
			continue
		}
		outbound := p.dst == f.remote
		key := flowKey{local: p.src, remote: p.dst}
		if !outbound {
			key = flowKey{local: p.dst, remote: p.src}
		}
		f.mu.Lock()
		state := f.flows[key]
		f.mu.Unlock()
		if state == nil {
			continue
		}
		state.mu.Lock()
		inject := state.observe(p, outbound)
		state.mu.Unlock()
		if inject {
			f.wg.Add(1)
			go f.inject(frame, p, state)
		}
	}
}

func (f *Forwarder) inject(frame frame, p packet, state *flow) {
	defer f.wg.Done()
	timer := time.NewTimer(time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-f.ctx.Done():
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.finished || f.ctx.Err() != nil {
		return
	}
	if err := f.packets.Send(fakeFrame(frame, p, state.isn, state.hello)); err != nil {
		state.finish(ErrConfirmation)
		return
	}
	state.sent = true
}

// SupportsTransport rejects transports which can establish UDP/QUIC or a
// separate download connection outside this one TCP forwarder.
func SupportsTransport(protocol, network string) bool {
	// These protocols carry application UDP inside the spoofed TCP stream.
	// Native UDP protocols and SOCKS UDP association need a different backend.
	switch protocol {
	case "vless", "vmess", "trojan":
	default:
		return false
	}
	switch network {
	case "", "tcp", "raw", "ws", "websocket", "httpupgrade", "grpc":
		return true
	}
	return false
}

func (f *Forwarder) connect() (net.Conn, error) {
	if f.name == "" {
		d := net.Dialer{Timeout: 5 * time.Second, LocalAddr: &net.TCPAddr{IP: net.IP(f.local.AsSlice())}}
		return d.DialContext(f.ctx, "tcp4", f.remote.String())
	}
	hello, err := clientHello(f.name)
	if err != nil {
		return nil, ErrConfirmation
	}
	state := &flow{hello: hello, done: make(chan struct{})}
	var key flowKey
	defer func() {
		state.mu.Lock()
		state.finish(ErrConfirmation)
		state.mu.Unlock()
		f.mu.Lock()
		delete(f.flows, key)
		f.mu.Unlock()
	}()
	server, err := dialOwned(f.ctx, f.local, f.remote, func(local netip.AddrPort) error {
		key = flowKey{local: local, remote: f.remote}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.ctx.Err() != nil {
			return f.ctx.Err()
		}
		f.flows[key] = state
		return nil
	})
	if err != nil {
		return nil, ErrConfirmation
	}
	ok := false
	defer func() {
		if !ok {
			server.Close()
		}
	}()
	stopClose := context.AfterFunc(f.ctx, func() { server.Close() })
	defer stopClose()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-state.done:
		state.mu.Lock()
		err = state.err
		state.mu.Unlock()
		if err != nil {
			return nil, ErrConfirmation
		}
	case <-timer.C:
		return nil, ErrConfirmation
	case <-f.ctx.Done():
		return nil, ErrConfirmation
	}
	f.mu.Lock()
	delete(f.flows, key)
	f.mu.Unlock()
	ok = true
	return server, nil
}
