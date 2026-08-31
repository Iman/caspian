// SPDX-License-Identifier: AGPL-3.0-or-later

package panel

import (
	"net/url"
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/state"
)

// The client IPv6 policy reaches the privileged side, and it comes from the
// stored state rather than from a constant here.
//
// This is the pairing that would otherwise break quietly. The privileged side
// refuses any value but the blocking one, so a panel that forgot to copy the
// field would send empty and every start would fail with a fault the user
// cannot act on. The two policy fields beside it are copied the same way and
// have the same failure mode.
//
// The second case stores a value this build does not support on purpose. The
// panel's job is to report what is stored, not to correct it; substituting the
// safe value here would hide a setting the user changed and turn the refusal
// further down into a lie.
func TestStartRequestCarriesTheStoredClientIPv6Policy(t *testing.T) {
	for _, stored := range []string{state.ClientIPv6Block, "some-future-value"} {
		t.Run(stored, func(t *testing.T) {
			h := newHarness(t)
			h.ready()
			if err := h.store.Update(func(st *state.State) error {
				st.Advanced.ClientIPv6 = stored
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			if res, _ := h.postForm("/power", url.Values{"csrf": {h.tokenOn("/")}, "on": {"1"}}); res.StatusCode != 303 {
				t.Fatal("could not switch on")
			}
			starts := h.priv.Starts()
			if len(starts) != 1 {
				t.Fatalf("%d start requests, want 1", len(starts))
			}
			if got := starts[0].Network.ClientIPv6; got != stored {
				t.Errorf("the request carried ClientIPv6 = %q, want the stored %q", got, stored)
			}
		})
	}
}

// The refusal has a fault of its own, so the panel can say what is wrong
// instead of showing the "something went wrong and Caspian could not work out
// what" sentence.
//
// FaultUnknown was the alternative and it is the wrong one here: this is not an
// unknown failure, it is a setting this build does not support, and a user who
// turned it on in advanced mode needs to be told that rather than left to guess
// at their config.
func TestTheIPv6FaultHasItsOwnWordsInBothLanguages(t *testing.T) {
	if FaultIPv6Unsupported == FaultUnknown || FaultIPv6Unsupported == FaultNone {
		t.Fatal("the IPv6 fault is not a fault of its own")
	}
	key := FaultIPv6Unsupported.Key()
	if key == MsgFaultUnknown || key == MsgFaultUnrecognised {
		t.Errorf("FaultIPv6Unsupported.Key() = %q, which is the generic message", key)
	}
	for _, lang := range Langs {
		got := T(lang, key)
		if strings.Contains(got, missingMarker) {
			t.Errorf("%s: the IPv6 fault renders as %q", lang, got)
		}
	}

	// And it is in the list the catalogue tests walk. A fault missing from
	// that slice has no message check at all, which is how a fault ships
	// rendering as a marker in one language.
	found := false
	for _, f := range faults {
		if f == FaultIPv6Unsupported {
			found = true
		}
	}
	if !found {
		t.Error("FaultIPv6Unsupported is not in the faults slice, so no catalogue test covers it")
	}
}
