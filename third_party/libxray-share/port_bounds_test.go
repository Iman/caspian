package share

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"testing"
)

func TestShareLinkPortsStayWithinTCPUDPRange(t *testing.T) {
	users := map[string]string{
		"ss":    ssUserB64("aes-128-gcm", "fixture-password"),
		"vmess": testShareUUID, "vless": testShareUUID,
		"socks": ssUserB64("fixture-user", "fixture-password"), "trojan": "fixture-password", "hysteria2": "fixture-password",
	}
	for scheme, user := range users {
		for _, port := range []int{0, 1, 443, 65535, 65536, 70000} {
			t.Run(fmt.Sprintf("%s/%d", scheme, port), func(t *testing.T) {
				raw := scheme + "://" + user + "@192.0.2.1:" + strconv.Itoa(port)
				u, err := url.Parse(raw)
				if err != nil {
					t.Fatal(err)
				}
				ob, err := (xrayShareLink{link: u, rawText: raw}).outbound()
				if port > 65535 {
					if err == nil {
						t.Fatalf("accepted out-of-range port %d", port)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				var settings struct {
					Port uint16 `json:"port"`
				}
				if err = json.Unmarshal(*ob.Settings, &settings); err != nil {
					t.Fatal(err)
				}
				if int(settings.Port) != port {
					t.Fatalf("port changed: %d -> %d", port, settings.Port)
				}
			})
		}
	}
}

func TestVMessQRPortsStayWithinTCPUDPRange(t *testing.T) {
	for _, port := range []any{-1, 0, 1, 443, 65535, 65536, 70000, "-1", "0", "1", "65535", "65536", "70000"} {
		t.Run(fmt.Sprintf("%T-%v", port, port), func(t *testing.T) {
			p, err := strconv.Atoi(fmt.Sprint(port))
			if err != nil {
				t.Fatal(err)
			}
			ob, err := (vmessQrCode{Port: port, Add: "192.0.2.1", Id: testShareUUID}).outbound()
			if p < 0 || p > 65535 {
				if err == nil {
					t.Fatalf("accepted out-of-range port %v", port)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var settings struct {
				Port uint16 `json:"port"`
			}
			if err = json.Unmarshal(*ob.Settings, &settings); err != nil {
				t.Fatal(err)
			}
			if int(settings.Port) != p {
				t.Fatalf("port changed: %d -> %d", p, settings.Port)
			}
		})
	}
}
