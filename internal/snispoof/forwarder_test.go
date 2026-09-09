// SPDX-License-Identifier: AGPL-3.0-or-later
package snispoof

import (
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"
)

type silentPackets struct {
	closed chan struct{}
	once   sync.Once
}

func (p *silentPackets) Receive() (frame, error) { <-p.closed; return frame{}, io.EOF }
func (p *silentPackets) Send(frame) error        { return nil }
func (p *silentPackets) Close() error            { p.once.Do(func() { close(p.closed) }); return nil }
func TestForwarderNeverLeaksRealBytesWithoutConfirmation(t *testing.T) {
	for _, mode := range []string{"timeout", "capture failure", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			ln, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			packets := &silentPackets{closed: make(chan struct{})}
			f, err := startWithPackets(ln.Addr().(*net.TCPAddr).AddrPort(), netip.MustParseAddr("127.0.0.1"), "cover.example.invalid", packets)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			client, err := net.Dial("tcp4", f.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			client.SetDeadline(time.Now().Add(5 * time.Second))
			server, err := ln.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			server.SetDeadline(time.Now().Add(5 * time.Second))
			if _, err = client.Write([]byte("real confidential stream")); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "capture failure":
				packets.Close()
			case "shutdown":
				f.Close()
			}
			b := make([]byte, 100)
			n, err := server.Read(b)
			if n != 0 || err == nil {
				t.Fatalf("forwarded %d bytes without confirmation", n)
			}
			if e, ok := err.(net.Error); ok && e.Timeout() {
				t.Fatal("failed to close remote connection")
			}
		})
	}
}

type queuedPackets struct {
	silentPackets
	incoming chan frame
	sent     chan frame
}

func (p *queuedPackets) Receive() (frame, error) {
	select {
	case f := <-p.incoming:
		return f, nil
	case <-p.closed:
		return frame{}, io.EOF
	}
}
func (p *queuedPackets) Send(f frame) error { p.sent <- f; return nil }
func TestCaptureIgnoresUnownedConnectionsWithMatchingPorts(t *testing.T) {
	p := &queuedPackets{silentPackets: silentPackets{closed: make(chan struct{})}, incoming: make(chan frame, 8), sent: make(chan frame, 8)}
	f, err := startWithPackets(netip.MustParseAddrPort("127.0.0.2:443"), netip.MustParseAddr("127.0.0.1"), "cover.example.invalid", p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	state := handshake(t, 10)
	state.scheduled = false
	f.mu.Lock()
	f.flows[flowKey{local: netip.MustParseAddrPort("127.0.0.1:50000"), remote: f.remote}] = state
	f.mu.Unlock()
	for _, which := range []string{"local address", "local port", "remote address", "remote port"} {
		b := rawPacket(11, 51, ack)
		switch which {
		case "local address":
			b[15] = 3
		case "local port":
			binary.BigEndian.PutUint16(b[20:22], 50001)
		case "remote address":
			b[19] = 4
		case "remote port":
			binary.BigEndian.PutUint16(b[22:24], 444)
		}
		p.incoming <- frame{ip: b}
	}
	// The correct packet serves as an ordered barrier after all unrelated frames.
	p.incoming <- frame{ip: rawPacket(11, 51, ack)}
	select {
	case <-p.sent:
	case <-time.After(time.Second):
		t.Fatal("owned flow was not injected")
	}
	select {
	case <-p.sent:
		t.Fatal("injected an unrelated connection")
	default:
	}
}
func TestTransportCompatibility(t *testing.T) {
	for _, network := range []string{"tcp", "raw", "websocket", "ws", "httpupgrade", "grpc"} {
		if !SupportsTransport("vless", network) {
			t.Fatal("rejected TCP transport", network)
		}
	}
	for _, network := range []string{"quic", "kcp", "splithttp", "xhttp", "unknown"} {
		if SupportsTransport("vless", network) {
			t.Fatal("accepted unsupported transport", network)
		}
	}
	if SupportsTransport("hysteria2", "") || SupportsTransport("socks", "tcp") || SupportsTransport("shadowsocks", "tcp") || SupportsTransport("freedom", "tcp") {
		t.Fatal("accepted QUIC protocol")
	}
}
func TestEthernetFrameRejectsMalformedTagsAndPreservesHeaders(t *testing.T) {
	for _, tags := range []int{0, 1, 2, 3} {
		b := make([]byte, 14+tags*4)
		binary.BigEndian.PutUint16(b[12:14], 0x0800)
		for i := 0; i < tags; i++ {
			binary.BigEndian.PutUint16(b[12+i*4:14+i*4], 0x8100)
			binary.BigEndian.PutUint16(b[16+i*4:18+i*4], 0x0800)
		}
		b = append(b, rawPacket(1, 2, ack)...)
		f, ok := ethernetFrame(b)
		if ok != (tags <= 2) {
			t.Fatal("wrong VLAN acceptance", tags)
		}
		if ok && len(f.prefix) != 14+tags*4 {
			t.Fatal("wrong Ethernet prefix")
		}
	}
	for _, b := range [][]byte{nil, make([]byte, 13), make([]byte, 14), append(make([]byte, 12), 0x81, 0x00, 0)} {
		if _, ok := ethernetFrame(b); ok {
			t.Fatal("accepted malformed Ethernet frame")
		}
	}
}
