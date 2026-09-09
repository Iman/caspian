// SPDX-License-Identifier: AGPL-3.0-or-later
package state

import (
	"errors"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"time"
)

// A reader or scanner can briefly prevent replacement on Windows. Keep the
// old file intact and retry only sharing/access conflicts, for at most a second.
func replaceStateFile(from, to string) error {
	var err error
	for attempt := 0; attempt < 50; attempt++ {
		err = os.Rename(from, to)
		if err == nil || (!errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_ACCESS_DENIED)) {
			return err
		}
		if attempt < 49 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	return err
}

// Readers permit replacement, so a writer can publish the next complete file
// while this reader finishes reading the previous one.
func readStateFile(path string) ([]byte, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	return io.ReadAll(f)
}
