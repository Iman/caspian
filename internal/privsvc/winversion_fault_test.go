// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"errors"
	"fmt"
	"testing"

	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
)

// A Windows older than version 2004 cannot point DNS at the tunnel adapter,
// so internal/netcfg refuses to apply a plan on it. These tests hold the
// classification of that refusal, which is what decides the sentence the
// person reads and therefore what they do next.
//
// The wrong answer here is not a crash, it is a plausible sentence that sends
// somebody to the wrong remedy: "restart the box" and "run the installer
// again" both fail forever on a build that will never gain the call.

// TestAnOldWindowsIsItsOwnFault is the happy path of the classifier: the
// refusal keeps its own word all the way through the wrapping the apply path
// puts around it.
func TestAnOldWindowsIsItsOwnFault(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"as returned", netcfg.ErrWindowsTooOld},
		{"wrapped once, as the runner wraps it with the build number",
			fmt.Errorf("%w (build %d)", netcfg.ErrWindowsTooOld, 17763)},
		{"wrapped twice, as a step and then a plan wrap it",
			fmt.Errorf("apply step 4: %w", fmt.Errorf("iphlpapi dns set: %w", netcfg.ErrWindowsTooOld))},
		{"carried by the package's own fault error",
			fail("apply", panel.FaultWindowsTooOld, netcfg.ErrWindowsTooOld)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := faultOf(tc.err); got != panel.FaultWindowsTooOld {
				t.Errorf("faultOf = %q, want %q", got, panel.FaultWindowsTooOld)
			}
		})
	}
}

// TestAnOldWindowsIsNotReportedAsSomethingInstallable is the unhappy path
// this classification exists to prevent. Both of these faults tell the person
// to install or restart something, and neither can ever help here.
func TestAnOldWindowsIsNotReportedAsSomethingInstallable(t *testing.T) {
	got := faultOf(netcfg.ErrWindowsTooOld)
	for _, wrong := range []panel.Fault{
		panel.FaultSoftwareMissing,
		panel.FaultUnavailable,
		panel.FaultHotspotFailed,
		panel.FaultUnknown,
	} {
		if got == wrong {
			t.Errorf("an old Windows is reported as %q, which sends the person to a remedy that cannot work", wrong)
		}
	}
}

// TestTheOldWindowsRefusalIsDistinctFromAnUnsupportedPlatform guards the pair
// that is easiest to confuse. One means Caspian does not run on this kind of
// machine at all; the other means it runs on this kind of machine and this
// copy is too old. They lead to different sentences and different actions.
func TestTheOldWindowsRefusalIsDistinctFromAnUnsupportedPlatform(t *testing.T) {
	if errors.Is(netcfg.ErrWindowsTooOld, netcfg.ErrUnsupportedPlatform) {
		t.Fatal("the old-Windows refusal is an unsupported-platform error, so the two cannot be told apart")
	}
	if faultOf(netcfg.ErrUnsupportedPlatform) == panel.FaultWindowsTooOld {
		t.Error("an unsupported platform is reported as an old Windows")
	}
	if faultOf(netcfg.ErrWindowsTooOld) == faultOf(netcfg.ErrUnsupportedPlatform) {
		t.Error("the two refusals produce the same fault")
	}
}
