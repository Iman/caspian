// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Required modes. The directory is 0700 and the file 0600, and both are
// enforced on load rather than merely set on save. See perm_unix.go.
const (
	dirMode  fs.FileMode = 0o700
	fileMode fs.FileMode = 0o600
)

// Store is the guarded, in-memory view of the state file, and the only writer
// of it.
//
// Concurrency, for the panel and the supervisor reading at once (requirement 6):
// reads take no lock at all. The current State hangs off an atomic pointer, so
// Snapshot is one atomic load plus a struct copy, and readers never contend
// with each other or block behind a save. Writers take mu, which serialises the
// read-modify-write in Update so two concurrent edits cannot lose one another.
//
// The State handed out is a copy, not a pointer, so a caller cannot mutate what
// other readers can see. That holds only because State contains no reference
// types; see the note on the State declaration.
//
// A Store is safe for concurrent use. The zero Store is not usable; call Load.
type Store struct {
	dir  string
	path string

	mu       sync.Mutex            // serialises writes only
	cur      atomic.Pointer[State] // read without a lock
	firstRun atomic.Bool

	// hookAfterTempWrite runs after the temporary file is written and synced
	// but before the rename. It is nil in every build except the tests that
	// need a write to fail at that exact point, which is the only way to prove
	// the previous file survives a failure and no temporary file is left
	// behind. It is a field rather than a package variable so tests that set it
	// cannot interfere with each other.
	hookAfterTempWrite func(tmpPath string) error
}

// Load reads the state file in dir and returns a Store.
//
// Nothing existing is not an error (requirement 7). A missing directory or a
// missing file yields a usable state carrying the fail-closed policy defaults,
// with FirstRun reporting true so the panel can show setup rather than a broken
// screen. Nothing is written to disk until the first Save or Update.
//
// A file that exists but cannot be read, parsed, or trusted IS an error. In
// particular a corrupt file is refused rather than replaced with defaults:
// silently resetting would throw away the user's proxy config, and they would
// have no way to tell that had happened.
func Load(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("state: Load needs a directory")
	}
	s := &Store{dir: dir, path: filepath.Join(dir, FileName)}

	di, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s.asFirstRun(), nil
	case err != nil:
		return nil, fmt.Errorf("state: examining %s: %w", dir, err)
	case !di.IsDir():
		return nil, fmt.Errorf("state: %s is not a directory", dir)
	}
	if err := checkPerms(dir, di, dirMode, "directory"); err != nil {
		return nil, err
	}

	fi, err := os.Stat(s.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s.asFirstRun(), nil
	case err != nil:
		return nil, fmt.Errorf("state: examining %s: %w", s.path, err)
	case fi.IsDir():
		return nil, fmt.Errorf("state: %s is a directory, not a state file", s.path)
	}
	if err := checkPerms(s.path, fi, fileMode, "file"); err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("state: reading %s: %w", s.path, err)
	}

	// Start from the defaults so that a field absent from an older file lands
	// on the safe value rather than on Go's zero value, whatever the migration
	// chain does afterwards.
	st := defaultState()
	// Version has to come from the file, not the default, or a v0 file would
	// look current and skip its migration.
	st.Version = 0
	st.Advanced.DNSMode = ""
	st.Advanced.OnTunnelDown = ""
	st.Advanced.ClientIPv6 = ""
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("state: %s is not valid JSON and has not been touched; move it aside to start over: %w", s.path, err)
	}
	if err := migrate(&st, s.path); err != nil {
		return nil, err
	}

	s.cur.Store(&st)
	s.firstRun.Store(false)
	return s, nil
}

func (s *Store) asFirstRun() *Store {
	st := defaultState()
	s.cur.Store(&st)
	s.firstRun.Store(true)
	return s
}

// Path is the state file this Store reads and writes.
func (s *Store) Path() string { return s.path }

// Dir is the directory holding the state file.
func (s *Store) Dir() string { return s.dir }

// FirstRun reports that no state file existed when Load ran. It stays true
// until the first successful write.
func (s *Store) FirstRun() bool { return s.firstRun.Load() }

// NeedsSetup reports that the panel cannot show its normal screen yet, because
// either no panel password has been chosen or no proxy config has been pasted.
// It is the question the panel actually has, and it stays right after a partial
// setup that FirstRun alone would call finished.
func (s *Store) NeedsSetup() bool {
	st := s.cur.Load()
	return !st.Panel.IsSet() || !st.Proxy.IsConfigured()
}

// Snapshot returns a copy of the whole state. It takes no lock.
func (s *Store) Snapshot() State { return *s.cur.Load() }

// Proxy returns a copy of the stored proxy config. The credential inside it is
// a Secret, so it will not print by accident.
func (s *Store) Proxy() ProxyConfig { return s.cur.Load().Proxy }

// Hotspot returns a copy of the stored hotspot settings.
func (s *Store) Hotspot() HotspotConfig { return s.cur.Load().Hotspot }

// Advanced returns a copy of the stored advanced-mode overrides. A zero value
// on a detection field means "not overridden"; see the Advanced declaration.
func (s *Store) Advanced() Advanced { return s.cur.Load().Advanced }

// Update applies fn to a copy of the state, persists the result atomically, and
// only then publishes it to readers. If fn or the write fails, nothing changes:
// neither the file nor what other goroutines can see.
//
// This is the only mutating entry point. Everything else in the package that
// writes goes through it, which is what makes "read, modify, write" a single
// serialised step rather than a race between the panel and the supervisor.
func (s *Store) Update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := *s.cur.Load() // whole-struct copy; see the State declaration
	if fn != nil {
		if err := fn(&next); err != nil {
			return err
		}
	}
	next.Version = CurrentVersion
	next.UpdatedAt = time.Now().UTC()
	if err := next.validate(); err != nil {
		return err
	}
	if err := s.writeAtomic(next); err != nil {
		return err
	}
	s.cur.Store(&next)
	s.firstRun.Store(false)
	return nil
}

// Save persists the current state unchanged. It is Update with no edit, and
// exists so a first run can commit its defaults.
func (s *Store) Save() error { return s.Update(nil) }

// SetProxyConfig stores a pasted config. raw is untrusted input (design section
// 6) and is stored verbatim; parsing and validating it is internal/xcfg's job,
// and scheme is whatever that produced.
func (s *Store) SetProxyConfig(raw, scheme, label string) error {
	return s.Update(func(st *State) error {
		if raw == "" {
			return errors.New("state: proxy config must not be empty")
		}
		st.Proxy.Raw = Secret(raw)
		st.Proxy.Scheme = scheme
		st.Proxy.Label = label
		st.Proxy.AddedAt = time.Now().UTC()
		return nil
	})
}

// SetHotspot stores the access point name and passphrase.
func (s *Store) SetHotspot(ssid, passphrase string) error {
	return s.Update(func(st *State) error {
		st.Hotspot.SSID = ssid
		st.Hotspot.Passphrase = Secret(passphrase)
		return nil
	})
}

// SetPanelPassword hashes plaintext and stores the verifier. The plaintext is
// never persisted, never logged and does not outlive this call.
func (s *Store) SetPanelPassword(plaintext string) error {
	hash, err := hashPassword(plaintext)
	if err != nil {
		return err
	}
	return s.Update(func(st *State) error {
		st.Panel.PasswordHash = Secret(hash)
		return nil
	})
}

// VerifyPanelPassword reports whether plaintext is the panel password.
//
// A wrong password is (false, nil). An error means the stored verifier could
// not be understood, or that no password has been set at all
// (ErrNoPanelPassword), which the panel has to tell apart from a refusal so it
// can show setup instead.
func (s *Store) VerifyPanelPassword(plaintext string) (bool, error) {
	stored := s.cur.Load().Panel.PasswordHash
	if stored.IsZero() {
		return false, ErrNoPanelPassword
	}
	return verifyPassword(stored.Reveal(), plaintext)
}

// validate enforces the invariants this package owns. It is deliberately short.
//
// The policy fields are checked because state owns the promise that neither can
// reach a downstream package empty; see the Advanced declaration. Hotspot and
// interface rules are not checked here: 802.11 constraints belong to
// internal/hotspot and internal/netcfg, and duplicating them would put two
// packages in disagreement about what is legal.
func (s State) validate() error {
	if s.Version != CurrentVersion {
		return fmt.Errorf("state: refusing to write schema version %d, this build writes %d", s.Version, CurrentVersion)
	}
	if s.Advanced.DNSMode == "" {
		return errors.New("state: refusing to write an empty DNS mode; empty must never be readable as 'let client traffic out'")
	}
	if s.Advanced.OnTunnelDown == "" {
		return errors.New("state: refusing to write an empty tunnel-down policy; empty must never be readable as 'let client traffic out'")
	}
	if s.Advanced.ClientIPv6 == "" {
		return errors.New("state: refusing to write an empty client IPv6 policy; empty must never be readable as 'let client IPv6 out', which bypasses the tunnel entirely")
	}
	return nil
}

// writeAtomic persists st so that a power cut can never produce a truncated
// file. The caller must hold mu.
//
// The sequence, and why each step is there:
//
//  1. the temporary file is created in the SAME directory as the target, so
//     the rename below is within one filesystem and therefore atomic. A
//     temporary file in /tmp would make step 5 a copy, which is not,
//  2. its mode is set explicitly, because os.CreateTemp's 0600 is still
//     subject to the process umask,
//  3. the bytes are written,
//  4. the file is fsynced, so the data is on the medium before anything points
//     at it. Rename first and fsync after is the classic way to end up with a
//     file of the right name and zero length,
//  5. the rename replaces the target in one step. A reader either sees all of
//     the old file or all of the new one, never a mixture,
//  6. the directory is fsynced, so the rename itself survives a power cut.
//     Without this the new file's contents are durable but the name still
//     points at the old inode.
//
// On any failure the temporary file is removed and the target is left exactly
// as it was.
func (s *Store) writeAtomic(st State) (err error) {
	if err := s.ensureDir(); err != nil {
		return err
	}

	// Indented, with a trailing newline: an admin may well open this file, and
	// a diff of it should be readable.
	//
	// HTML escaping is turned off, which json.Marshal and json.MarshalIndent do
	// not allow. It is not cosmetic here: a share link is mostly query
	// parameters, so the default rewrites every ampersand as the six-character
	// escape "backslash-u-0-0-2-6", and a typical stored config becomes
	// unreadable. Measured on the test vector before this was changed: one
	// vless link came back with six of them. Nothing in this package emits
	// the file into an HTML document, so the escaping protects nothing.
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(st); err != nil {
		return fmt.Errorf("state: encoding state: %w", err)
	}
	buf := out.Bytes() // Encode already appended the trailing newline

	// The dot prefix keeps a leftover out of a casual directory listing, and
	// the fixed suffix makes any leftover identifiable as ours.
	tmp, err := os.CreateTemp(s.dir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("state: creating a temporary file in %s: %w", s.dir, err)
	}
	tmpPath := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			tmp.Close()        // no-op if already closed
			os.Remove(tmpPath) // leave nothing behind on the failure path
		}
	}()

	if err := tmp.Chmod(fileMode); err != nil {
		return fmt.Errorf("state: setting permissions on %s: %w", tmpPath, err)
	}
	if _, err := tmp.Write(buf); err != nil {
		return fmt.Errorf("state: writing %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("state: flushing %s to the medium: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("state: closing %s: %w", tmpPath, err)
	}

	if s.hookAfterTempWrite != nil {
		if err := s.hookAfterTempWrite(tmpPath); err != nil {
			return err
		}
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("state: replacing %s: %w", s.path, err)
	}
	renamed = true

	return syncDir(s.dir)
}

// ensureDir creates the state directory at 0700 if it is absent, and refuses to
// write into an existing one whose mode is looser. Tightening it automatically
// was rejected: an administrator who widened it did so for a reason, and
// silently reverting that is the kind of surprise that gets debugged twice.
func (s *Store) ensureDir() error {
	di, err := os.Stat(s.dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// MkdirAll cannot produce a mode looser than the one asked for; a
		// umask can only remove bits.
		if err := os.MkdirAll(s.dir, dirMode); err != nil {
			return fmt.Errorf("state: creating %s: %w", s.dir, err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("state: examining %s: %w", s.dir, err)
	case !di.IsDir():
		return fmt.Errorf("state: %s is not a directory", s.dir)
	}
	return checkPerms(s.dir, di, dirMode, "directory")
}

// syncDir fsyncs a directory so that a rename into it is durable.
//
// Note for a later macOS phase (design section 2): on Darwin, fsync flushes to
// the drive but does not order the drive's own write cache. Full durability
// there needs fcntl F_FULLFSYNC. The target for v1 is Linux on a Raspberry Pi,
// where fsync is sufficient, so that is not implemented.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("state: opening %s to flush it: %w", dir, err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("state: flushing directory %s: %w", dir, err)
	}
	return nil
}
