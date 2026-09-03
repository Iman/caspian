// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package tunnel

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/xcfg"

	"github.com/xtls/xray-core/core"
	confserial "github.com/xtls/xray-core/infra/conf/serial"

	// The server side needs the same protocol, transport and config
	// registrations the client side gets. internal/engine blank-imports this
	// for its own use; this package builds a core.Instance directly, so it
	// imports it directly too rather than relying on that being a side effect
	// of another package's import list.
	_ "github.com/xtls/xray-core/main/distro/all"
)

// ---------------------------------------------------------------------------
// The names and addresses in this file
//
// Everything is invented and everything is loopback. "origin.invalid" is in
// the .invalid top level domain, which RFC 6761 section 6.4 reserves so that
// it never resolves; that property is load-bearing here and not decoration.
// No credential below is, or has ever been, a working one.
// ---------------------------------------------------------------------------

// originHost is the name the client asks for. It is never the origin's real
// address, which is a loopback port the client is never told.
const originHost = "origin.invalid"

// certCommonName is the subject of the generated server certificate. It is
// never used for verification: the share links pin the certificate by digest.
const certCommonName = "caspian-test-server.invalid"

// The invented credentials. One per protocol, so a failure names the protocol
// on its own.
const (
	credVMess       = "11111111-2222-4333-8444-555555555555"
	credVLess       = "11111111-2222-4333-8444-666666666666"
	credSSMethod    = "aes-256-gcm"
	credSSPassword  = "caspian-test-shadowsocks-password"
	credSocksUser   = "caspian-test-socks-user"
	credSocksPass   = "caspian-test-socks-pass"
	credTrojan      = "caspian-test-trojan-password"
	credHysteria    = "caspian-test-hysteria-auth"
	wrongCredential = "caspian-test-wrong-credential"
	wrongUUID       = "99999999-8888-4777-8666-555555555555"
)

// ---------------------------------------------------------------------------
// TestMain
// ---------------------------------------------------------------------------

// TestMain claims the engine's log routes before any xray-core instance is
// built.
//
// internal/engine installs a handler creator for app/log's console type the
// first time an Engine is constructed (internal/engine/logring.go,
// captureEngineLogs). Until that has happened, common/log's own init handler is
// in place and every instance this package starts writes to stdout, which turns
// a test run into a wall of engine chatter and puts the SERVER's log lines
// somewhere no assertion can read them. Constructing one Engine here and
// dropping it installs the creator once, for the whole package.
func TestMain(m *testing.M) {
	_ = engine.New()
	m.Run()
}

// ---------------------------------------------------------------------------
// Ports
// ---------------------------------------------------------------------------

// freeLoopbackPort asks the kernel for an unused port and gives it straight
// back. Same helper, and the same reasoning, as
// internal/engine/engine_test.go: a hardcoded port collides with a parallel
// test every time, this collides essentially never.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("releasing the reserved port: %v", err)
	}
	return port
}

// ---------------------------------------------------------------------------
// The HTTP endpoints
// ---------------------------------------------------------------------------

// seenRequest is what an endpoint records about one request it served.
//
// Host is the field the bypass proof rests on. It is the authority the CLIENT
// addressed, carried in the payload, not the socket the request arrived on.
type seenRequest struct {
	Host   string
	Path   string
	Remote string
}

// endpoint is one loopback HTTP server under this test's control.
type endpoint struct {
	port int
	body string

	mu   sync.Mutex
	seen []seenRequest
}

func (e *endpoint) record(r seenRequest) {
	e.mu.Lock()
	e.seen = append(e.seen, r)
	e.mu.Unlock()
}

func (e *endpoint) requests() []seenRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]seenRequest(nil), e.seen...)
}

// startEndpoint brings up an HTTP server on a kernel-assigned loopback port.
func startEndpoint(t *testing.T, body string) *endpoint {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening for an http endpoint: %v", err)
	}
	ep := &endpoint{port: l.Addr().(*net.TCPAddr).Port, body: body}
	srv := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ep.record(seenRequest{Host: req.Host, Path: req.URL.Path, Remote: req.RemoteAddr})
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, body)
		}),
	}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return ep
}

// ---------------------------------------------------------------------------
// The server certificate
// ---------------------------------------------------------------------------

// serverCert is a freshly generated self-signed certificate and the SHA-256 of
// its DER, which is what a share link's pcs= parameter pins.
type serverCert struct {
	certPEM []string
	keyPEM  []string
	pinHex  string
}

// makeServerCert generates the certificate the TLS-carrying protocols use.
//
// # Why the trust anchor comes out of the SHARE LINK and not out of a patched
// config
//
// trojan and hysteria2 are TLS-only here: internal/link forces security to tls
// for a trojan link (link.requireTLSForTrojan) and the vendored parser does the
// same for hysteria2 (third_party/libxray-share/stream.go, the switch in
// parseSecurityFromURL). A self-signed server therefore has to be trusted
// somehow, and the obvious two ways are both wrong for this suite: installing a
// root into the machine's trust store is not something a test may do, and
// editing the emitted client document would stop it being the document the
// product emits.
//
// It is not needed. The share link itself can carry the trust decision. The
// vendored parser maps the pcs= query parameter to tlsSettings
// pinnedPeerCertSha256 (third_party/libxray-share/stream.go,
// parseSecurityFromURL), infra/conf/transport_internet.go TLSConfig.Build
// decodes it as a 32-byte hex digest, and transport/internet/tls/config.go
// verifyPeerCert returns success as soon as the leaf's SHA-256 matches a pin.
// So the whole client configuration, credential and trust anchor included,
// still comes from one pasted string.
//
// allowInsecure would have been the other candidate and is unavailable on
// purpose: TLSConfig.Build refuses it outright once the wall clock is past
// 2026-06-01, pointing the reader at pinnedPeerCertSha256 instead.
func makeServerCert(t *testing.T) serverCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating a server key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: certCommonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{certCommonName},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("generating a server certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshalling the server key: %v", err)
	}
	sum := sha256.Sum256(der)
	return serverCert{
		certPEM: pemLines("CERTIFICATE", der),
		keyPEM:  pemLines("PRIVATE KEY", keyDER),
		pinHex:  hex.EncodeToString(sum[:]),
	}
}

// pemLines encodes a block and splits it into the array of lines the engine's
// "certificate" and "key" fields take (infra/conf/transport_internet.go,
// readFileOrString joins the array with newlines).
func pemLines(kind string, der []byte) []string {
	body := string(pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}))
	return strings.Split(strings.TrimRight(body, "\n"), "\n")
}

// jsonArray renders a string slice as a JSON array, for embedding in the
// server config templates.
func jsonArray(v []string) string {
	b, err := json.Marshal(v)
	if err != nil {
		// Marshalling a []string cannot fail.
		panic(err)
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// The xray-core server
// ---------------------------------------------------------------------------

// startXrayServer loads and starts a real xray-core instance.
//
// The config goes through infra/conf/serial.LoadJSONConfig, which is the same
// loader internal/engine uses (internal/engine/engine.go, loadConfig), so a
// server this package fails to start is a server the appliance's own loader
// would also have refused.
func startXrayServer(t *testing.T, configJSON string) {
	t.Helper()
	cfg, err := confserial.LoadJSONConfig(strings.NewReader(configJSON))
	if err != nil {
		t.Fatalf("the test server config did not load: %v\n%s", err, configJSON)
	}
	inst, err := core.New(cfg)
	if err != nil {
		t.Fatalf("core.New for the test server: %v", err)
	}
	if err := inst.Start(); err != nil {
		t.Fatalf("starting the test server: %v", err)
	}
	t.Cleanup(func() { _ = inst.Close() })
}

// serverConfig wraps one inbound in the surrounding document.
//
// The single outbound is a freedom with "redirect" pointing at the origin.
// That is the whole of the server's reach: it can send a connection to exactly
// one place, whatever destination the client asked for, and it never resolves
// a name to get there (infra/conf/freedom.go, Build sets DestinationOverride
// from Redirect). The origin's port enters the test HERE and nowhere the
// client can see.
// The empty "ipsBlocked" list is load bearing since xray-core v26.4.15. From
// that version the freedom outbound behind a vless, vmess, trojan,
// shadowsocks or hysteria INBOUND refuses private and loopback destinations
// by default (proxy/freedom/freedom.go, "applying default private IP
// blocking policy"), which is exactly what this loopback server does when it
// forwards to the origin on 127.0.0.1. An explicit empty list is the
// documented opt-out. The appliance itself is not affected: its inbounds are
// tun, socks and dokodemo, none of which the default names.
func serverConfig(inbound string, originPort int) string {
	return fmt.Sprintf(`{
  "log": {"loglevel": "warning"},
  "inbounds": [%s],
  "outbounds": [{
    "tag": "to-origin",
    "protocol": "freedom",
    "settings": {"domainStrategy": "AsIs", "redirect": "127.0.0.1:%d", "ipsBlocked": []}
  }]
}`, inbound, originPort)
}

// ---------------------------------------------------------------------------
// The client, built the way the product builds it
// ---------------------------------------------------------------------------

// startClient runs the appliance's own path from pasted text to running engine:
// link.Parse, then xcfg.Build, then engine.Start. Nothing here writes a config.
//
// Three fields differ from xcfg.Defaults(), and all three are named rather than
// left implicit:
//
//	TUN.Disabled  the SOCKS-only form the design calls for on a machine with no
//	              /dev/net/tun and no root (internal/xcfg/options.go, TUN.Disabled).
//	SOCKS.Port    a kernel-assigned port instead of the fixed 10808, so two
//	              subtests cannot collide.
//	LogLevel      raised to info so the engine's own routing decision is
//	              readable from the log ring. It changes what is logged and
//	              nothing about what is routed.
func startClient(t *testing.T, shareLink string, socksPort int) (*engine.Engine, error) {
	t.Helper()
	l, err := link.Parse(shareLink)
	if err != nil {
		return nil, fmt.Errorf("the share link did not parse: %w", err)
	}
	o := xcfg.Defaults()
	o.Link = l
	o.TUN.Disabled = true
	o.SOCKS.Listen = "127.0.0.1"
	o.SOCKS.Port = uint16(socksPort)
	o.LogLevel = xcfg.LogInfo

	doc, err := xcfg.Build(o)
	if err != nil {
		return nil, fmt.Errorf("xcfg.Build refused the parsed link: %w", err)
	}

	e := engine.New()
	if err := e.Start(context.Background(), doc); err != nil {
		return nil, fmt.Errorf("the engine refused to start with the composed document: %w", err)
	}
	t.Cleanup(func() { _ = e.Stop() })

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c, derr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), time.Second)
		if derr == nil {
			_ = c.Close()
			return e, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil, fmt.Errorf("the engine reports %s but nothing ever accepted on 127.0.0.1:%d",
		e.State().Phase, socksPort)
}

// ---------------------------------------------------------------------------
// Driving a request through the tunnel
// ---------------------------------------------------------------------------

// socksGet performs one HTTP GET through a SOCKS5 proxy.
//
// It is hand written for one reason that matters to this suite: the destination
// is sent as a SOCKS5 DOMAINNAME (address type 0x03), so the NAME travels to
// the proxy and this process never resolves it. A client that resolved the name
// locally would defeat control 2 in the package doc.
//
// timeout bounds the whole exchange after the proxy has accepted. It is a
// parameter because the two directions want different numbers: see
// carryTimeout and blockedTimeout.
func socksGet(proxyAddr, host string, port int, path string, timeout time.Duration) (string, error) {
	c, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("the socks inbound did not accept a connection: %w", err)
	}
	defer func() { _ = c.Close() }()
	if err := c.SetDeadline(time.Now().Add(timeout)); err != nil {
		return "", err
	}

	// Greeting: version 5, one method, no authentication.
	if _, err := c.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return "", fmt.Errorf("writing the socks greeting: %w", err)
	}
	var greeting [2]byte
	if _, err := io.ReadFull(c, greeting[:]); err != nil {
		return "", fmt.Errorf("reading the socks greeting reply: %w", err)
	}
	if greeting[0] != 0x05 || greeting[1] != 0x00 {
		return "", fmt.Errorf("the socks inbound refused the greeting: %v", greeting)
	}

	if len(host) > 255 {
		return "", fmt.Errorf("host name too long for socks5: %d bytes", len(host))
	}
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, host...)
	req = append(req, byte(port>>8), byte(port))
	if _, err := c.Write(req); err != nil {
		return "", fmt.Errorf("writing the socks connect request: %w", err)
	}
	var head [4]byte
	if _, err := io.ReadFull(c, head[:]); err != nil {
		return "", fmt.Errorf("reading the socks connect reply: %w", err)
	}
	if head[1] != 0x00 {
		return "", fmt.Errorf("the tunnel refused the connect request, socks reply code %d", head[1])
	}
	if err := discardBoundAddress(c, head[3]); err != nil {
		return "", err
	}

	authority := fmt.Sprintf("%s:%d", host, port)
	if _, err := fmt.Fprintf(c, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n",
		path, authority); err != nil {
		return "", fmt.Errorf("writing the http request into the tunnel: %w", err)
	}
	return readHTTPBody(c)
}

// discardBoundAddress consumes the bound address a SOCKS5 reply carries after
// its four-byte header.
func discardBoundAddress(c net.Conn, atyp byte) error {
	var n int
	switch atyp {
	case 0x01:
		n = 4 + 2
	case 0x04:
		n = 16 + 2
	case 0x03:
		var l [1]byte
		if _, err := io.ReadFull(c, l[:]); err != nil {
			return fmt.Errorf("reading the socks reply address length: %w", err)
		}
		n = int(l[0]) + 2
	default:
		return fmt.Errorf("the socks reply used an unknown address type %d", atyp)
	}
	if _, err := io.ReadFull(c, make([]byte, n)); err != nil {
		return fmt.Errorf("reading the socks reply address: %w", err)
	}
	return nil
}

// readHTTPBody parses one HTTP response off the wire.
//
// http.ReadResponse rather than io.ReadAll: the body is read to its declared
// length instead of waiting for the far end to close, which is the difference
// between a subtest that finishes in milliseconds and one that waits out an
// idle timeout.
func readHTTPBody(c net.Conn) (string, error) {
	resp, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		return "", fmt.Errorf("the tunnel returned no readable http response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading the response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the origin answered %s", resp.Status)
	}
	return string(body), nil
}

// directGet is the bypass control: an ordinary HTTP request, with no proxy,
// to whatever the client was told the origin's address is.
//
// It is EXECUTED in every subtest rather than argued about. If it ever returns
// the origin's token, the tunnel was not needed and the subtest fails.
func directGet(host string, port int, path string) (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d%s", host, port, path))
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ---------------------------------------------------------------------------
// Tokens
// ---------------------------------------------------------------------------

// newToken returns a value that exists only for the subtest that made it, so a
// cached response, a crossed wire between two subtests or a stale listener
// cannot satisfy an assertion.
func newToken(t *testing.T, kind string) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("generating a token: %v", err)
	}
	return kind + "-" + hex.EncodeToString(b[:])
}
