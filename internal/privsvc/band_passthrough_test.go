// SPDX-License-Identifier: AGPL-3.0-or-later

package privsvc

import (
	"os"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// The band the user chose has to reach the PLANNER, because the planner is the
// only thing that knows which channels the radio has.
//
// This used to be applied after the plan instead, by overwriting the band that
// the chosen channel implied. The two never had to agree, and when they did not
// hostapd was handed a contradiction: a 2.4GHz channel labelled 5GHz, which
// cannot work. It surfaced as "the hotspot failed" with nothing anywhere
// pointing at the band, and the user's explicit choice was silently replaced.
//
// Dropping the assignment failed NO test when this guard was written, which is
// the same silent shape the fix exists to remove: nothing is missing from the
// document, no type changes, and the box just quietly does something else.
func TestTheChosenBandReachesThePlanner(t *testing.T) {
	w := newWorld(t)
	req := startRequest(t)

	for _, band := range []string{"", "2.4GHz", "5GHz"} {
		t.Run("band="+bandName(band), func(t *testing.T) {
			req.Hotspot.Band = band
			opts, err := w.svc.netOptionsFor(req)
			if err != nil {
				t.Fatalf("netOptionsFor: %v", err)
			}
			if string(opts.HotspotBand) != band {
				t.Errorf("the planner was given band %q while the user asked for %q. "+
					"The planner picks the channel, so a band it never sees cannot "+
					"constrain that choice, and whatever is applied afterwards is a "+
					"label rather than a decision.",
					opts.HotspotBand, band)
			}
		})
	}
}

// And the other half: nothing downstream may re-decide the band once the
// planner has chosen a channel in it. The band must be derived from the
// channel, so the two cannot disagree by construction.
func TestTheBandIsNotOverriddenAfterThePlan(t *testing.T) {
	b, err := os.ReadFile("plans.go")
	if err != nil {
		t.Fatalf("read plans.go: %v", err)
	}
	if strings.Contains(string(b), "band = hotspot.Band(req.Hotspot.Band)") {
		t.Error("the band is being overwritten after the plan chose a channel. " +
			"That is how a 2.4GHz channel came to be labelled 5GHz. Let " +
			"bandForChannel derive it, and let netcfg refuse when the radio " +
			"has no channel in the band that was asked for.")
	}
}

func bandName(b string) string {
	if b == "" {
		return "auto"
	}
	return b
}

var _ = netcfg.BandAuto
var _ = panel.HotspotSpec{}
