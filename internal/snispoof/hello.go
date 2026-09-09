// SPDX-License-Identifier: GPL-3.0-only
// Original template: patterniha/SNI-Spoofing contributors.
// Go translation and validation changes: Copyright (C) 2026 Iman Samizadeh
// ClientHello layout translated from patterniha/SNI-Spoofing (GPL-3.0).
// See third_party/sni-spoofing/README.md for provenance.

// Package snispoof implements an optional TCP prelude before the real proxy stream.
package snispoof

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/netip"
	"strings"

	"golang.org/x/net/idna"
)

var (
	ErrName         = errors.New("the spoof name must be a DNS hostname of at most 219 ASCII bytes")
	ErrUnsupported  = errors.New("SNI spoofing needs an IPv4 TCP transport and a supported packet backend")
	ErrUnavailable  = errors.New("the SNI spoofing packet backend is unavailable")
	ErrConfirmation = errors.New("the SNI spoofing handshake was not confirmed")
)

// NormalizeName returns a canonical DNS name. Empty means spoofing is disabled.
func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	name, err := idna.Lookup.ToASCII(name)
	if err != nil {
		return "", ErrName
	}
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if len(name) == 0 || len(name) > 219 {
		return "", ErrName
	}
	if _, err := netip.ParseAddr(name); err == nil {
		return "", ErrName
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrName
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", ErrName
			}
		}
	}
	return name, nil
}

const helloHex = "1603010200010001fc030341d5b549d9cd1adfa7296c8418d157dc7b624c842824ff493b9375bb48d34f2b20bf018bcc90a7c89a230094815ad0c15b736e38c01209d72d282cb5e2105328150024130213031301c02cc030c02bc02fcca9cca8c024c028c023c027009f009e006b006700ff0100018f0000000b00090000066d63692e6972000b000403000102000a00160014001d0017001e0019001801000101010201030104002300000010000e000c02683208687474702f312e310016000000170000000d002a0028040305030603080708080809080a080b080408050806040105010601030303010302040205020602002b00050403040303002d00020101003300260024001d0020435bacc4d05f9d41fef44ab3ad55616c36e0613473e2338770efdaa98693d217001500d5000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

func clientHello(name string) ([]byte, error) {
	name, err := NormalizeName(name)
	if err != nil || name == "" {
		return nil, ErrName
	}
	template, _ := hex.DecodeString(helloHex)
	random := make([]byte, 96)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 517)
	out = append(out, template[:11]...)
	out = append(out, random[:32]...)
	out = append(out, 0x20)
	out = append(out, random[32:64]...)
	out = append(out, template[76:120]...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(name)+5))
	out = binary.BigEndian.AppendUint16(out, uint16(len(name)+3))
	out = append(out, 0)
	out = binary.BigEndian.AppendUint16(out, uint16(len(name)))
	out = append(out, name...)
	out = append(out, template[133:268]...)
	out = append(out, random[64:]...)
	out = append(out, 0, 21)
	out = binary.BigEndian.AppendUint16(out, uint16(219-len(name)))
	out = append(out, make([]byte, 219-len(name))...)
	return out, nil
}
