// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"errors"
	"testing"

	"caspianbyoc.org/caspian/internal/panel"
)

// TestAnEngineStartFailureCrossesTheSocketAsTheRightWord pins the words the
// panel receives for the engine refusals that have a remedy of their own. The
// texts are the ones xray-core produced on 2026-09-12: the Windows bind failure
// from issue #2, and the two removed features a survey of public share links
// turned up. Anything else is still "the engine rejected the config".
func TestAnEngineStartFailureCrossesTheSocketAsTheRightWord(t *testing.T) {
	cases := []struct {
		name string
		text string
		want panel.Fault
	}{
		{"another program holds the port", "start: app/proxyman/inbound: failed to listen TCP on 10808 > transport/internet: failed to listen on address: 127.0.0.1:10808 > listen tcp 127.0.0.1:10808: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted.", panel.FaultPortInUse},
		{"the link asks to skip the certificate check", `start: infra/conf: Failed to build TLS config. > common/errors: The feature "allowInsecure" has been removed and migrated to "pinnedPeerCertSha256".`, panel.FaultInsecureRemoved},
		{"a Shadowsocks method the engine dropped", "start: infra/conf: failed to build outbound handler for protocol shadowsocks > infra/conf: unknown cipher method: aes-256-cfb", panel.FaultCipherRemoved},
		{"anything else the engine refuses", "start: the engine refused this configuration", panel.FaultEngineRejectedConfig},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.eng.startErr = errors.New(c.text)
			err := w.svc.Start(context.Background(), startRequest(t))
			if err == nil {
				t.Fatal("a start whose engine refused reported success")
			}
			if got := faultOf(err); got != c.want {
				t.Errorf("fault = %q, want %q for: %s", got, c.want, c.text)
			}
		})
	}
}
