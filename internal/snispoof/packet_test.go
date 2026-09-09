// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestNames(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		bad      bool
	}{
		{"", "", false}, {" Example.COM. ", "example.com", false}, {"example.test", "example.test", false},
		{"https://example.test", "", true}, {"127.0.0.1", "", true}, {"a..test", "", true}, {"-a.test", "", true}, {"a_.test", "", true},
		{strings.Repeat("a", 64) + ".test", "", true}, {strings.Repeat("a.", 110) + "test", "", true}, {"a\n.test", "", true},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, err := NormalizeName(tc.in)
			if (err != nil) != tc.bad || got != tc.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestClientHelloMatchesMasterLayoutAndTLSParser(t *testing.T) {
	for _, name := range []string{"example.test", strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 27)} {
		hello, err := clientHello(name)
		if err != nil {
			t.Fatal(err)
		}
		if len(hello) != 517 || binary.BigEndian.Uint16(hello[3:5]) != 512 || string(hello[127:127+len(name)]) != name {
			t.Fatal("master layout changed")
		}
		client, server := net.Pipe()
		server.SetDeadline(time.Now().Add(time.Second))
		client.SetDeadline(time.Now().Add(time.Second))
		seen := make(chan string, 1)
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer server.Close()
			s := tls.Server(server, &tls.Config{GetConfigForClient: func(h *tls.ClientHelloInfo) (*tls.Config, error) {
				seen <- h.ServerName
				return nil, errors.New("inspection complete")
			}})
			s.Handshake()
		}()
		client.Write(hello)
		client.Close()
		<-done
		select {
		case got := <-seen:
			if got != name {
				t.Fatal("wrong TLS SNI")
			}
		default:
			t.Fatal("TLS parser did not accept the ClientHello")
		}
	}
	if _, err := clientHello(""); !errors.Is(err, ErrName) {
		t.Fatal(err)
	}
}

func handshake(t *testing.T, isn uint32) *flow {
	t.Helper()
	f := &flow{done: make(chan struct{}), hello: make([]byte, 517)}
	f.observe(packet{seq: isn, flags: syn}, true)
	f.observe(packet{seq: 50, ack: isn + 1, flags: syn | ack}, false)
	if !f.observe(packet{seq: isn + 1, ack: 51, flags: ack}, true) {
		t.Fatal("missing injection")
	}
	if f.observe(packet{seq: isn + 1, ack: 51, flags: ack}, true) {
		t.Fatal("duplicate injection")
	}
	return f
}

func TestConfirmationRequiresBothSequencesAfterSend(t *testing.T) {
	for _, isn := range []uint32{0, 1000, ^uint32(0)} {
		f := handshake(t, isn)
		f.observe(packet{seq: 51, ack: isn + 1, flags: ack}, false)
		if f.finished {
			t.Fatal("confirmed before send")
		}
		f.sent = true
		f.observe(packet{seq: 51, ack: isn + 1, flags: ack}, false)
		if !f.finished || f.err != nil {
			t.Fatal("valid confirmation failed")
		}
	}
	for _, p := range []packet{{seq: 52, ack: 11, flags: ack}, {seq: 51, ack: 12, flags: ack}, {seq: 51, ack: 11, flags: rst | ack}, {seq: 51, ack: 11, flags: ack, payloadLen: 1}} {
		f := handshake(t, 10)
		f.sent = true
		f.observe(p, false)
		if !f.finished || !errors.Is(f.err, ErrConfirmation) {
			t.Fatal("accepted invalid confirmation")
		}
	}
}

func TestRetransmittedSYNPreservesState(t *testing.T) {
	f := handshake(t, 10)
	f.observe(packet{seq: 10, flags: syn}, true)
	if f.finished || !f.scheduled {
		t.Fatal("retransmission reset state")
	}
	f.observe(packet{seq: 11, flags: syn}, true)
	if !errors.Is(f.err, ErrConfirmation) {
		t.Fatal("changed SYN sequence accepted")
	}
}

func rawPacket(seq, acknum uint32, flags byte) []byte {
	b := make([]byte, 40)
	b[0] = 0x45
	b[8] = 64
	b[9] = 6
	b[12] = 127
	b[15] = 1
	b[16] = 127
	b[19] = 2
	binary.BigEndian.PutUint16(b[2:4], 40)
	binary.BigEndian.PutUint16(b[20:22], 50000)
	binary.BigEndian.PutUint16(b[22:24], 443)
	binary.BigEndian.PutUint32(b[24:28], seq)
	binary.BigEndian.PutUint32(b[28:32], acknum)
	b[32] = 0x50
	b[33] = flags
	return b
}

func TestFakePacketPreservesHeadersAndHasValidChecksums(t *testing.T) {
	raw := rawPacket(1, 51, ack)
	original := bytes.Clone(raw)
	p, ok := parsePacket(raw)
	if !ok {
		t.Fatal("invalid fixture")
	}
	fake := fakeFrame(frame{ip: raw}, p, 0, make([]byte, 517)).ip
	if !bytes.Equal(raw, original) {
		t.Fatal("mutated captured packet")
	}
	if checksum(fake[:20], 0) != 0 {
		t.Fatal("bad IP checksum")
	}
	pseudo := append(bytes.Clone(fake[12:20]), 0, 6)
	pseudo = binary.BigEndian.AppendUint16(pseudo, uint16(len(fake)-20))
	if checksum(append(pseudo, fake[20:]...), 0) != 0 {
		t.Fatal("bad TCP checksum")
	}
	got, ok := parsePacket(fake)
	if !ok || got.seq != ^uint32(515) || got.ack != 51 || got.payloadLen != 517 {
		t.Fatalf("bad fake: %+v", got)
	}
}

func TestMalformedHeadersAreIgnored(t *testing.T) {
	for n := 0; n < 40; n++ {
		if _, ok := parsePacket(make([]byte, n)); ok {
			t.Fatal(n)
		}
	}
	for _, mutate := range []func([]byte){func(b []byte) { b[0] = 0x4f }, func(b []byte) { b[32] = 0xf0 }, func(b []byte) { b[2] = 255 }, func(b []byte) { b[6] = 0x20 }, func(b []byte) { b[0] = 0x44 }, func(b []byte) { b[32] = 0x40 }} {
		b := rawPacket(1, 2, ack)
		mutate(b)
		if _, ok := parsePacket(b); ok {
			t.Fatal("malformed packet accepted")
		}
	}
}

func FuzzPacketParser(f *testing.F) {
	f.Add(rawPacket(1, 2, ack))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		if p, ok := parsePacket(b); ok {
			out := fakeFrame(frame{ip: b}, p, 0, make([]byte, 517))
			if _, valid := parsePacket(out.ip); !valid {
				t.Fatal("generated invalid packet")
			}
		}
		ethernetFrame(b)
	})
}
