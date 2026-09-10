// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import (
	"encoding/binary"
	"errors"
	"io"
	"time"
)

// Options keeps each intervention independent. Empty options relay normally.
type Options struct {
	FakeSNI        string
	TCPSplit       bool
	TLSRecordSplit bool
}

var ErrClientHello = errors.New("DPI splitting: invalid or oversized TLS ClientHello")

// splitHello reads bounded TLS records, regardless of socket read boundaries.
// It preserves all handshake bytes and any following bytes in the final record.
// The split falls inside the SNI hostname, or after the handshake type without SNI.
func splitHello(r io.Reader, records bool) ([]byte, int, error) {
	var wire, hello []byte
	type span struct{ wire, payload, size int }
	var spans []span
	need := 4
	for len(hello) < need {
		var header [5]byte
		if _, err := io.ReadFull(r, header[:]); err != nil {
			return nil, 0, err
		}
		n := int(binary.BigEndian.Uint16(header[3:]))
		if header[0] != 22 || header[1] != 3 || header[2] > 3 || n == 0 || n > 16384 || len(spans) >= 64 || len(hello)+n > 65536 {
			return nil, 0, ErrClientHello
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, 0, err
		}
		spans = append(spans, span{len(wire), len(hello), n})
		wire = append(wire, header[:]...)
		wire = append(wire, body...)
		hello = append(hello, body...)
		if len(hello) >= 4 {
			need = 4 + int(hello[1])<<16 + int(hello[2])<<8 + int(hello[3])
			if hello[0] != 1 || need > 65536 || need < 42 {
				return nil, 0, ErrClientHello
			}
		}
	}
	cut, err := helloSplitPosition(hello[:need])
	if err != nil {
		return nil, 0, err
	}
	for _, s := range spans {
		if cut < s.payload || cut >= s.payload+s.size {
			continue
		}
		part := cut - s.payload
		at := s.wire + 5 + part
		if records && part > 0 {
			out := make([]byte, 0, len(wire)+5)
			out = append(out, wire[:at]...)
			binary.BigEndian.PutUint16(out[s.wire+3:s.wire+5], uint16(part))
			out = append(out, wire[s.wire:s.wire+3]...)
			out = binary.BigEndian.AppendUint16(out, uint16(s.size-part))
			out = append(out, wire[at:]...)
			return out, at + 5, nil
		}
		return wire, at, nil
	}
	return nil, 0, ErrClientHello
}

func helloSplitPosition(b []byte) (int, error) {
	// Legacy version + random, followed by length-prefixed session/ciphers/compression.
	if len(b) < 39 {
		return 0, ErrClientHello
	}
	p := 38
	n := int(b[p])
	p++
	if n > 32 || p+n+2 > len(b) {
		return 0, ErrClientHello
	}
	p += n
	n = int(binary.BigEndian.Uint16(b[p:]))
	p += 2
	if n == 0 || n%2 != 0 || p+n+1 > len(b) {
		return 0, ErrClientHello
	}
	p += n
	n = int(b[p])
	p++
	if n == 0 || p+n > len(b) {
		return 0, ErrClientHello
	}
	p += n
	if p == len(b) {
		return 1, nil
	}
	if p+2 > len(b) || int(binary.BigEndian.Uint16(b[p:])) != len(b)-p-2 {
		return 0, ErrClientHello
	}
	p += 2
	cut := 1
	seen := false
	for p < len(b) {
		if p+4 > len(b) {
			return 0, ErrClientHello
		}
		kind := binary.BigEndian.Uint16(b[p:])
		n = int(binary.BigEndian.Uint16(b[p+2:]))
		p += 4
		if p+n > len(b) {
			return 0, ErrClientHello
		}
		if kind == 0 {
			if seen || n < 5 || int(binary.BigEndian.Uint16(b[p:])) != n-2 {
				return 0, ErrClientHello
			}
			seen = true
			end := p + n
			for q := p + 2; q < end; {
				if q+3 > end {
					return 0, ErrClientHello
				}
				nameType := b[q]
				size := int(binary.BigEndian.Uint16(b[q+1:]))
				q += 3
				if size == 0 || q+size > end {
					return 0, ErrClientHello
				}
				if nameType == 0 {
					cut = q + size/2
				}
				q += size
			}
		}
		p += n
	}
	return cut, nil
}

func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if n < 0 || n > len(b) {
			return io.ErrShortWrite
		}
		b = b[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func forwardHello(w io.Writer, r io.Reader, options Options) error {
	b, cut, err := splitHello(r, options.TLSRecordSplit)
	if err != nil {
		return err
	}
	if !options.TCPSplit {
		return writeAll(w, b)
	}
	if err := writeAll(w, b[:cut]); err != nil {
		return err
	}
	// Best effort TCP segmentation, never byte reordering. Packet captures are
	// required to verify segmentation on a particular host/network.
	time.Sleep(time.Millisecond)
	return writeAll(w, b[cut:])
}
