// SPDX-License-Identifier: AGPL-3.0-or-later
package snispoof

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func record(payload []byte) []byte {
	b := []byte{22, 3, 1, 0, 0}
	binary.BigEndian.PutUint16(b[3:], uint16(len(payload)))
	return append(b, payload...)
}
func handshakeBytes(t *testing.T, wire []byte) ([]byte, int) {
	t.Helper()
	var all []byte
	count := 0
	for len(wire) > 0 {
		if len(wire) < 5 {
			t.Fatal("truncated record")
		}
		n := int(binary.BigEndian.Uint16(wire[3:]))
		if n > len(wire)-5 {
			t.Fatal("bad record length")
		}
		all = append(all, wire[5:5+n]...)
		wire = wire[5+n:]
		count++
	}
	return all, count
}

type smallReader struct{ io.Reader }

func (r smallReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}
func TestSplitPreservesHandshakeAcrossInputBoundaries(t *testing.T) {
	original, err := clientHello("cover.example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := handshakeBytes(t, original)
	for _, fragmented := range []bool{false, true} {
		wire := original
		if fragmented {
			wire = append(record(payload[:17]), record(payload[17:])...)
		}
		for _, tlsSplit := range []bool{false, true} {
			suffix := []byte("following record untouched")
			input := bytes.NewReader(append(append([]byte{}, wire...), suffix...))
			got, cut, err := splitHello(smallReader{input}, tlsSplit)
			if err != nil {
				t.Fatal(err)
			}
			recovered, count := handshakeBytes(t, got)
			if !bytes.Equal(recovered, payload) {
				t.Fatal("handshake changed")
			}
			_, before := handshakeBytes(t, wire)
			if tlsSplit && count != before+1 {
				t.Fatal("TLS record not split")
			}
			if !tlsSplit && !bytes.Equal(got, wire) {
				t.Fatal("TCP-only altered TLS records")
			}
			if cut <= 5 || cut >= len(got) {
				t.Fatal("invalid split boundary")
			}
			rest, _ := io.ReadAll(input)
			if !bytes.Equal(rest, suffix) {
				t.Fatal("consumed following traffic")
			}
		}
	}
}
func TestSplitRejectsMalformedAndTruncatedGreetings(t *testing.T) {
	valid, _ := clientHello("cover.example.invalid")
	cases := [][]byte{nil, valid[:4], valid[:20], []byte("GET / HTTP/1.1"), {22, 3, 1, 255, 255}, record([]byte{1, 255, 255, 255})}
	bad := append([]byte{}, valid...)
	bad[43] = 255
	cases = append(cases, bad)
	for i, b := range cases {
		if _, _, err := splitHello(bytes.NewReader(b), true); err == nil {
			t.Fatalf("accepted case %d", i)
		}
	}
}

type shortWriter struct {
	bytes.Buffer
	zero bool
	fail bool
}

func (w *shortWriter) Write(b []byte) (int, error) {
	if w.zero {
		return 0, nil
	}
	if w.fail {
		return 0, io.ErrClosedPipe
	}
	if len(b) > 3 {
		b = b[:3]
	}
	return w.Buffer.Write(b)
}
func TestSplitHandlesShortWritesAndWriteFailure(t *testing.T) {
	original, _ := clientHello("cover.example.invalid")
	for _, tcp := range []bool{false, true} {
		for _, mode := range []string{"short", "zero", "error"} {
			w := &shortWriter{zero: mode == "zero", fail: mode == "error"}
			err := forwardHello(w, bytes.NewReader(original), Options{TCPSplit: tcp, TLSRecordSplit: true})
			if mode == "zero" && !errors.Is(err, io.ErrShortWrite) {
				t.Fatal(err)
			}
			if mode == "error" && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal(err)
			}
			if mode == "short" {
				if err != nil {
					t.Fatal(err)
				}
				got, _ := handshakeBytes(t, w.Bytes())
				want, _ := handshakeBytes(t, original)
				if !bytes.Equal(got, want) {
					t.Fatal("short write lost bytes")
				}
			}
		}
	}
}
func FuzzSplitHello(f *testing.F) {
	b, _ := clientHello("cover.example.invalid")
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 70000 {
			t.Skip()
		}
		out, _, err := splitHello(bytes.NewReader(b), true)
		if err == nil {
			handshakeBytes(t, out)
		}
	})
}

func TestSplitForwarderClosesWithoutSendingMalformedGreeting(t *testing.T) {
	for _, stopEarly := range []bool{false, true} {
		ln, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		f, err := StartWithOptions(ln.Addr().(*net.TCPAddr).AddrPort(), "", Options{TCPSplit: true, TLSRecordSplit: true})
		if err != nil {
			t.Fatal(err)
		}
		c, err := net.Dial("tcp4", f.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		server, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		server.SetReadDeadline(time.Now().Add(time.Second))
		if stopEarly {
			c.Write([]byte{22, 3})
			f.Close()
		} else {
			c.Write([]byte("GET / HTTP/1.1"))
		}
		b := make([]byte, 20)
		n, err := server.Read(b)
		if n != 0 || err == nil {
			t.Fatal("malformed/partial greeting leaked")
		}
		if e, ok := err.(net.Error); ok && e.Timeout() {
			t.Fatal("connection not closed")
		}
		c.Close()
		server.Close()
		ln.Close()
		f.Close()
	}
}
