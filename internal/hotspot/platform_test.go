// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

import (
	"runtime"
	"strings"
	"testing"
)

// TestNewSystemRunnerIsGatedToLinux documents the build-tag split: the pure
// half of this package works everywhere so the configuration can be generated
// and checked during development, and only starting the processes is Linux.
func TestNewSystemRunnerIsGatedToLinux(t *testing.T) {
	sys, err := NewSystemRunner()
	if runtime.GOOS == "linux" {
		if err != nil || sys == nil {
			t.Fatalf("NewSystemRunner on linux returned %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("NewSystemRunner on %s returned a runner", runtime.GOOS)
	}
	// The message is one a person can read, since it reaches the panel.
	if !strings.Contains(err.Error(), "Linux") {
		t.Errorf("the message does not say what the requirement is: %v", err)
	}
}
