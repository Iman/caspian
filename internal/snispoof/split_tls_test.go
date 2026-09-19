// SPDX-License-Identifier: AGPL-3.0-or-later
package snispoof

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSplitCombinationsPreserveRealTLS(t *testing.T) { testTLSCombinations(t, "") }
func testTLSCombinations(t *testing.T, fake string) {
	for _, version := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		for _, tcp := range []bool{false, true} {
			for _, rec := range []bool{false, true} {
				t.Run(fmt.Sprintf("tls%d/tcp=%t/record=%t", version, tcp, rec), func(t *testing.T) {
					want := bytes.Repeat([]byte("verified TLS payload\x00"), 4096)
					server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.TLS.ServerName != "example.com" {
							t.Error("real SNI changed")
						}
						body, err := io.ReadAll(r.Body)
						if err != nil || !bytes.Equal(body, want) {
							t.Error("upload changed")
						}
						w.Write(want)
					}))
					observed := make(chan *recordedConn, 4)
					server.Listener = recordingListener{server.Listener, observed}
					server.TLS = &tls.Config{MinVersion: version, MaxVersion: version}
					server.StartTLS()
					defer server.Close()
					f, err := StartWithOptions(server.Listener.Addr().(*net.TCPAddr).AddrPort(), "", Options{FakeSNI: fake, TCPSplit: tcp, TLSRecordSplit: rec})
					if err != nil {
						t.Fatal(err)
					}
					defer f.Close()
					roots := x509.NewCertPool()
					roots.AddCert(server.Certificate())
					for _, name := range []string{"example.com", "wrong.example.invalid"} {
						transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: name, MinVersion: version, MaxVersion: version},
							DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
								return (&net.Dialer{}).DialContext(ctx, "tcp4", f.Addr().String())
							}}
						defer transport.CloseIdleConnections()
						client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
						response, err := client.Post(server.URL, "application/octet-stream", bytes.NewReader(want))
						seen := <-observed
						seen.mu.Lock()
						captured := append([]byte{}, seen.data...)
						seen.mu.Unlock()
						wire, _, parseErr := splitHello(bytes.NewReader(captured), false)
						if parseErr != nil {
							t.Fatalf("server did not receive a complete ClientHello: %v", parseErr)
						}
						_, recordCount := handshakeBytes(t, wire)
						expectedRecords := 1
						if rec {
							expectedRecords = 2
						}
						if recordCount != expectedRecords {
							t.Fatalf("server saw %d TLS records, want %d", recordCount, expectedRecords)
						}
						if name != "example.com" {
							if err == nil {
								response.Body.Close()
								t.Fatal("certificate verification bypassed")
							}
							var mismatch x509.HostnameError
							if !errors.As(err, &mismatch) {
								t.Fatalf("expected hostname error, got %v", err)
							}
							continue
						}
						if err != nil {
							t.Fatal(err)
						}
						got, err := io.ReadAll(response.Body)
						response.Body.Close()
						if err != nil || !bytes.Equal(got, want) {
							t.Fatalf("download changed: %v", err)
						}
					}
				})
			}
		}
	}
}

type recordedConn struct {
	net.Conn
	mu   sync.Mutex
	data []byte
}

func (c *recordedConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.mu.Lock()
	if len(c.data) < 70000 {
		c.data = append(c.data, b[:n]...)
	}
	c.mu.Unlock()
	return n, err
}

type recordingListener struct {
	net.Listener
	observed chan *recordedConn
}

func (l recordingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	r := &recordedConn{Conn: c}
	l.observed <- r
	return r, nil
}
