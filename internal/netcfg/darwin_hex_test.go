package netcfg

import "testing"

func TestDarwinHexFlagsKeepBridgeAddressOnBridge(t *testing.T) {
	links := ParseIfconfig("utun3: flags=8051<UP,POINTOPOINT> mtu 1380\nbridge100: flags=8a63<UP,BROADCAST,RUNNING> mtu 1500\n\tinet 10.83.51.1 netmask 0xffffff00\nap1: flags=8b43<UP,BROADCAST,RUNNING> mtu 1500\n")
	if len(links) != 3 || len(links[0].Prefixes) != 0 || links[1].Name != "bridge100" || links[1].State != "UP" || len(links[1].Prefixes) != 1 || links[1].Prefixes[0].String() != "10.83.51.1/24" {
		t.Fatalf("bridge address misattributed: %+v", links)
	}
}
