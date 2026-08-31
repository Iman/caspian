// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// The panel password is verified, never stored.
//
// Algorithm: Argon2id, from golang.org/x/crypto/argon2, which is already in
// go.mod. It is the memory-hard function the requirement asked for and the
// winner of the Password Hashing Competition. The alternatives available in this
// tree were rejected: crypto/pbkdf2 in the standard library is not memory-hard,
// so a GPU or ASIC attacker gets the full parallelism advantage; bcrypt is only
// weakly memory-hard, with a fixed 4 KiB working set that fits in cache;
// x/crypto/scrypt is memory-hard and would be an acceptable second choice, but
// Argon2id is the current recommendation (RFC 9106) and resists both GPU and
// side-channel attack by combining the data-independent and data-dependent
// passes.
//
// Parameters follow the second option in RFC 9106 section 4: 64 MiB of memory,
// three passes, four lanes. The first option, 2 GiB, is not available on a
// Raspberry Pi that also has to run an xray-core instance and an access point.
// The cost of one verification on the target hardware is not measured; see the
// note in the package report.
const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // KiB, so 64 MiB
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// Parsing bounds. The state file is 0600 and its directory 0700, so a hostile
// PHC string is not the expected case. These exist anyway because the failure
// mode is bad out of proportion to the cost of the check: a corrupted or
// tampered m= value of a few million KiB would have the box try to allocate
// gigabytes during a login attempt and be killed by the OOM reaper, which turns
// a bad byte on an SD card into an appliance that cannot be logged into.
const (
	maxArgonMemory  uint32 = 1024 * 1024 // KiB, so 1 GiB
	maxArgonTime    uint32 = 16
	maxArgonThreads uint8  = 16
	maxArgonKeyLen  int    = 64
	maxArgonSaltLen int    = 64
)

// ErrNoPanelPassword is returned by VerifyPanelPassword when no password has
// been set yet. It is distinguishable from a wrong password on purpose: the
// panel has to show a setup screen in the first case and a refusal in the
// second. Design section 5.6.
var ErrNoPanelPassword = errors.New("state: no panel password has been set")

// hashParams are the parameters recovered from a stored PHC string. They are
// per-hash rather than global so that raising the cost in a later release does
// not invalidate passwords already set on deployed boxes: an old hash still
// verifies with the parameters it was made with.
type hashParams struct {
	time    uint32
	memory  uint32
	threads uint8
	salt    []byte
	key     []byte
}

// hashPassword derives a PHC-format argon2id string from a plaintext password.
// The plaintext is not retained, not logged and not returned.
func hashPassword(plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("state: panel password must not be empty")
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("state: generating password salt: %w", err)
	}
	key := argon2.IDKey([]byte(plaintext), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return encodeHash(hashParams{
		time:    argonTime,
		memory:  argonMemory,
		threads: argonThreads,
		salt:    salt,
		key:     key,
	}), nil
}

// verifyPassword reports whether plaintext matches the stored PHC string. It
// returns an error only when the stored string cannot be understood, which is a
// corrupt-state condition and not a wrong password; a wrong password is
// (false, nil).
func verifyPassword(stored, plaintext string) (bool, error) {
	p, err := decodeHash(stored)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(plaintext), p.salt, p.time, p.memory, p.threads, uint32(len(p.key)))
	// Constant time: a timing-distinguishable compare would leak how many
	// leading bytes of a guess were right, which turns an offline problem into
	// an online one.
	return subtle.ConstantTimeCompare(got, p.key) == 1, nil
}

// encodeHash writes the standard PHC string format, the same shape the
// reference Argon2 implementation and every other Go example produce:
//
//	$argon2id$v=19$m=65536,t=3,p=4$<base64 salt>$<base64 key>
//
// Storing the parameters beside the hash is what makes the cost upgradable, and
// makes the file readable by a person debugging without any tool.
func encodeHash(p hashParams) string {
	b64 := base64.RawStdEncoding.Strict()
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.time, p.threads,
		b64.EncodeToString(p.salt), b64.EncodeToString(p.key))
}

func decodeHash(stored string) (hashParams, error) {
	var p hashParams
	parts := strings.Split(stored, "$")
	// A leading "$" gives an empty first field, so a well-formed string splits
	// into exactly six.
	if len(parts) != 6 || parts[0] != "" {
		return p, errors.New("state: stored panel password hash is not in PHC format")
	}
	if parts[1] != "argon2id" {
		return p, fmt.Errorf("state: stored panel password hash uses %q, but this build only verifies argon2id", parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return p, fmt.Errorf("state: stored panel password hash has an unreadable version field %q", parts[2])
	}
	if version != argon2.Version {
		return p, fmt.Errorf("state: stored panel password hash is argon2 version %d, but this build links version %d", version, argon2.Version)
	}

	var memory, timeCost uint64
	var threads uint64
	fields := strings.Split(parts[3], ",")
	if len(fields) != 3 {
		return p, fmt.Errorf("state: stored panel password hash has an unreadable parameter field %q", parts[3])
	}
	var err error
	if memory, err = parseKV(fields[0], "m"); err != nil {
		return p, err
	}
	if timeCost, err = parseKV(fields[1], "t"); err != nil {
		return p, err
	}
	if threads, err = parseKV(fields[2], "p"); err != nil {
		return p, err
	}
	if memory == 0 || memory > uint64(maxArgonMemory) {
		return p, fmt.Errorf("state: stored panel password hash asks for %d KiB of memory, outside the accepted range 1..%d", memory, maxArgonMemory)
	}
	if timeCost == 0 || timeCost > uint64(maxArgonTime) {
		return p, fmt.Errorf("state: stored panel password hash asks for %d passes, outside the accepted range 1..%d", timeCost, maxArgonTime)
	}
	if threads == 0 || threads > uint64(maxArgonThreads) {
		return p, fmt.Errorf("state: stored panel password hash asks for %d lanes, outside the accepted range 1..%d", threads, maxArgonThreads)
	}

	b64 := base64.RawStdEncoding.Strict()
	if p.salt, err = b64.DecodeString(parts[4]); err != nil {
		return p, errors.New("state: stored panel password hash has an undecodable salt")
	}
	if p.key, err = b64.DecodeString(parts[5]); err != nil {
		return p, errors.New("state: stored panel password hash has an undecodable digest")
	}
	if len(p.salt) == 0 || len(p.salt) > maxArgonSaltLen {
		return p, fmt.Errorf("state: stored panel password hash has a %d byte salt, outside the accepted range 1..%d", len(p.salt), maxArgonSaltLen)
	}
	if len(p.key) == 0 || len(p.key) > maxArgonKeyLen {
		return p, fmt.Errorf("state: stored panel password hash has a %d byte digest, outside the accepted range 1..%d", len(p.key), maxArgonKeyLen)
	}

	p.memory = uint32(memory)
	p.time = uint32(timeCost)
	p.threads = uint8(threads)
	return p, nil
}

func parseKV(field, key string) (uint64, error) {
	name, value, ok := strings.Cut(field, "=")
	if !ok || name != key {
		return 0, fmt.Errorf("state: stored panel password hash expected %q= in %q", key, field)
	}
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("state: stored panel password hash has an unreadable %q value %q", key, value)
	}
	return n, nil
}

// describeHash reports a stored hash's algorithm and cost parameters and
// nothing else, for the redacted rendering. The salt and digest never appear.
func describeHash(stored string) string {
	p, err := decodeHash(stored)
	if err != nil {
		return "unreadable"
	}
	return fmt.Sprintf("argon2id(m=%d,t=%d,p=%d)", p.memory, p.time, p.threads)
}
