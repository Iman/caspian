// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Iman Samizadeh

// Handshake algorithm adapted from patterniha/SNI-Spoofing.
// See third_party/sni-spoofing/README.md for provenance and modifications.
package snispoof

import (
	"encoding/binary"
	"net/netip"
	"sync"
)

const (
	fin = 1
	syn = 2
	rst = 4
	ack = 16
)

// frame carries one IPv4 packet and the backend's reinjection metadata.
type frame struct {
	ip      []byte
	prefix  []byte
	address [80]byte
}

type packetIO interface {
	Receive() (frame, error)
	Send(frame) error
	Close() error
}

type packet struct {
	src, dst                  netip.AddrPort
	seq, ack                  uint32
	flags                     byte
	ipLen, tcpLen, payloadLen int
}

func parsePacket(b []byte) (packet, bool) {
	var p packet
	if len(b) < 40 || b[0]>>4 != 4 || b[9] != 6 {
		return p, false
	}
	ihl := int(b[0]&15) * 4
	total := int(binary.BigEndian.Uint16(b[2:4]))
	// No fragments: a partial TCP segment cannot prove a handshake state.
	if ihl < 20 || total > len(b) || total < ihl+20 || binary.BigEndian.Uint16(b[6:8])&0x3fff != 0 {
		return p, false
	}
	tcp := b[ihl:total]
	thl := int(tcp[12]>>4) * 4
	if thl < 20 || thl > len(tcp) {
		return p, false
	}
	p.src = netip.AddrPortFrom(netip.AddrFrom4([4]byte(b[12:16])), binary.BigEndian.Uint16(tcp[:2]))
	p.dst = netip.AddrPortFrom(netip.AddrFrom4([4]byte(b[16:20])), binary.BigEndian.Uint16(tcp[2:4]))
	p.seq = binary.BigEndian.Uint32(tcp[4:8])
	p.ack = binary.BigEndian.Uint32(tcp[8:12])
	p.flags = tcp[13]
	p.ipLen, p.tcpLen, p.payloadLen = ihl, thl, len(tcp)-thl
	return p, true
}

func checksum(b []byte, sum uint32) uint16 {
	for len(b) >= 2 {
		sum += uint32(binary.BigEndian.Uint16(b))
		b = b[2:]
	}
	if len(b) == 1 {
		sum += uint32(b[0]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

func fakeFrame(f frame, p packet, isn uint32, hello []byte) frame {
	out := append([]byte(nil), f.ip[:p.ipLen+p.tcpLen]...)
	out = append(out, hello...)
	binary.BigEndian.PutUint16(out[2:4], uint16(len(out)))
	binary.BigEndian.PutUint16(out[4:6], binary.BigEndian.Uint16(out[4:6])+1)
	out[10], out[11] = 0, 0
	binary.BigEndian.PutUint16(out[10:12], checksum(out[:p.ipLen], 0))
	tcp := out[p.ipLen:]
	tcp[13] |= 8 // PSH, keeping the third ACK's other header fields.
	binary.BigEndian.PutUint32(tcp[4:8], isn+1-uint32(len(hello)))
	tcp[16], tcp[17] = 0, 0
	pseudo := append([]byte(nil), out[12:20]...)
	pseudo = append(pseudo, 0, 6)
	pseudo = binary.BigEndian.AppendUint16(pseudo, uint16(len(tcp)))
	binary.BigEndian.PutUint16(tcp[16:18], checksum(tcp, uint32(^checksum(pseudo, 0))))
	f.ip = out
	return f
}

type flowKey struct{ local, remote netip.AddrPort }

// A flow exists only after the forwarder reserves its own socket's local port.
type flow struct {
	mu                                           sync.Mutex
	hello                                        []byte
	isn, peerISN                                 uint32
	sawSYN, sawSYNACK, scheduled, sent, finished bool
	done                                         chan struct{}
	err                                          error
}

func (f *flow) finish(err error) {
	if !f.finished {
		f.err = err
		f.finished = true
		close(f.done)
	}
}

// observe returns an injection candidate only for the verified third ACK.
// The caller holds mu. Repeated handshake packets retain the same state.
func (f *flow) observe(p packet, outbound bool) bool {
	if f.finished {
		return false
	}
	if p.flags&(fin|rst) != 0 {
		f.finish(ErrConfirmation)
		return false
	}
	if outbound && f.sent && p.seq == f.isn+1-uint32(len(f.hello)) && p.payloadLen == len(f.hello) {
		return false
	}
	if p.payloadLen != 0 {
		f.finish(ErrConfirmation)
		return false
	}
	if outbound && p.flags&syn != 0 && p.flags&ack == 0 {
		if p.ack != 0 || (f.sawSYN && f.isn != p.seq) {
			f.finish(ErrConfirmation)
			return false
		}
		f.isn = p.seq
		f.sawSYN = true
		return false
	}
	if !outbound && p.flags&(syn|ack) == syn|ack {
		if !f.sawSYN || p.ack != f.isn+1 || (f.sawSYNACK && f.peerISN != p.seq) {
			f.finish(ErrConfirmation)
			return false
		}
		f.peerISN = p.seq
		f.sawSYNACK = true
		return false
	}
	if p.flags&ack == 0 || p.flags&syn != 0 {
		f.finish(ErrConfirmation)
		return false
	}
	if outbound {
		if !f.sawSYNACK || p.seq != f.isn+1 || p.ack != f.peerISN+1 {
			f.finish(ErrConfirmation)
			return false
		}
		if !f.scheduled {
			f.scheduled = true
			return true
		}
	} else if f.sent {
		if p.seq != f.peerISN+1 || p.ack != f.isn+1 {
			f.finish(ErrConfirmation)
		} else {
			f.finish(nil)
		}
	}
	return false
}
