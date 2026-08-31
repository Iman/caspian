// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"net/netip"
	"path"
	"strings"
	"testing"
	"time"
)

// testDNS is the reference DHCP and DNS configuration. The upstream is a
// loopback address with a non-standard port because that is where this
// program's own resolver listens; the tunnel is behind it.
func testDNS() DNSConfig {
	return DNSConfig{
		Interface:  "wlan0",
		Subnet:     netip.MustParsePrefix("192.168.66.0/24"),
		Gateway:    netip.MustParseAddr("192.168.66.1"),
		RangeStart: netip.MustParseAddr("192.168.66.50"),
		RangeEnd:   netip.MustParseAddr("192.168.66.150"),
		LeaseTime:  12 * time.Hour,
		LeaseFile:  "/var/lib/caspian/dnsmasq.leases",
		Upstream:   netip.MustParseAddrPort("127.0.0.1:5354"),
		CacheSize:  1000,

		ServiceUser:  DefaultServiceUser,
		ServiceGroup: DefaultServiceGroup,
	}
}

func TestRenderDnsmasqGolden(t *testing.T) {
	got, err := RenderDnsmasq(testDNS())
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	assertGolden(t, "dnsmasq.golden", got)
}

// TestNoQueryLoggingDirective is the regression test for
// 004-hotspot/install.sh:352 (log-queries) and :381 (log-dhcp).
//
// It checks two different things, because they are two different failures.
// log-queries must be absent, and quiet-dhcp must be PRESENT: dnsmasq logs
// every DHCP transaction by default, so leaving log-dhcp out is not enough to
// stop the box keeping a dated record of every device that joined.
func TestNoQueryLoggingDirective(t *testing.T) {
	got, err := RenderDnsmasq(testDNS())
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	for _, forbidden := range []string{"log-queries", "log-dhcp", "log-async", "log-debug"} {
		if containsDirective(got, forbidden) {
			t.Errorf("%q appears in the generated dnsmasq configuration; this box must not record "+
				"what its users look up", forbidden)
		}
	}
	for _, required := range []string{"quiet-dhcp", "quiet-dhcp6", "quiet-ra"} {
		if !containsDirective(got, required) {
			t.Errorf("%q is missing; without it dnsmasq logs every DHCP transaction by default", required)
		}
	}
}

// TestForwardingTargetIsLocalOnly covers the second guarantee this file makes:
// a client query never leaves the box except through the tunnel.
func TestForwardingTargetIsLocalOnly(t *testing.T) {
	got, err := RenderDnsmasq(testDNS())
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	if !containsDirective(got, "no-resolv") {
		t.Error("no-resolv is missing; dnsmasq would inherit the uplink's resolver from /etc/resolv.conf")
	}
	if !containsDirective(got, "server=127.0.0.1#5354") {
		t.Error("the local resolver is not the forwarding target")
	}

	var servers []string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "server=") {
			servers = append(servers, line)
		}
	}
	if len(servers) != 1 {
		t.Errorf("expected exactly one server line, got %d: %v", len(servers), servers)
	}
}

// TestNoGoogleAnywhere is a project rule, not a preference:
// docs/2026-08-29-design.md section 2 and section 6. The reference
// implementation used 8.8.8.8 at install.sh:143, install.sh:339 and
// xray-hotspot-fixed.sh:253.
func TestNoGoogleAnywhere(t *testing.T) {
	conf, err := RenderDnsmasq(testDNS())
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	hconf, err := RenderHostapd(testAP())
	if err != nil {
		t.Fatalf("RenderHostapd: %v", err)
	}
	for _, needle := range []string{"8.8.8.8", "8.8.4.4", "2001:4860:4860", "dns.google", "google"} {
		if strings.Contains(strings.ToLower(conf), needle) {
			t.Errorf("%q appears in the generated dnsmasq configuration", needle)
		}
		if strings.Contains(strings.ToLower(hconf), needle) {
			t.Errorf("%q appears in the generated hostapd configuration", needle)
		}
	}

	// And it cannot be introduced through the struct either.
	for _, addr := range []string{"8.8.8.8:53", "8.8.4.4:53", "[2001:4860:4860::8888]:53"} {
		cfg := testDNS()
		cfg.Upstream = netip.MustParseAddrPort(addr)
		if _, err := RenderDnsmasq(cfg); err == nil {
			t.Errorf("%s was accepted as the client DNS forwarding target", addr)
		}
	}
}

func TestPublicResolverIsRefused(t *testing.T) {
	// Not just Google: anything that is not on this machine is a plaintext
	// query leaving beside the tunnel.
	for _, addr := range []string{"1.1.1.1:53", "9.9.9.9:53", "192.168.66.1:53", "[2606:4700:4700::1111]:53"} {
		cfg := testDNS()
		cfg.Upstream = netip.MustParseAddrPort(addr)
		if _, err := RenderDnsmasq(cfg); err == nil {
			t.Errorf("%s was accepted as the client DNS forwarding target", addr)
		}
	}
	// Loopback is accepted, on either family.
	for _, addr := range []string{"127.0.0.1:5354", "127.0.0.53:53", "[::1]:5354"} {
		cfg := testDNS()
		cfg.Upstream = netip.MustParseAddrPort(addr)
		if _, err := RenderDnsmasq(cfg); err != nil {
			t.Errorf("%s was refused as the client DNS forwarding target: %v", addr, err)
		}
	}
}

func TestDNSConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DNSConfig)
		want   string
	}{
		{"gateway outside subnet", func(c *DNSConfig) {
			c.Gateway = netip.MustParseAddr("10.0.0.1")
		}, "not inside the hotspot subnet"},
		{"range runs backwards", func(c *DNSConfig) {
			c.RangeStart, c.RangeEnd = c.RangeEnd, c.RangeStart
		}, "runs backwards"},
		{"gateway inside the pool", func(c *DNSConfig) {
			c.RangeStart = netip.MustParseAddr("192.168.66.1")
		}, "would be given away"},
		{"lease shorter than the dnsmasq minimum", func(c *DNSConfig) {
			c.LeaseTime = 30 * time.Second
		}, "minimum"},
		{"relative lease file path", func(c *DNSConfig) {
			c.LeaseFile = "leases"
		}, "absolute path"},
		{"subnet given as a host address", func(c *DNSConfig) {
			c.Subnet = netip.MustParsePrefix("192.168.66.1/24")
		}, "host address"},
		{"no interface", func(c *DNSConfig) { c.Interface = "" }, "no interface"},
		{"interface with a newline", func(c *DNSConfig) {
			c.Interface = "wlan0\nserver=8.8.8.8"
		}, "not allowed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testDNS()
			tc.mutate(&cfg)
			_, err := RenderDnsmasq(cfg)
			if err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestFilterAAAAIsOffUnlessAsked(t *testing.T) {
	got, err := RenderDnsmasq(testDNS())
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	if containsDirective(got, "filter-AAAA") {
		t.Error("filter-AAAA was emitted without being asked for; it needs dnsmasq 2.81 " +
			"and an older dnsmasq refuses to start on an unknown option")
	}

	cfg := testDNS()
	cfg.FilterAAAA = true
	got, err = RenderDnsmasq(cfg)
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	if !containsDirective(got, "filter-AAAA") {
		t.Error("filter-AAAA was asked for and not emitted")
	}
}

func TestLeaseTimeString(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{12 * time.Hour, "12h"},
		{time.Hour, "1h"},
		{90 * time.Minute, "90m"},
		{2 * time.Minute, "2m"},
		{150 * time.Second, "150"},
	}
	for _, tc := range tests {
		if got := leaseTimeString(tc.d); got != tc.want {
			t.Errorf("leaseTimeString(%s) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestIPv4Netmask(t *testing.T) {
	tests := []struct {
		bits int
		want string
	}{
		{24, "255.255.255.0"},
		{16, "255.255.0.0"},
		{25, "255.255.255.128"},
		{30, "255.255.255.252"},
	}
	for _, tc := range tests {
		if got := ipv4Netmask(tc.bits); got != tc.want {
			t.Errorf("ipv4Netmask(%d) = %q, want %q", tc.bits, got, tc.want)
		}
	}
}

// TestDnsmasqAlwaysNamesTheUser is the regression test for a defect a golden
// file cannot catch, because the defect was an ABSENT directive rather than a
// wrong one: the generated configuration named no user, so dnsmasq dropped to
// its own compiled-in default account, could not traverse the 0700
// service-user-owned directory holding the lease file, and recorded no leases.
//
// The symptom was silence. The hotspot worked, DHCP worked, nothing logged an
// error, and the panel reported zero connected devices for ever.
func TestDnsmasqAlwaysNamesTheUser(t *testing.T) {
	got, err := RenderDnsmasq(testDNS())
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	if !containsDirective(got, "user="+DefaultServiceUser) {
		t.Errorf("the generated configuration names no user; dnsmasq would drop to its own "+
			"default account and silently fail to write %s", testDNS().LeaseFile)
	}
	if !containsDirective(got, "group="+DefaultServiceGroup) {
		t.Error("the generated configuration names no group; user= does not imply it")
	}

	// It has to follow the parameter, or the installer and the configuration
	// can disagree about who owns the lease directory.
	cfg := testDNS()
	cfg.ServiceUser = "someoneelse"
	cfg.ServiceGroup = "someothergroup"
	got, err = RenderDnsmasq(cfg)
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	if !containsDirective(got, "user=someoneelse") || !containsDirective(got, "group=someothergroup") {
		t.Error("the account is hardcoded rather than taken from the parameter")
	}
	if containsDirective(got, "user="+DefaultServiceUser) {
		t.Error("the default account was emitted alongside the one that was asked for")
	}
}

// TestDnsmasqRequiredDirectives guards the whole absent-directive class, which
// is the class this package has now been bitten by twice: once by the query
// logging directives and once by the missing user. A golden file proves the
// bytes have not changed; it cannot say that a line which was never there
// should have been. This list can.
//
// Every entry is load-bearing. Add to it rather than relying on the golden.
func TestDnsmasqRequiredDirectives(t *testing.T) {
	got, err := RenderDnsmasq(testDNS())
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}
	required := map[string]string{
		"user=":              "dnsmasq drops to its own default account and cannot write the lease file",
		"group=":             "half the process identity is left unstated",
		"interface=":         "dnsmasq would serve on every interface",
		"bind-interfaces":    "dnsmasq binds the wildcard address and answers on the uplink",
		"listen-address=":    "no address is pinned",
		"no-resolv":          "dnsmasq inherits the uplink resolver and client queries leak past the tunnel",
		"server=":            "there is no forwarding target",
		"domain-needed":      "bare names are sent to a stranger",
		"bogus-priv":         "reverse lookups for the hotspot's own devices are sent to a stranger",
		"no-hosts":           "the box's /etc/hosts is served to clients",
		"quiet-dhcp":         "dnsmasq logs every DHCP transaction by default",
		"quiet-dhcp6":        "dnsmasq logs every DHCPv6 transaction by default",
		"quiet-ra":           "dnsmasq logs router advertisements by default",
		"dhcp-authoritative": "a client renewing a lease from another network waits for a timeout",
		"dhcp-range=":        "no addresses are handed out at all",
		"dhcp-leasefile=":    "the panel has no source for the connected-device count",
		"cache-size=":        "the cache size is left to a default",
		"port=":              "the DNS port is left to a default",
	}
	for directive, consequence := range required {
		if !containsDirective(got, directive) {
			t.Errorf("%q is missing from the generated dnsmasq configuration: %s", directive, consequence)
		}
	}
}

// TestLeaseFileIsWritableByTheDeclaredUser ties the two halves together. The
// defect was not that user= was missing in the abstract; it was that the lease
// path pointed somewhere only the service user can reach while the process was
// running as somebody else. A config that names a lease file outside the
// directories the declared account owns is the same defect wearing a different
// path.
func TestLeaseFileIsWritableByTheDeclaredUser(t *testing.T) {
	cfg := testDNS()
	got, err := RenderDnsmasq(cfg)
	if err != nil {
		t.Fatalf("RenderDnsmasq: %v", err)
	}

	leaseDir := path.Dir(cfg.LeaseFile)
	// docs/LAYOUT.md: /var/lib/caspian is mode 0700 owned by the service user,
	// so only a process running as that user can write inside it.
	if leaseDir != "/var/lib/caspian" {
		t.Fatalf("the lease file moved to %s; check docs/LAYOUT.md for its mode and owner "+
			"and update this test with the answer", leaseDir)
	}
	if !containsDirective(got, "user="+cfg.ServiceUser) {
		t.Fatalf("the lease file is in %s, which only %s can write, but the configuration "+
			"does not tell dnsmasq to run as %s", leaseDir, cfg.ServiceUser, cfg.ServiceUser)
	}
	if cfg.ServiceUser != DefaultServiceUser {
		t.Errorf("the lease directory is owned by %s per docs/LAYOUT.md but the configuration "+
			"runs dnsmasq as %s", DefaultServiceUser, cfg.ServiceUser)
	}
}

func TestServiceAccountValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		user string
	}{
		{"empty", ""},
		{"newline injection", "caspian\nuser=root"},
		{"leading hyphen reads as an option", "-caspian"},
		{"space", "cas pian"},
		{"too long", strings.Repeat("a", 33)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testDNS()
			cfg.ServiceUser = tc.user
			if _, err := RenderDnsmasq(cfg); err == nil {
				t.Errorf("service user %q was accepted", tc.user)
			}
			cfg = testDNS()
			cfg.ServiceGroup = tc.user
			if _, err := RenderDnsmasq(cfg); err == nil {
				t.Errorf("service group %q was accepted", tc.user)
			}
		})
	}
}

// TestNewPlanSuppliesTheDocumentedAccount: a caller that does not care still
// gets the account the installer creates, so the two cannot drift apart.
func TestNewPlanSuppliesTheDocumentedAccount(t *testing.T) {
	dns := testDNS()
	dns.ServiceUser = ""
	dns.ServiceGroup = ""

	plan, err := NewPlan(testAP(), dns, RadioConstraint{
		SupportsAP: true, MaxAPs: 1, MaxChannels: 1, ClientChannel: 10,
	})
	if err != nil {
		t.Fatalf("NewPlan: %v", err)
	}
	if plan.DNS.ServiceUser != DefaultServiceUser || plan.DNS.ServiceGroup != DefaultServiceGroup {
		t.Fatalf("NewPlan left the account as %q/%q", plan.DNS.ServiceUser, plan.DNS.ServiceGroup)
	}
	if !containsDirective(plan.DnsmasqConf, "user="+DefaultServiceUser) {
		t.Error("the plan's rendered configuration does not name the account")
	}
}
