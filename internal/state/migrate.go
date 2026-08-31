// SPDX-License-Identifier: AGPL-3.0-or-later

package state

import "fmt"

// Schema versioning.
//
// The file carries its version so that a future release can migrate rather than
// guess. Two rules make that work:
//
//   - a version below CurrentVersion is upgraded in memory by the chain in
//     migrations, and written back at CurrentVersion on the next Save,
//   - a version above CurrentVersion is refused. This build cannot know which
//     fields a newer release added, so loading it would work and the next Save
//     would silently drop them. Refusing turns a data-loss bug into a message
//     telling the user to update.
//
// Version 0 means "written before the version field existed": a JSON object
// with no version key unmarshals to 0. No release has shipped a v0 file, so
// migrateV0ToV1 is written against that shape rather than against history.

// migrations[v] upgrades a State in place from schema version v to v+1. A gap
// in this map for any version below CurrentVersion is a programming error, and
// migrate reports it as one rather than proceeding.
var migrations = map[int]func(*State) error{
	0: migrateV0ToV1,
	1: migrateV1ToV2,
}

// ErrFutureVersion is the sentinel behind the refusal of a newer file, so a
// caller such as the panel can present the "update this box" screen rather than
// a generic failure.
type ErrFutureVersion struct {
	Path     string
	Found    int
	Supports int
}

func (e *ErrFutureVersion) Error() string {
	return fmt.Sprintf(
		"state: %s was written by schema version %d, but this build understands at most version %d; "+
			"install a newer Caspian release, or move that file aside to start over (its proxy config and passwords will be lost)",
		e.Path, e.Found, e.Supports)
}

// migrate brings a freshly decoded State up to CurrentVersion.
func migrate(st *State, path string) error {
	if st.Version > CurrentVersion {
		return &ErrFutureVersion{Path: path, Found: st.Version, Supports: CurrentVersion}
	}
	if st.Version < 0 {
		return fmt.Errorf("state: %s declares schema version %d, which is not a version", path, st.Version)
	}
	for st.Version < CurrentVersion {
		from := st.Version
		up, ok := migrations[from]
		if !ok {
			return fmt.Errorf("state: %s is at schema version %d and this build has no migration from %d to %d", path, from, from, from+1)
		}
		if err := up(st); err != nil {
			return fmt.Errorf("state: migrating %s from schema version %d to %d: %w", path, from, from+1, err)
		}
		if st.Version != from+1 {
			// A migration that forgets to advance the version would spin here
			// forever. Fail loudly instead.
			return fmt.Errorf("state: migration from schema version %d left the version at %d", from, st.Version)
		}
	}
	return nil
}

// migrateV0ToV1 upgrades a pre-versioning file.
//
// The substantive change is the policy fields. A v0 file has no opinion on DNS
// behaviour or on what happens when the tunnel drops, so both decode empty, and
// empty is not a safe reading: a downstream package that treats "" as "no
// policy configured" and lets forwarded traffic out would break the fail-closed
// promise in design sections 6 and 7. The migration makes the safe position
// explicit rather than leaving it to be inferred.
//
// It fills only what is absent. A v0 file that already carries a value keeps
// it, because that value was a user's choice.
func migrateV0ToV1(st *State) error {
	if st.Advanced.DNSMode == "" {
		st.Advanced.DNSMode = DNSModeTunnel
	}
	if st.Advanced.OnTunnelDown == "" {
		st.Advanced.OnTunnelDown = OnTunnelDownBlock
	}
	st.Version = 1
	return nil
}

// migrateV1ToV2 fills in the client IPv6 policy.
//
// v1 has no client_ipv6 key, so every file written by a shipped build decodes
// it empty, and empty is the one reading this package promises a policy field
// will never produce: a downstream package that treated it as "no policy
// configured" would let client IPv6 out, which bypasses the tunnel completely
// rather than merely misbehaving.
//
// This is the whole of the change from 1 to 2. Nothing else moves, no field is
// renamed and no field is dropped, so a v1 file loads with everything in it
// intact and one field added. The reverse is refused by design: a v2 file read
// by a v1 build raises ErrFutureVersion, which tells the user to update rather
// than silently dropping the field on the next Save.
//
// Like migrateV0ToV1 it fills only what is absent, because a value already in
// the file was somebody's choice and this package does not overrule it. Whether
// that choice can be honoured is decided at the privileged boundary, which
// supports ClientIPv6Block alone and names its refusal.
func migrateV1ToV2(st *State) error {
	if st.Advanced.ClientIPv6 == "" {
		st.Advanced.ClientIPv6 = ClientIPv6Block
	}
	st.Version = 2
	return nil
}
