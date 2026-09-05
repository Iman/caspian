// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
)

// generatedIdentifiers is deliberately response-only. Generated values never
// enter Panel, the state store, a session, or a log record: once this response
// has been written, the browser is the only place Caspian keeps them.
type generatedIdentifiers struct {
	UUID string `json:"uuid"`
	IMEI string `json:"imei"`
}

func (p *Panel) handleIdentifiersJSON(w http.ResponseWriter, _ *http.Request) {
	identifiers, err := generateIdentifiers(cryptorand.Reader)
	if err != nil {
		// Do not include partially generated values or the random source's error.
		// Neither helps the person using the panel, and keeping the log constant
		// makes it impossible for an identifier to reach it through a wrapped
		// error in a future implementation.
		p.log.Error("identifier generation failed")
		writeJSONError(w, http.StatusInternalServerError, "identifier generation failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(identifiers); err != nil {
		p.log.Warn("writing identifier json failed")
	}
}

func generateIdentifiers(random io.Reader) (generatedIdentifiers, error) {
	uuid, err := randomUUIDv4(random)
	if err != nil {
		return generatedIdentifiers{}, err
	}
	imei, err := randomIMEIShape(random)
	if err != nil {
		return generatedIdentifiers{}, err
	}
	return generatedIdentifiers{UUID: uuid, IMEI: imei}, nil
}

// randomUUIDv4 sets the version and variant bits defined for a UUIDv4. The
// remaining 122 bits come directly from the operating system's random source.
func randomUUIDv4(random io.Reader) (string, error) {
	var raw [16]byte
	if _, err := io.ReadFull(random, raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], raw[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], raw[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], raw[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], raw[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], raw[10:16])
	return string(encoded), nil
}

// randomIMEIShape returns 14 independently random decimal digits followed by
// the Luhn check digit used by an IMEI. It is only an IMEI-shaped test value:
// Caspian does not allocate a TAC and must never present it as a device's real
// cellular identity.
func randomIMEIShape(random io.Reader) (string, error) {
	const bodyDigits = 14
	body := make([]byte, bodyDigits, bodyDigits+1)
	ten := big.NewInt(10)
	for i := range body {
		digit, err := cryptorand.Int(random, ten)
		if err != nil {
			return "", err
		}
		body[i] = byte(digit.Int64()) + '0'
	}
	return string(append(body, imeiCheckDigit(body))), nil
}

func imeiCheckDigit(body []byte) byte {
	sum := 0
	for i, raw := range body {
		digit := int(raw - '0')
		if i%2 == 1 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return byte((10-sum%10)%10) + '0'
}
