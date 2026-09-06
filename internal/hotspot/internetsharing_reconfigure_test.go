// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInternetSharingChangedPasswordRequiresOldBridgeToStop(t *testing.T) {
	for _, fault := range []string{"", "write", "notify", "stuck"} {
		t.Run(fault, func(t *testing.T) {
			m := &macResponder{radioOn: true, bridgeUp: true, bridgeGW: "10.83.51.1"}
			s, rec := newMac(t, m)
			plan := macPlan(t)
			rec.Files[s.paths.NATPrefs] = []byte(renderNATPrefs(plan, "AAAA-1111", true))
			plan.AP.Passphrase = "replacement-password"
			var transitions []bool
			rec.Responder = func(r *Recorder, name string, args []string) (Result, error) {
				if strings.HasSuffix(name, "scutil") {
					transitions = append(transitions, strings.Contains(string(r.Files[s.paths.NATPrefs]), "<integer>0</integer>\n\t\t\t<key>NetworkName"))
					if fault == "stuck" {
						return Result{}, nil
					}
				}
				return m.respond(r, name, args)
			}
			if fault == "write" {
				rec.WriteErr = errors.New("disk failure")
			}
			m.scutilErr = fault == "notify"
			st, err := s.Start(context.Background(), plan)
			if fault != "" {
				if err == nil || st.Running {
					t.Fatal("reported success with old credentials still active")
				}
				return
			}
			if err != nil || !st.Running {
				t.Fatalf("start: %v", err)
			}
			if len(transitions) != 2 || !transitions[0] || transitions[1] {
				t.Fatalf("expected an off transition before enabling new credentials: %v", transitions)
			}
			if !strings.Contains(string(rec.Files[s.paths.NATPrefs]), utf16LEBase64(plan.AP.Passphrase)) {
				t.Fatal("new password not written")
			}
		})
	}
}

func TestInternetSharingStopReportsRejectedPreferences(t *testing.T) {
	m := &macResponder{radioOn: true, bridgeGW: "10.83.51.1"}
	s, _ := newMac(t, m)
	if _, err := s.Start(context.Background(), macPlan(t)); err != nil {
		t.Fatal(err)
	}
	m.scutilErr = true
	if err := s.Stop(context.Background()); err == nil {
		t.Fatal("stop hid the failed configd notification")
	}
}

func TestInternetSharingStopDoesNotRewriteUnknownEnabledValues(t *testing.T) {
	for _, xml := range []string{
		"<key>NAT</key><dict><key>AirPort</key><dict><key>Enabled</key><integer>1</integer></dict></dict>",
		"<key>NAT</key><dict><key>Enabled</key><string>unknown</string></dict>",
		"<key>NAT</key><dict>",
	} {
		if got := disableInNATPrefs(xml); got != xml {
			t.Fatal("rewrote an unrelated or unknown setting")
		}
	}
}
