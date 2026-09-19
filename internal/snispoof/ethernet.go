// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package snispoof

import "encoding/binary"

func ethernetFrame(b []byte) (frame, bool) {
	if len(b) < 14 {
		return frame{}, false
	}
	offset := 14
	typ := binary.BigEndian.Uint16(b[12:14])
	for tags := 0; typ == 0x8100 || typ == 0x88a8; tags++ {
		if tags >= 2 || len(b) < offset+4 {
			return frame{}, false
		}
		typ = binary.BigEndian.Uint16(b[offset+2 : offset+4])
		offset += 4
	}
	if typ != 0x0800 {
		return frame{}, false
	}
	return frame{ip: append([]byte(nil), b[offset:]...), prefix: append([]byte(nil), b[:offset]...)}, true
}
