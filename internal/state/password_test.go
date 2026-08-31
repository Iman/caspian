// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import (
	"errors"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestHashAndVerifyPassword(t *testing.T) {
	tests := []struct {
		name    string
		set     string
		try     string
		wantOK  bool
		comment string
	}{
		{name: "the right password verifies", set: fakePanelPass, try: fakePanelPass, wantOK: true},
		{name: "a wrong password is rejected", set: fakePanelPass, try: "not-the-password", wantOK: false},
		{name: "an empty attempt is rejected", set: fakePanelPass, try: "", wantOK: false},
		{name: "a one-character difference is rejected", set: fakePanelPass, try: fakePanelPass + "x", wantOK: false},
		{name: "a prefix is rejected", set: fakePanelPass, try: fakePanelPass[:len(fakePanelPass)-1], wantOK: false},
		{name: "case matters", set: fakePanelPass, try: strings.ToUpper(fakePanelPass), wantOK: false},
		{name: "a unicode password verifies", set: "sifre-çok-güçlü-یک", try: "sifre-çok-güçlü-یک", wantOK: true},
		{name: "a long password verifies", set: strings.Repeat("a-long-passphrase ", 20), try: strings.Repeat("a-long-passphrase ", 20), wantOK: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := hashPassword(tc.set)
			if err != nil {
				t.Fatalf("hashPassword: %v", err)
			}
			// The plaintext must not be recoverable from, or present in, the
			// stored form.
			if strings.Contains(hash, tc.set) {
				t.Fatal("the stored hash contains the plaintext password")
			}
			ok, err := verifyPassword(hash, tc.try)
			if err != nil {
				t.Fatalf("verifyPassword returned an error for a well-formed hash: %v", err)
			}
			if ok != tc.wantOK {
				t.Errorf("verifyPassword = %t, want %t", ok, tc.wantOK)
			}
		})
	}
}

func TestHashPasswordRejectsEmpty(t *testing.T) {
	if _, err := hashPassword(""); err == nil {
		t.Error("hashPassword accepted an empty password")
	}
}

// TestHashIsSaltedPerCall. Two boxes with the same password must not produce
// the same stored value, or one cracked hash breaks every box that shares it.
func TestHashIsSaltedPerCall(t *testing.T) {
	a, err := hashPassword(fakePanelPass)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	b, err := hashPassword(fakePanelPass)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if a == b {
		t.Fatal("hashing the same password twice produced the same string; the salt is not random")
	}
	// Both must still verify.
	for i, h := range []string{a, b} {
		ok, err := verifyPassword(h, fakePanelPass)
		if err != nil || !ok {
			t.Errorf("hash %d did not verify: ok=%t err=%v", i, ok, err)
		}
	}
}

// TestHashFormatIsPHC pins the on-disk shape, because it is documented and
// because a change to it is a compatibility event that should be deliberate.
func TestHashFormatIsPHC(t *testing.T) {
	hash, err := hashPassword(fakePanelPass)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" {
		t.Fatalf("hash %q does not split into the six PHC fields", hash)
	}
	if parts[1] != "argon2id" {
		t.Errorf("algorithm = %q, want argon2id", parts[1])
	}
	if parts[2] != "v=19" {
		t.Errorf("version field = %q, want v=19 (argon2.Version is %d)", parts[2], argon2.Version)
	}
	if parts[3] != "m=65536,t=3,p=4" {
		t.Errorf("parameters = %q, want m=65536,t=3,p=4", parts[3])
	}

	p, err := decodeHash(hash)
	if err != nil {
		t.Fatalf("decodeHash on our own output: %v", err)
	}
	if p.memory != argonMemory || p.time != argonTime || p.threads != argonThreads {
		t.Errorf("decoded parameters m=%d t=%d p=%d, want m=%d t=%d p=%d",
			p.memory, p.time, p.threads, argonMemory, argonThreads, argonTime)
	}
	if len(p.salt) != argonSaltLen {
		t.Errorf("salt is %d bytes, want %d", len(p.salt), argonSaltLen)
	}
	if len(p.key) != int(argonKeyLen) {
		t.Errorf("digest is %d bytes, want %d", len(p.key), argonKeyLen)
	}
}

// TestVerifyUsesTheParametersInTheHash is what makes a future cost increase
// safe: a hash written with old parameters still verifies after the constants
// are raised.
func TestVerifyUsesTheParametersInTheHash(t *testing.T) {
	// A hash made with parameters deliberately unlike the current constants.
	salt := []byte("sixteen-byte-slt")
	key := argon2.IDKey([]byte(fakePanelPass), salt, 1, 8*1024, 1, 16)
	old := encodeHash(hashParams{time: 1, memory: 8 * 1024, threads: 1, salt: salt, key: key})

	ok, err := verifyPassword(old, fakePanelPass)
	if err != nil {
		t.Fatalf("verifyPassword on an old-parameter hash: %v", err)
	}
	if !ok {
		t.Error("a hash written with older parameters no longer verifies; deployed boxes would be locked out by a cost change")
	}
	ok, err = verifyPassword(old, "wrong")
	if err != nil {
		t.Fatalf("verifyPassword: %v", err)
	}
	if ok {
		t.Error("a wrong password verified against an old-parameter hash")
	}
}

// TestDecodeHashRejectsMalformedInput. A corrupt or tampered stored value must
// produce an error, not a panic and not a false accept. The oversized memory
// case matters most: without the bound, one bad byte on an SD card turns a
// login attempt into an out-of-memory kill.
func TestDecodeHashRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name   string
		stored string
	}{
		{"empty", ""},
		{"not PHC at all", "hunter2"},
		{"too few fields", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA"},
		{"too many fields", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$a2V5$extra"},
		{"no leading dollar", "argon2id$v=19$m=65536,t=3,p=4$c2FsdA$a2V5"},
		{"a different algorithm", "$argon2i$v=19$m=65536,t=3,p=4$c2FsdA$a2V5"},
		{"bcrypt", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"},
		{"unreadable version", "$argon2id$v=abc$m=65536,t=3,p=4$c2FsdA$a2V5"},
		{"wrong argon2 version", "$argon2id$v=16$m=65536,t=3,p=4$c2FsdA$a2V5"},
		{"missing a parameter", "$argon2id$v=19$m=65536,t=3$c2FsdA$a2V5"},
		{"parameters out of order", "$argon2id$v=19$t=3,m=65536,p=4$c2FsdA$a2V5"},
		{"unreadable memory", "$argon2id$v=19$m=lots,t=3,p=4$c2FsdA$a2V5"},
		{"zero memory", "$argon2id$v=19$m=0,t=3,p=4$c2FsdA$a2V5"},
		{"absurd memory would exhaust the box", "$argon2id$v=19$m=99999999,t=3,p=4$c2FsdA$a2V5"},
		{"zero passes", "$argon2id$v=19$m=65536,t=0,p=4$c2FsdA$a2V5"},
		{"absurd passes", "$argon2id$v=19$m=65536,t=9999,p=4$c2FsdA$a2V5"},
		{"zero lanes", "$argon2id$v=19$m=65536,t=3,p=0$c2FsdA$a2V5"},
		{"absurd lanes", "$argon2id$v=19$m=65536,t=3,p=999$c2FsdA$a2V5"},
		{"undecodable salt", "$argon2id$v=19$m=65536,t=3,p=4$!!!!$a2V5"},
		{"undecodable digest", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$!!!!"},
		{"padded base64 is refused as non-strict", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA==$a2V5"},
		{"empty salt", "$argon2id$v=19$m=65536,t=3,p=4$$a2V5"},
		{"empty digest", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeHash(tc.stored); err == nil {
				t.Error("decodeHash accepted a malformed stored hash")
			}
			ok, err := verifyPassword(tc.stored, fakePanelPass)
			if err == nil {
				t.Error("verifyPassword accepted a malformed stored hash")
			}
			if ok {
				t.Error("verifyPassword returned true for a malformed stored hash")
			}
		})
	}
}

// ------------------------------------------------- through the Store

func TestStorePanelPassword(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Before a password is set, the panel has to be able to tell "not set up"
	// from "wrong password".
	ok, err := st.VerifyPanelPassword(fakePanelPass)
	if ok {
		t.Error("VerifyPanelPassword returned true before any password was set")
	}
	if !errors.Is(err, ErrNoPanelPassword) {
		t.Errorf("error = %v, want ErrNoPanelPassword so the panel can show setup", err)
	}

	if err := st.SetPanelPassword(fakePanelPass); err != nil {
		t.Fatalf("SetPanelPassword: %v", err)
	}
	if err := st.SetPanelPassword(""); err == nil {
		t.Error("SetPanelPassword accepted an empty password")
	}

	tests := []struct {
		name   string
		try    string
		wantOK bool
	}{
		{"correct", fakePanelPass, true},
		{"wrong", "wrong-password", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name+" in memory", func(t *testing.T) {
			ok, err := st.VerifyPanelPassword(tc.try)
			if err != nil {
				t.Fatalf("VerifyPanelPassword: %v", err)
			}
			if ok != tc.wantOK {
				t.Errorf("= %t, want %t", ok, tc.wantOK)
			}
		})
	}

	// And after a reload from disk, which is the case that matters after a
	// reboot.
	reloaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, tc := range tests {
		t.Run(tc.name+" after reload", func(t *testing.T) {
			ok, err := reloaded.VerifyPanelPassword(tc.try)
			if err != nil {
				t.Fatalf("VerifyPanelPassword: %v", err)
			}
			if ok != tc.wantOK {
				t.Errorf("= %t, want %t", ok, tc.wantOK)
			}
		})
	}

	// The plaintext must be nowhere in the file.
	rawBytes, err := os.ReadFile(st.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	raw := string(rawBytes)
	if strings.Contains(raw, fakePanelPass) {
		t.Fatal("the panel password plaintext is in the state file")
	}
	if !strings.Contains(raw, "$argon2id$v=19$") {
		t.Error("the state file does not contain an argon2id verifier")
	}
}

func TestChangingThePanelPasswordInvalidatesTheOldOne(t *testing.T) {
	dir := tempStateDir(t)
	st, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.SetPanelPassword("first-password-not-real"); err != nil {
		t.Fatalf("SetPanelPassword: %v", err)
	}
	if err := st.SetPanelPassword("second-password-not-real"); err != nil {
		t.Fatalf("SetPanelPassword: %v", err)
	}
	if ok, _ := st.VerifyPanelPassword("first-password-not-real"); ok {
		t.Error("the old password still verifies after a change")
	}
	if ok, err := st.VerifyPanelPassword("second-password-not-real"); err != nil || !ok {
		t.Errorf("the new password does not verify: ok=%t err=%v", ok, err)
	}
}

func TestDescribeHashRevealsOnlyParameters(t *testing.T) {
	hash, err := hashPassword(fakePanelPass)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	got := describeHash(hash)
	if got != "argon2id(m=65536,t=3,p=4)" {
		t.Errorf("describeHash = %q", got)
	}
	if strings.Contains(got, fakePanelPass) {
		t.Error("describeHash leaked the password")
	}
	// The salt and digest must not appear either.
	parts := strings.Split(hash, "$")
	for _, secret := range []string{parts[4], parts[5]} {
		if strings.Contains(got, secret) {
			t.Error("describeHash leaked part of the stored hash")
		}
	}
	if describeHash("nonsense") != "unreadable" {
		t.Error("describeHash on a corrupt hash should report it as unreadable")
	}
}
