// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package state

import (
	"caspianbyoc.org/caspian/internal/snispoof"
	"errors"
)

// SetSpoofSNI changes only the optional fake name, never the imported TLS name.
func (s *Store) SetSpoofSNI(name string) error {
	name, err := snispoof.NormalizeName(name)
	if err != nil {
		return err
	}
	return s.Update(func(st *State) error {
		if !st.Proxy.IsConfigured() {
			return errors.New("state: no proxy configuration is stored")
		}
		st.Proxy.SpoofSNI = Secret(name)
		return nil
	})
}
