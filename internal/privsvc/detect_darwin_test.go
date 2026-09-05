package privsvc

import (
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
	"testing"
	"time"
)

func TestDarwinPortalUsesSharingBridgeNotWiFiClientAddress(t *testing.T) {
	f := netcfg.Facts{Links: netcfg.ParseIfconfig("en0: flags=8863<UP> mtu 1500\n\tinet 10.0.0.17 netmask 0xffffff00\nbridge100: flags=8a63<UP> mtu 1500\n\tinet 10.83.51.1 netmask 0xffffff00\n")}
	p := &netcfg.Plan{Platform: netcfg.PlatformDarwin, Hotspot: "en0"}
	if got := detectionFrom(f, p, "", panel.FaultNone, time.Now()).HotspotAddress; got != "10.83.51.1" {
		t.Fatalf("portal on wrong network: %s", got)
	}
	f.Links = f.Links[:1]
	if got := detectionFrom(f, p, "", panel.FaultNone, time.Now()).HotspotAddress; got != "" {
		t.Fatalf("absent bridge exposed Wi-Fi client address: %s", got)
	}
	p.Platform = netcfg.PlatformLinux
	if got := detectionFrom(f, p, "", panel.FaultNone, time.Now()).HotspotAddress; got != "10.0.0.17" {
		t.Fatalf("Linux behavior changed: %s", got)
	}
}
