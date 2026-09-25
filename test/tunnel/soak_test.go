// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/proxy"

	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/xcfg"
)

// # The load test, and why it exists
//
// GitHub issue 7: on v0.2.12-rc.2, caspian.exe on Windows grew to 1.9 GB at
// 0% CPU. This test found 2 loops through the tunnel. In both, the private
// rule sent traffic direct, and the direct connection went back into the
// tunnel as a new flow.
//
//   - Variant T: Windows sends NetBIOS broadcasts to the tunnel subnet.
//     xcfg.loopGuardRule blocks them. Before, T ended at 48,946 goroutines.
//   - Variant V: a private address off the LAN, which on Windows only the
//     tunnel has a route for. Binding direct to the uplink (xcfg
//     Direct.Interface) fixes it. Before, V ended at 44,415 goroutines.
//
// The test puts sustained load through the real engine for each variant
// below. It passes when the goroutine count falls back near its start value
// after the load stops.
//
// It is opt-in, because it runs for minutes and moves gigabytes on loopback:
//
//	CASPIAN_SOAK=1 go test ./test/tunnel -run TestSustainedLoadLeavesNoGoroutinesBehind -v
//
// Other settings:
//
//	CASPIAN_SOAK_DUR      load time for each variant, default 60s
//	CASPIAN_SOAK_ONLY     variant letters to run, for example "TU"
//	CASPIAN_SOAK_PROF     directory for heap and goroutine profiles and the engine log
//	CASPIAN_SOAK_INFO     set to raise the engine log level to info
//	CASPIAN_SOAK_NOGUARD  set to leave out the tunnel subnet and the direct
//	                      binding, which shows the old leak
//
// Variants T and U use the real TUN inbound. They run on Windows only, as
// administrator, with wintun.dll next to the test binary:
//
//	go test -c -o soak/tunnel.test.exe ./test/tunnel
//	copy wintun.dll soak\
//	soak\tunnel.test.exe -test.run TestSustainedLoadLeavesNoGoroutinesBehind -test.v
//
// They route only 203.0.113.0/24 (TEST-NET-3) through the adapter, so the
// traffic of the machine itself stays out of the test.

const (
	soakTunName   = "soaktun"
	soakTunAddr   = "10.231.99.1"
	soakTunSubnet = "10.231.99.0/24"
	soakTarget    = "203.0.113.0/24"
	soakPrivate   = "10.99.0.0/16"
	soakWorkers   = 16
	// soakSlack is how many goroutines above the start value the idle sample
	// can keep. The fixed engine ends below its start value. The leak ended
	// near 49,000.
	soakSlack = 100
)

// soakVariant is one client shape. The fields map to what a BPB JSON config
// carries: a server named by domain or address, pinned addresses, the
// happyEyeballs race and the TLS ClientHello fragment.
type soakVariant struct {
	name     string
	address  string
	pinned   []netip.Addr
	happy    bool
	fragment bool
	stall    bool
	tun      bool
	// private sends the load to a private address that only the tunnel has
	// a route for. On Windows that is every private address off the LAN,
	// because the default route of the whole host is the tunnel.
	private bool
}

func soakVariants() []soakVariant {
	two := []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("127.0.0.2")}
	dead := []netip.Addr{two[0], netip.MustParseAddr("127.0.0.3")}
	deadFirst := []netip.Addr{dead[1], two[0]}
	return []soakVariant{
		{name: "A-ip-literal", address: "127.0.0.1"},
		{name: "B-domain-1pin", address: serverName, pinned: two[:1]},
		{name: "C-domain-2pin-happy", address: serverName, pinned: two, happy: true},
		{name: "D-domain-2pin-happy-fragment", address: serverName, pinned: two, happy: true, fragment: true},
		{name: "E-ip-literal-fragment", address: "127.0.0.1", fragment: true},
		{name: "F-domain-2pin-onedead-random", address: serverName, pinned: dead},
		{name: "G-domain-2pin-onedead-happy", address: serverName, pinned: deadFirst, happy: true, fragment: true},
		{name: "H-ip-literal-stall", address: "127.0.0.1", happy: true, fragment: true, stall: true},
		{name: "I-domain-2pin-happy-stall", address: serverName, pinned: two, happy: true, fragment: true, stall: true},
		{name: "T-tun-ip-literal", address: "127.0.0.1", happy: true, fragment: true, tun: true},
		{name: "U-tun-domain-2pin-happy-fragment", address: serverName, pinned: two, happy: true, fragment: true, tun: true},
		{name: "V-tun-private-off-lan", address: "127.0.0.1", happy: true, fragment: true, tun: true, private: true},
	}
}

func TestSustainedLoadLeavesNoGoroutinesBehind(t *testing.T) {
	if os.Getenv("CASPIAN_SOAK") == "" {
		t.Skip("set CASPIAN_SOAK=1 to run the load test; it runs for minutes")
	}
	dur := 60 * time.Second
	if v := os.Getenv("CASPIAN_SOAK_DUR"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			t.Fatalf("CASPIAN_SOAK_DUR=%q is not a positive duration", v)
		}
		dur = d
	}
	only := os.Getenv("CASPIAN_SOAK_ONLY")
	for _, v := range soakVariants() {
		if only != "" && !strings.Contains(only, v.name[:1]) {
			continue
		}
		t.Run(v.name, func(t *testing.T) {
			if v.tun && runtime.GOOS != "windows" {
				t.Skip("the TUN variants add the adapter address and route with PowerShell, so they run on Windows only")
			}
			soakOne(t, v, dur)
		})
	}
}

// soakOrigin serves n bytes for a GET of /?n=<n>.
func soakOrigin(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening for the origin: %v", err)
	}
	chunk := make([]byte, 64<<10)
	srv := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n, _ := strconv.Atoi(r.URL.Query().Get("n"))
			w.Header().Set("Content-Length", strconv.Itoa(n))
			for n > 0 {
				k := min(n, len(chunk))
				if _, err := w.Write(chunk[:k]); err != nil {
					return
				}
				n -= k
			}
		}),
	}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

// soakStallOrigin accepts connections and never answers. It models a server
// that takes the connection and then carries nothing.
func soakStallOrigin(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening for the stalled origin: %v", err)
	}
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = l.Close()
		mu.Lock()
		for _, c := range held {
			_ = c.Close()
		}
		mu.Unlock()
	})
	return l.Addr().(*net.TCPAddr).Port
}

// soakServer runs VLESS over WebSocket and TLS on 127.0.0.1 and 127.0.0.2, so
// a name can pin to 2 live addresses.
func soakServer(t *testing.T, port int, cert serverCert, originPort int) {
	t.Helper()
	in := func(tag, ip string) string {
		return fmt.Sprintf(`{"tag": %q, "listen": %q, "port": %d, "protocol": "vless",
  "settings": {"clients": [{"id": %q}], "decryption": "none"},
  "streamSettings": {"network": "ws", "wsSettings": {"path": "/soak"}, "security": "tls",
    "tlsSettings": {"alpn": ["http/1.1"], "certificates": [{"usage": "encipherment", "certificate": %s, "key": %s}]}}}`,
			tag, ip, port, credVLess, jsonArray(cert.certPEM), jsonArray(cert.keyPEM))
	}
	startXrayServer(t, serverConfig(in("in1", "127.0.0.1")+","+in("in2", "127.0.0.2"), originPort))
}

// soakClientJSON is a BPB-style xray JSON config. It enters the product
// through link.Parse, the same path as a pasted JSON config.
func soakClientJSON(v soakVariant, port int, pin string) string {
	sockopt := `{"domainStrategy": "UseIP"}`
	if v.happy {
		sockopt = `{"domainStrategy": "UseIP", "happyEyeballs": {"tryDelayMs": 250, "prioritizeIPv6": false, "interleave": 2, "maxConcurrentTry": 4}}`
	}
	mask := ""
	if v.fragment {
		mask = `, "finalmask": {"tcp": [{"type": "fragment", "settings": {"packets": "tlshello", "length": "100-200", "delay": "1"}}]}`
	}
	return fmt.Sprintf(`{"remarks": "soak", "outbounds": [{"tag": "proxy", "protocol": "vless",
  "settings": {"vnext": [{"address": %q, "port": %d, "users": [{"id": %q, "encryption": "none"}]}]},
  "streamSettings": {"network": "ws", "wsSettings": {"host": %q, "path": "/soak"}, "security": "tls",
    "tlsSettings": {"serverName": %q, "alpn": ["http/1.1"], "pinnedPeerCertSha256": %q},
    "sockopt": %s%s}},
  {"tag": "direct", "protocol": "freedom"}]}`, v.address, port, credVLess, certCommonName, certCommonName, pin, sockopt, mask)
}

func soakOne(t *testing.T, v soakVariant, dur time.Duration) {
	originPort := soakOrigin(t)
	if v.stall {
		originPort = soakStallOrigin(t)
	}
	cert := makeServerCert(t)
	serverPort := freeLoopbackPort(t)
	socksPort := freeLoopbackPort(t)
	soakServer(t, serverPort, cert, originPort)

	l, err := link.Parse(soakClientJSON(v, serverPort, cert.pinHex))
	if err != nil {
		t.Fatalf("the JSON config did not parse: %v", err)
	}
	o := xcfg.Defaults()
	o.Link = l
	o.TUN.Disabled = !v.tun
	o.TUN.Name = soakTunName
	guard := os.Getenv("CASPIAN_SOAK_NOGUARD") == ""
	if guard {
		o.TUN.Subnet = netip.MustParsePrefix(soakTunSubnet)
		if v.tun {
			// As internal/privsvc does on Windows: bind direct to the uplink.
			o.Direct.Interface = soakUplink(t)
		}
	}
	o.SOCKS.Listen = "127.0.0.1"
	o.SOCKS.Port = uint16(socksPort)
	o.PinnedServer = v.pinned
	if os.Getenv("CASPIAN_SOAK_INFO") != "" {
		o.LogLevel = xcfg.LogInfo
	}
	doc, err := xcfg.Build(o)
	if err != nil {
		t.Fatalf("xcfg.Build: %v", err)
	}
	e := engine.New()
	if err := e.Start(context.Background(), doc); err != nil {
		t.Fatalf("engine start: %v", err)
	}
	defer func() { _ = e.Stop() }()
	time.Sleep(500 * time.Millisecond)

	var d proxy.Dialer
	target := fmt.Sprintf("%s:%d", originHost, 80)
	if v.tun {
		// The adapter needs a moment to appear before it takes an address.
		time.Sleep(2 * time.Second)
		soakPowerShell(t, fmt.Sprintf("New-NetIPAddress -InterfaceAlias %s -IPAddress %s -PrefixLength 24 | Out-Null", soakTunName, soakTunAddr))
		soakPowerShell(t, fmt.Sprintf("New-NetRoute -InterfaceAlias %s -DestinationPrefix %s -RouteMetric 0 -PolicyStore ActiveStore | Out-Null", soakTunName, soakTarget))
		t.Cleanup(func() {
			_ = exec.Command("powershell", "-NoProfile", "-Command",
				fmt.Sprintf("Remove-NetRoute -DestinationPrefix %s -Confirm:$false -ErrorAction SilentlyContinue", soakTarget)).Run()
		})
		if v.private {
			soakPowerShell(t, fmt.Sprintf("New-NetRoute -InterfaceAlias %s -DestinationPrefix %s -RouteMetric 0 -PolicyStore ActiveStore | Out-Null", soakTunName, soakPrivate))
			t.Cleanup(func() {
				_ = exec.Command("powershell", "-NoProfile", "-Command",
					fmt.Sprintf("Remove-NetRoute -DestinationPrefix %s -Confirm:$false -ErrorAction SilentlyContinue", soakPrivate)).Run()
			})
		}
		time.Sleep(2 * time.Second)
		if guard {
			soakDirectReachesLoopback(t, socksPort, originPort, v.stall)
		}
		d = &net.Dialer{Timeout: 8 * time.Second}
		target = "203.0.113.10:80"
		if v.private {
			target = "10.99.1.1:80"
		}
	} else {
		d, err = proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), nil, proxy.Direct)
		if err != nil {
			t.Fatalf("SOCKS dialer: %v", err)
		}
	}

	var ok, fail, moved atomic.Int64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < soakWorkers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			// A new transport for each request and no keep-alive, so every
			// request makes a new tunnel connection, as a hotspot of browsers does.
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				n := 2048
				if (w+i)%4 == 0 {
					n = 1 << 20
				}
				tr := &http.Transport{
					DialContext:       func(_ context.Context, nw, a string) (net.Conn, error) { return d.Dial(nw, a) },
					DisableKeepAlives: true,
				}
				c := &http.Client{Transport: tr, Timeout: 8 * time.Second}
				resp, err := c.Get(fmt.Sprintf("http://%s/?n=%d", target, n))
				if err != nil {
					fail.Add(1)
					continue
				}
				k, _ := io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				moved.Add(k)
				if k == int64(n) {
					ok.Add(1)
				} else {
					fail.Add(1)
				}
				tr.CloseIdleConnections()
			}
		}(w)
	}

	sample := func(label string) int {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		g := runtime.NumGoroutine()
		t.Logf("%-6s heapLive=%6.1fMB sys=%6.1fMB goroutines=%5d ok=%6d fail=%5d moved=%7.0fMB",
			label, float64(m.HeapAlloc)/1e6, float64(m.Sys)/1e6, g, ok.Load(), fail.Load(), float64(moved.Load())/1e6)
		return g
	}
	start := sample("t=0")
	began := time.Now()
	tick := time.NewTicker(dur / 6)
	for time.Since(began) < dur {
		<-tick.C
		sample(fmt.Sprintf("t=%ds", int(time.Since(began).Seconds())))
	}
	tick.Stop()
	close(stop)
	wg.Wait()
	time.Sleep(3 * time.Second)
	idle := sample("idle")

	if dir := os.Getenv("CASPIAN_SOAK_PROF"); dir != "" {
		soakProfiles(t, dir, v.name, e)
	}
	if !v.stall && !v.private && ok.Load() == 0 {
		t.Fatalf("no request completed, so the test measured nothing")
	}
	if idle > start+soakSlack {
		t.Fatalf("%d goroutines remain after the load stopped, against %d at the start. "+
			"Something the load created is never released. Set CASPIAN_SOAK_PROF to see what.", idle, start)
	}
}

func soakPowerShell(t *testing.T, cmd string) {
	t.Helper()
	out, err := exec.Command("powershell", "-NoProfile", "-Command", cmd).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s\nThe TUN variants need administrator rights and wintun.dll next to the test binary.", cmd, err, out)
	}
}

// soakProfiles writes the heap profile, the goroutine dump and the engine log
// of one variant into dir.
func soakProfiles(t *testing.T, dir, name string, e *engine.Engine) {
	t.Helper()
	write := func(file string, fill func(io.Writer) error) {
		f, err := os.Create(fmt.Sprintf("%s/%s-%s", dir, file, name))
		if err != nil {
			t.Errorf("creating %s: %v", file, err)
			return
		}
		defer f.Close()
		if err := fill(f); err != nil {
			t.Errorf("writing %s: %v", file, err)
		}
	}
	write("heap.pprof", pprof.WriteHeapProfile)
	write("goroutine.txt", func(w io.Writer) error { return pprof.Lookup("goroutine").WriteTo(w, 1) })
	write("log.txt", func(w io.Writer) error {
		for _, le := range e.Logs() {
			if _, err := fmt.Fprintf(w, "%+v\n", le); err != nil {
				return err
			}
		}
		return nil
	})
}

// soakUplink returns the alias of the adapter that carries the default route
// of this machine, the same adapter netcfg.Plan.Uplink names on Windows.
func soakUplink(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"Get-NetRoute -DestinationPrefix 0.0.0.0/0 | Sort-Object { $_.RouteMetric + "+
			"(Get-NetIPInterface -InterfaceIndex $_.InterfaceIndex -AddressFamily IPv4).InterfaceMetric } | "+
			"Select-Object -First 1 -ExpandProperty InterfaceAlias").Output()
	alias := strings.TrimSpace(string(out))
	if err != nil || alias == "" {
		t.Fatalf("no default route on this machine, so there is no uplink to bind to: %v", err)
	}
	return alias
}

// soakDirectReachesLoopback sends one request through the SOCKS inbound to a
// loopback address. The private rule sends it direct. It proves that the
// binding to the uplink does not cut the direct outbound off from loopback.
func soakDirectReachesLoopback(t *testing.T, socksPort, originPort int, stalled bool) {
	t.Helper()
	if stalled {
		return
	}
	d, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), nil, proxy.Direct)
	if err != nil {
		t.Fatalf("SOCKS dialer: %v", err)
	}
	c := &http.Client{
		Transport: &http.Transport{DialContext: func(_ context.Context, nw, a string) (net.Conn, error) { return d.Dial(nw, a) }},
		Timeout:   8 * time.Second,
	}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/?n=16", originPort))
	if err != nil {
		t.Fatalf("the bound direct outbound did not reach loopback: %v", err)
	}
	n, _ := io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if n != 16 {
		t.Fatalf("the bound direct outbound returned %d bytes from loopback, want 16", n)
	}
}
