package hotspot

import (
	"strings"
	"testing"
)

func TestDisableSharingDoesNotChangeNestedEnabled(t *testing.T) {
	input := `<dict><key>NAT</key><dict><key>AirPort</key><dict><key>Enabled</key><integer>1</integer></dict><key>Enabled</key><integer>1</integer><key>PrimaryInterface</key><dict><key>Enabled</key><integer>0</integer></dict></dict></dict>`
	got := disableInNATPrefs(input)
	if !strings.Contains(got, `</dict><key>Enabled</key><integer>0</integer>`) || !strings.Contains(got, `<key>AirPort</key><dict><key>Enabled</key><integer>1</integer>`) {
		t.Fatalf("wrong Enabled key changed: %s", got)
	}
}

func TestSharingRecognizesLiveMacHexFlags(t *testing.T) {
	up, addrs := ParseIfconfigBrief("bridge100: flags=8a63<UP,BROADCAST,SMART,RUNNING> mtu 1500\n\tinet 10.83.51.1 netmask 0xffffff00\n")
	if !up || len(addrs) != 1 || addrs[0].String() != "10.83.51.1" {
		t.Fatalf("live bridge was missed: up=%v addresses=%v", up, addrs)
	}
}
