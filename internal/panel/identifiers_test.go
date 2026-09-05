// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var (
	uuidV4Shape = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	imeiShape   = regexp.MustCompile(`^[0-9]{15}$`)
)

func TestGeneratedIdentifiersHaveThePromisedShapes(t *testing.T) {
	seenUUID := make(map[string]bool)
	seenIMEI := make(map[string]bool)
	for range 128 {
		got, err := generateIdentifiers(rand.Reader)
		if err != nil {
			t.Fatalf("generateIdentifiers: %v", err)
		}
		if !uuidV4Shape.MatchString(got.UUID) {
			t.Errorf("UUID %q is not an RFC UUIDv4", got.UUID)
		}
		if !imeiShape.MatchString(got.IMEI) {
			t.Errorf("IMEI-shaped value %q is not 15 decimal digits", got.IMEI)
		}
		if !validLuhn(got.IMEI) {
			t.Errorf("IMEI-shaped value %q has a bad Luhn check digit", got.IMEI)
		}
		if seenUUID[got.UUID] {
			t.Fatalf("UUID repeated in a small sample: %q", got.UUID)
		}
		if seenIMEI[got.IMEI] {
			t.Fatalf("IMEI-shaped value repeated in a small sample: %q", got.IMEI)
		}
		seenUUID[got.UUID] = true
		seenIMEI[got.IMEI] = true
	}
}

func TestZeroRandomInputStillSetsUUIDBitsAndIMEICheckDigit(t *testing.T) {
	got, err := generateIdentifiers(bytes.NewReader(make([]byte, 64)))
	if err != nil {
		t.Fatal(err)
	}
	if got.UUID != "00000000-0000-4000-8000-000000000000" {
		t.Errorf("UUID = %q", got.UUID)
	}
	if got.IMEI != "000000000000000" {
		t.Errorf("IMEI-shaped value = %q", got.IMEI)
	}
}

func TestIdentifierGenerationReturnsNoPartialValueOnRandomFailure(t *testing.T) {
	wantErr := errors.New("random source failed")
	got, err := generateIdentifiers(io.MultiReader(
		bytes.NewReader(make([]byte, 16)),
		errorReader{err: wantErr},
	))
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if got != (generatedIdentifiers{}) {
		t.Errorf("failure returned partial identifiers: %+v", got)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestIdentifierEndpointIsAuthenticatedEphemeralJSON(t *testing.T) {
	h := newHarness(t)
	h.ready()
	before := h.store.Snapshot()

	res, body := h.get("/identifiers.json")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q", got)
	}
	if got := res.Header.Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	var first generatedIdentifiers
	if err := json.Unmarshal([]byte(body), &first); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !uuidV4Shape.MatchString(first.UUID) || !imeiShape.MatchString(first.IMEI) || !validLuhn(first.IMEI) {
		t.Fatalf("bad identifiers from endpoint: %+v", first)
	}

	_, secondBody := h.get("/identifiers.json")
	var second generatedIdentifiers
	if err := json.Unmarshal([]byte(secondBody), &second); err != nil {
		t.Fatalf("invalid second JSON: %v", err)
	}
	if first == second {
		t.Fatal("two requests returned the same identifier pair")
	}
	if after := h.store.Snapshot(); !reflect.DeepEqual(after, before) {
		t.Error("generating identifiers changed persistent state")
	}
	for _, value := range []string{first.UUID, first.IMEI, second.UUID, second.IMEI} {
		if strings.Contains(h.logs.String(), value) {
			t.Errorf("generated value %q reached a log", value)
		}
	}

	h.signedOut()
	res, _ = h.get("/identifiers.json")
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("signed-out status = %d, want 401", res.StatusCode)
	}
}

func validLuhn(value string) bool {
	if !imeiShape.MatchString(value) {
		return false
	}
	sum := 0
	for i, raw := range []byte(value) {
		digit := int(raw - '0')
		if i%2 == 1 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}
