// SPDX-License-Identifier: AGPL-3.0-or-later
package privsvc

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/netcfg"
)

func TestDNSRedirectUsesEnginePortOnlyOnDarwin(t *testing.T) {
	for _, platform := range []netcfg.Platform{netcfg.PlatformDarwin, netcfg.PlatformLinux, netcfg.PlatformWindows} {
		for _, enginePort := range []uint16{5354, 15354} {
			t.Run(fmt.Sprintf("%s/%d", platform, enginePort), func(t *testing.T) {
				cfg := Config{Backend: netcfg.BackendFor(platform), DNSPort: 53, LocalDNSPort: enginePort, PanelPort: 8088}
				opts := cfg.netOptions()
				want := 53
				if platform == netcfg.PlatformDarwin {
					want = int(enginePort)
				}
				if opts.DNSPort != want {
					t.Fatalf("DNS redirected to %d, want %d", opts.DNSPort, want)
				}
				if platform == netcfg.PlatformDarwin {
					p := &netcfg.Plan{Platform: platform, Opts: opts, HotspotSubnet: netip.MustParsePrefix("10.83.51.0/24"), HotspotGateway: netip.MustParseAddr("10.83.51.1"), Tun: "utun100", Uplink: "en6"}
					for _, rule := range []string{p.PreEngineSteps(nil)[0].Do.Stdin, p.CutStep().Do.Stdin, p.RestoreStep().Do.Stdin} {
						if !strings.Contains(rule, fmt.Sprintf("port 53 -> 127.0.0.1 port %d", enginePort)) {
							t.Fatal("PF DNS target differs from the engine listener")
						}
					}
				}
			})
		}
	}
}
