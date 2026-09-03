// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package state

import (
	"fmt"
	"os"
)

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
