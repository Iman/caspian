// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import (
	"os"
	"strings"
	"testing"
)

// The tests in this file were written BEFORE ProxyConfig.Selected and
// Store.SelectProxyEntry existed, on 2026-09-08. Their first run was a compile
// failure naming both; that is the "red" recorded in the report.

// TestSelectProxyEntryPersistsAndLeavesTheConfigAlone: the selection survives a
// reload, and the credential it selects from is untouched byte for byte.
func TestSelectProxyEntryPersistsAndLeavesTheConfigAlone(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
		t.Fatalf("SetProxyConfig: %v", err)
	}
	before := st.Proxy()
	if before.Selected != 0 {
		t.Fatalf("a freshly stored config selects entry %d, want 0", before.Selected)
	}

	if err := st.SelectProxyEntry(2); err != nil {
		t.Fatalf("SelectProxyEntry(2): %v", err)
	}
	after := st.Proxy()
	if after.Selected != 2 {
		t.Errorf("Selected = %d after SelectProxyEntry(2)", after.Selected)
	}
	if after.Raw.Reveal() != before.Raw.Reveal() || after.Fingerprint() != before.Fingerprint() {
		t.Error("SelectProxyEntry changed the stored config")
	}
	if after.Scheme != before.Scheme || after.Label != before.Label || !after.AddedAt.Equal(before.AddedAt) {
		t.Error("SelectProxyEntry changed a field other than Selected")
	}

	again, err := Load(dir)
	if err != nil {
		t.Fatalf("Load again: %v", err)
	}
	if got := again.Proxy().Selected; got != 2 {
		t.Errorf("Selected = %d after a reload, want 2", got)
	}
	if again.Proxy().Raw.Reveal() != fakeProxyLink {
		t.Error("the config did not survive the reload")
	}
}

// TestSelectProxyEntryRefusesANegativeIndex: the store owns "never negative",
// because a negative index is not an entry and nothing downstream should have
// to defend against it. The refusal names the field and touches neither the
// published state nor the file.
func TestSelectProxyEntryRefusesANegativeIndex(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, fakeProxyLabel); err != nil {
		t.Fatalf("SetProxyConfig: %v", err)
	}
	if err := st.SelectProxyEntry(1); err != nil {
		t.Fatalf("SelectProxyEntry(1): %v", err)
	}
	fileBefore, err := os.ReadFile(st.Path())
	if err != nil {
		t.Fatalf("reading the state file: %v", err)
	}

	for _, i := range []int{-1, -100} {
		err := st.SelectProxyEntry(i)
		if err == nil {
			t.Fatalf("SelectProxyEntry(%d) accepted a negative index", i)
		}
		if !strings.Contains(err.Error(), "entry") {
			t.Errorf("the error does not name the field: %v", err)
		}
		if strings.Contains(err.Error(), fakeProxyLink) {
			t.Errorf("the error quotes the stored config: %v", err)
		}
	}
	if got := st.Proxy().Selected; got != 1 {
		t.Errorf("a refused selection changed Selected to %d", got)
	}
	fileAfter, err := os.ReadFile(st.Path())
	if err != nil {
		t.Fatalf("reading the state file: %v", err)
	}
	if string(fileAfter) != string(fileBefore) {
		t.Error("a refused selection rewrote the state file")
	}
}

// TestSelectProxyEntryRefusesWithoutAConfig: there is nothing to select from,
// and recording a selection anyway would make the next paste's reset look like
// it did something.
func TestSelectProxyEntryRefusesWithoutAConfig(t *testing.T) {
	st, err := Load(tempStateDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SelectProxyEntry(0); err == nil {
		t.Fatal("SelectProxyEntry accepted a selection with no config stored")
	}
	if st.Proxy().Selected != 0 {
		t.Error("a refused selection was published")
	}
}

// TestSetProxyConfigResetsTheSelection is acceptance item 3 of the proposal: a
// new paste is a new list, and an index into the old one means nothing in it.
func TestSetProxyConfigResetsTheSelection(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink, fakeProxyScheme, "first"); err != nil {
		t.Fatalf("SetProxyConfig: %v", err)
	}
	if err := st.SelectProxyEntry(2); err != nil {
		t.Fatalf("SelectProxyEntry: %v", err)
	}
	if err := st.SetProxyConfig(fakeProxyLink+"\n"+fakeProxyLink, fakeProxyScheme, "second"); err != nil {
		t.Fatalf("SetProxyConfig again: %v", err)
	}
	if got := st.Proxy().Selected; got != 0 {
		t.Errorf("Selected = %d after a re-paste, want 0", got)
	}
	again, err := Load(dir)
	if err != nil {
		t.Fatalf("Load again: %v", err)
	}
	if got := again.Proxy().Selected; got != 0 {
		t.Errorf("Selected = %d on disk after a re-paste, want 0", got)
	}
}

// TestStateFileWithoutSelectedLoadsAsEntryZero is acceptance item 6. Every
// file written before this field existed lacks the key, and the absent key has
// to read as entry zero, which is what those files meant: the first entry was
// the only one anything could use. No schema bump is needed for that, because
// zero is the safe reading and the fail-closed policy fields are the only ones
// the version guards. The file is at CurrentVersion so no migration runs.
func TestStateFileWithoutSelectedLoadsAsEntryZero(t *testing.T) {
	dir := writeStateFile(t, `{"version":`+itoa(CurrentVersion)+`,
		"proxy":{"raw":"`+fakeProxyLink+`","scheme":"vless","label":"old"},
		"advanced":{"dns_mode":"tunnel","on_tunnel_down":"block","client_ipv6":"block"}}`)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := st.Proxy()
	if p.Selected != 0 {
		t.Errorf("Selected = %d from a file with no selected key, want 0", p.Selected)
	}
	if !p.IsConfigured() || p.Label != "old" {
		t.Error("the rest of the proxy config did not load")
	}
	if st.Snapshot().Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", st.Snapshot().Version, CurrentVersion)
	}
	// And a file that carries the key reads it.
	dir2 := writeStateFile(t, `{"version":`+itoa(CurrentVersion)+`,
		"proxy":{"raw":"`+fakeProxyLink+`","selected":3},
		"advanced":{"dns_mode":"tunnel","on_tunnel_down":"block","client_ipv6":"block"}}`)
	st2, err := Load(dir2)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := st2.Proxy().Selected; got != 3 {
		t.Errorf("Selected = %d from a file carrying selected 3", got)
	}
}

// TestRedactedShowsTheSelection: diagnostics need to know which entry a box is
// on, and the number discloses nothing.
func TestRedactedShowsTheSelection(t *testing.T) {
	st := fullState(t)
	st.Proxy.Selected = 2
	if r := st.Redacted(); !strings.Contains(r, "proxy.selected=2") {
		t.Errorf("Redacted does not show the selection:\n%s", r)
	}
	// Not shown when there is no config: there is nothing it could index.
	empty := defaultState()
	if r := empty.Redacted(); strings.Contains(r, "proxy.selected") {
		t.Errorf("Redacted shows a selection for a box with no config:\n%s", r)
	}
}
