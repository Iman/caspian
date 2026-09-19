// SPDX-License-Identifier: AGPL-3.0-or-later
package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsStateReplacementWaitsForReader(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, "new"), filepath.Join(dir, "state")
	if err := os.WriteFile(from, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(to)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	done := make(chan error, 1)
	go func() { done <- replaceStateFile(from, to) }()
	select {
	case err := <-done:
		t.Fatalf("replacement finished while the reader held the file: %v", err)
	case <-time.After(60 * time.Millisecond):
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(to)
	if err != nil || string(got) != "new" {
		t.Fatalf("saved = %q, %v", got, err)
	}
}

func TestWindowsStateReplacementPreservesLockedFile(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, "new"), filepath.Join(dir, "state")
	if err := os.WriteFile(from, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(to)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := replaceStateFile(from, to); err == nil {
		t.Fatal("replaced a file held by a reader")
	}
	got, err := os.ReadFile(to)
	if err != nil || string(got) != "old" {
		t.Fatalf("old state = %q, %v", got, err)
	}
	if err := replaceStateFile(filepath.Join(dir, "absent"), to); !os.IsNotExist(err) {
		t.Fatalf("missing source = %v", err)
	}
}
