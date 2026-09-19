// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"strings"
	"testing"

	"caspianbyoc.org/caspian/internal/state"
)

// Written before subscription.go existed, on 2026-09-09; the first run was a
// compile failure naming parseUserinfo, labelFromHeaders and gigabytes.

// TestParseUserinfo is the table the design asks for, malformed pairs
// included. The rule: key=value pairs separated by semicolons, the four known
// keys with non-negative integers, and everything else ignored without
// touching the keys that were read.
func TestParseUserinfo(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want state.Quota
	}{
		{"empty", "", state.Quota{}},
		{"all four", "upload=1;download=2;total=3;expire=4", state.Quota{Upload: 1, Download: 2, Total: 3, Expire: 4}},
		{"spaces around", " upload = 1 ; download=2 ;total=3; expire=4 ", state.Quota{Upload: 1, Download: 2, Total: 3, Expire: 4}},
		{"a subset", "download=2;total=3", state.Quota{Download: 2, Total: 3}},
		{"upper case keys", "UPLOAD=1;Download=2", state.Quota{Upload: 1, Download: 2}},
		{"unknown keys ignored", "upload=1;quota=9;reset=7", state.Quota{Upload: 1}},
		{"a pair with no value", "upload;download=2", state.Quota{Download: 2}},
		{"a pair with no key", "=5;download=2", state.Quota{Download: 2}},
		{"a pair with two equals", "upload=1=2;download=2", state.Quota{Download: 2}},
		{"a value that is not a number", "upload=lots;download=2", state.Quota{Download: 2}},
		{"a negative value", "upload=-1;download=2", state.Quota{Download: 2}},
		{"a decimal", "upload=1.5;download=2", state.Quota{Download: 2}},
		{"an overflow", "upload=99999999999999999999;download=2", state.Quota{Download: 2}},
		{"the last of a repeated key wins", "upload=1;upload=7", state.Quota{Upload: 7}},
		{"empty pairs between", "upload=1;;;download=2;", state.Quota{Upload: 1, Download: 2}},
		{"large real figures", "upload=100000000; download=12300000000; total=100000000000; expire=1780272000",
			state.Quota{Upload: 100000000, Download: 12300000000, Total: 100000000000, Expire: 1780272000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseUserinfo(tc.in); got != tc.want {
				t.Errorf("parseUserinfo(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

// TestLabelFromHeaders: the title first, decoded when base64-prefixed; then
// the file name; capped and stripped of anything that is not printable.
func TestLabelFromHeaders(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{"nothing", nil, ""},
		{"plain title", map[string]string{"profile-title": "Provider"}, "Provider"},
		{"base64 title", map[string]string{"profile-title": "base64:UHJvdmlkZXIgUGx1cw=="}, "Provider Plus"},
		{"base64 title in utf-8", map[string]string{"profile-title": "base64:2LPYsdmI2LE="}, "سرور"},
		{"base64 that does not decode is used as written", map[string]string{"profile-title": "base64:!!!"}, "base64:!!!"},
		{"title wins over file name", map[string]string{"profile-title": "T", "content-disposition": `attachment; filename="f.txt"`}, "T"},
		{"file name", map[string]string{"content-disposition": `attachment; filename="sub.txt"`}, "sub.txt"},
		{"file name without quotes", map[string]string{"content-disposition": `attachment; filename=sub.txt`}, "sub.txt"},
		{"rfc 5987 file name", map[string]string{"content-disposition": `attachment; filename*=UTF-8''%D8%B3%D8%B1%D9%88%D8%B1.txt`}, "سرور.txt"},
		{"disposition with no file name", map[string]string{"content-disposition": "inline"}, ""},
		{"disposition that does not parse", map[string]string{"content-disposition": `;;=`}, ""},
		{"control characters stripped", map[string]string{"profile-title": "Pro\x00vi\nder\t!"}, "Provider!"},
		{"surrounding space trimmed", map[string]string{"profile-title": "  Provider  "}, "Provider"},
		{"capped at maxLabel", map[string]string{"profile-title": strings.Repeat("x", maxLabel+10)}, strings.Repeat("x", maxLabel)},
		{"cap does not split a rune", map[string]string{"profile-title": strings.Repeat("س", maxLabel)}, strings.Repeat("س", maxLabel/2)},
		{"only control characters", map[string]string{"profile-title": "\x01\x02"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := labelFromHeaders(tc.headers); got != tc.want {
				t.Errorf("labelFromHeaders = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestGigabytes pins the one-decimal rendering the usage line uses.
func TestGigabytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.0 GB"},
		{1, "0.0 GB"},
		{50_000_000, "0.1 GB"},
		{12_400_000_000, "12.4 GB"},
		{12_350_000_000, "12.4 GB"},
		{100_000_000_000, "100.0 GB"},
		{1_000_000_000_000, "1000.0 GB"},
	}
	for _, tc := range cases {
		if got := gigabytes(tc.in); got != tc.want {
			t.Errorf("gigabytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
