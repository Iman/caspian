// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package qr

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtures maps a golden file in testdata to the exact bytes it was generated
// from. See testdata/PROVENANCE.md for the commands and for the decoder round
// trip that was run against these same inputs.
//
// The set covers every version this package builds, 1 to 12, because the parts
// most likely to be wrong are version-dependent: the remainder bits change at
// version 2 and again at version 7, the alignment patterns gain a row at
// version 7 and move at every version, the version information blocks appear
// at version 7, and the second block group appears at version 8.
var fixtures = map[string]string{
	"hello":        "HELLO",
	"v2":           "abcdefghijklmnopqrstuvwxy",
	"v3":           "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOP",
	"wifi_short":   "WIFI:T:WPA;S:Caspian;P:hunter2hunter2;H:false;;",
	"wifi_typical": "WIFI:T:WPA;S:Caspian-a41f;P:sun-rope-glass-mint-7412;H:false;;",
	"escaped":      `WIFI:T:WPA;S:Cafe\;Bar;P:a\,b\:c\\d;H:false;;`,
	"v5":           "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ab.,;:!@#$%^&*()_+-=",
	"v6":           "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ab.,;:!@#$%^&*()_+-=abcdefghijklmnopqrstuv",
	"wifi_long":    "WIFI:T:WPA;S:Caspian-guest-network-abcdefgh;P:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa;H:false;;",
	"v8":           "WIFI:T:WPA;S:Caspian-guest-network-abcdefgh;P:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa;H:false;;xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	"wifi_max":     "WIFI:T:WPA;S:0123456789012345678901234567890a;P:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb;H:false;;padpadpadpadpadpadpadpadpadpadpadpad",
	"v10":          "WIFI:T:WPA;S:0123456789012345678901234567890a;P:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb;H:false;;padpadpadpadpadpadpadpadpadpadpadpadpadpadpadpadpadpadpadpadpadpad",
	"v11":          "WIFI:T:WPA;S:" + xs(32) + ";P:" + ys(63) + ";H:false;;" + zs(116),
	"v12":          "WIFI:T:WPA;S:" + xs(32) + ";P:" + ys(63) + ";H:false;;" + zs(150),
}

func xs(n int) string { return strings.Repeat("x", n) }
func ys(n int) string { return strings.Repeat("y", n) }
func zs(n int) string { return strings.Repeat("z", n) }

// ---------------------------------------------------------------------------
// The cross-check against an independent implementation
//
// libqrencode is a different implementation by a different author, so it is the
// only thing here that can catch a systematic misreading of the standard.
// Everything else in this file checks the package against itself or against a
// number copied out of the same standard the code was written from.
//
// It does NOT agree with this package on which mask to use, and that is
// expected rather than a defect. Measured 2026-08-30 over the fixture set: the
// two chose the same mask for 6 of 12 and differed for the other 6. The cause
// is penalty rule N3. This package implements the standard's own worked
// pattern, the eleven modules 10111010000 and its mirror, which is what ZXing
// and the common Go encoders do. libqrencode's mask.c generalises the 1:1:3:1:1
// ratio to any scale and requires a light run of four times that scale, so it
// scores some symbols differently and picks a different mask.
//
// Mask choice is a readability heuristic, not a correctness property: every one
// of the eight masks yields a valid symbol as long as the format information
// says which was used. So the golden comparison is done in two parts. The
// content comparison below strips the mask from both symbols and requires them
// to be identical module for module, which is the part where a real encoding
// bug would show. The exact comparison then requires byte identity for the
// fixtures where the mask happens to agree, which is what exercises the format
// bit placement end to end.
//
// The claim that a different mask still reads was not left as an argument. All
// twelve symbols were rendered to PNG and decoded back to the exact input by
// OpenCV 5.0.0; see testdata/PROVENANCE.md.
// ---------------------------------------------------------------------------

func TestMatchesLibqrencodeContent(t *testing.T) {
	agreed := 0
	for name, input := range fixtures {
		t.Run(name, func(t *testing.T) {
			ref := loadFixture(t, name)
			got, err := Encode([]byte(input))
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if ref.size != got.size {
				t.Fatalf("symbol is %d modules, libqrencode made %d", got.size, ref.size)
			}
			version := (got.size - 17) / 4

			refMask := readFormatMask(t, ref)
			gotMask := readFormatMask(t, got)
			if refMask == gotMask {
				agreed++
				if asciiArt(got) != asciiArt(ref) {
					t.Errorf("same mask %d but different modules\n--- got ---\n%s\n--- want ---\n%s",
						gotMask, asciiArt(got), asciiArt(ref))
				}
			}

			// Strip each symbol's own mask and compare what is underneath.
			refPlain := unmask(ref, refMask, version)
			gotPlain := unmask(got, gotMask, version)
			reserved := formatModules(got.size)
			for y := 0; y < got.size; y++ {
				for x := 0; x < got.size; x++ {
					if reserved[[2]int{x, y}] {
						continue // holds the mask number, so it must differ
					}
					if refPlain.At(x, y) != gotPlain.At(x, y) {
						t.Fatalf("unmasked module (%d,%d) is %v, libqrencode says %v (version %d, masks %d and %d)",
							x, y, gotPlain.At(x, y), refPlain.At(x, y), version, gotMask, refMask)
					}
				}
			}
		})
	}
	if agreed == 0 {
		t.Error("no fixture agreed on a mask, so the exact-match half of this test never ran")
	}
	t.Logf("mask agreed with libqrencode on %d of %d fixtures", agreed, len(fixtures))
}

// loadFixture parses the ASCII art libqrencode writes: two characters per
// module, "##" dark, one line per row.
func loadFixture(t *testing.T, name string) *Matrix {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name+".txt"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	m := &Matrix{size: len(lines), mods: make([]bool, len(lines)*len(lines))}
	for y, line := range lines {
		if len(line) != 2*m.size {
			t.Fatalf("fixture %s row %d is %d characters, want %d", name, y, len(line), 2*m.size)
		}
		for x := 0; x < m.size; x++ {
			m.set(x, y, line[2*x] == '#')
		}
	}
	return m
}

func asciiArt(m *Matrix) string {
	var b strings.Builder
	for y := 0; y < m.Size(); y++ {
		for x := 0; x < m.Size(); x++ {
			if m.At(x, y) {
				b.WriteString("##")
			} else {
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// readFormatMask recovers the mask number a finished symbol declares, from the
// first copy of the format information beside the top-left finder. It also
// checks the error correction level, which for this package is always M.
func readFormatMask(t *testing.T, m *Matrix) int {
	t.Helper()
	bit := func(i int) uint32 {
		var x, y int
		switch {
		case i <= 5:
			x, y = 8, i
		case i == 6:
			x, y = 8, 7
		case i == 7:
			x, y = 8, 8
		case i == 8:
			x, y = 7, 8
		default:
			x, y = 14-i, 8
		}
		if m.At(x, y) {
			return 1
		}
		return 0
	}
	var v uint32
	for i := 0; i < 15; i++ {
		v |= bit(i) << uint(i)
	}
	data := (v ^ 0b101010000010010) >> 10
	if level := data >> 3; level != 0b00 {
		t.Fatalf("format information says error correction level %02b, want M (00)", level)
	}
	return int(data & 7)
}

// unmask returns a copy of m with the given mask pattern removed from every
// module that is not a function pattern.
func unmask(m *Matrix, mask, version int) *Matrix {
	fnMap := newMatrix(version)
	drawFunctionPatterns(newMatrix(version), fnMap, version)
	out := &Matrix{size: m.size, mods: append([]bool(nil), m.mods...)}
	for y := 0; y < m.size; y++ {
		for x := 0; x < m.size; x++ {
			if !fnMap.At(x, y) && maskFn(mask, x, y) {
				out.set(x, y, !out.At(x, y))
			}
		}
	}
	return out
}

// formatModules is the set of positions holding format information, which
// encodes the mask number and so legitimately differs between two encoders that
// chose different masks.
func formatModules(size int) map[[2]int]bool {
	out := map[[2]int]bool{}
	for i := 0; i <= 8; i++ {
		if i != 6 {
			out[[2]int{i, 8}] = true
			out[[2]int{8, i}] = true
		}
	}
	for i := 0; i < 8; i++ {
		out[[2]int{size - 1 - i, 8}] = true
		out[[2]int{8, size - 1 - i}] = true
	}
	return out
}

func TestFixturesCoverEveryVersion(t *testing.T) {
	seen := map[int]bool{}
	for _, input := range fixtures {
		m, err := Encode([]byte(input))
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		seen[(m.Size()-17)/4] = true
	}
	for v := 1; v <= maxVersion; v++ {
		if !seen[v] {
			t.Errorf("no fixture produces a version %d symbol, so that row of the tables is unchecked", v)
		}
	}
}

// TestVersionTableIsSelfConsistent checks each row of the block table against
// its own totals and against the symbol geometry. A transposed digit in this
// table produces a symbol that scans as nothing, and this arithmetic is the
// cheapest way to catch one.
func TestVersionTableIsSelfConsistent(t *testing.T) {
	// Published byte-mode capacities at level M, ISO/IEC 18004 table 7.
	wantCapacity := [maxVersion + 1]int{
		1: 14, 2: 26, 3: 42, 4: 62, 5: 84, 6: 106,
		7: 122, 8: 152, 9: 180, 10: 213, 11: 251, 12: 287,
	}

	for v := 1; v <= maxVersion; v++ {
		c := versions[v]
		blocks := c.blocks1 + c.blocks2
		if got, want := c.dataCodewords()+blocks*c.ecWords, c.totalCodewords; got != want {
			t.Errorf("version %d: data plus error correction is %d codewords, total says %d", v, got, want)
		}
		if got := c.byteCapacity(v); got != wantCapacity[v] {
			t.Errorf("version %d: byte capacity %d, published %d", v, got, wantCapacity[v])
		}
		if c.blocks2 > 0 && c.words2 != c.words1+1 {
			t.Errorf("version %d: the second block group must hold exactly one more codeword than the first, got %d and %d", v, c.words1, c.words2)
		}
		// The symbol must have exactly as many free modules as there are
		// codeword bits plus remainder bits. This is what catches a wrong
		// alignment pattern row, which changes the function module count.
		size := 17 + 4*v
		if got, want := size*size-functionModuleCount(t, v), c.totalCodewords*8+remainderBits[v]; got != want {
			t.Errorf("version %d: %d free modules, but the codewords need %d bits plus %d remainder",
				v, got, c.totalCodewords*8, remainderBits[v])
		}
	}
}

func functionModuleCount(t *testing.T, version int) int {
	t.Helper()
	fn := newMatrix(version)
	drawFunctionPatterns(newMatrix(version), fn, version)
	n := 0
	for _, v := range fn.mods {
		if v {
			n++
		}
	}
	return n
}

// TestFormatBitsMatchStandard checks all eight level M format strings against
// ISO/IEC 18004 annex C table C.1. One wrong string makes a symbol that is
// correct in every data module and still unreadable.
func TestFormatBitsMatchStandard(t *testing.T) {
	want := []string{
		"101010000010010",
		"101000100100101",
		"101111001111100",
		"101101101001011",
		"100010111111001",
		"100000011001110",
		"100111110010111",
		"100101010100000",
	}
	for mask, w := range want {
		if s := bitString(formatBits(mask), 15); s != w {
			t.Errorf("mask %d: format bits %s, standard says %s", mask, s, w)
		}
	}
}

// TestVersionBitsMatchStandard checks the version information blocks against
// ISO/IEC 18004 annex D table D.1.
//
// These six strings were also read back out of the libqrencode fixtures, both
// copies of each, and they agree. That mattered: the version 11 string first
// written here was wrong, this test failed, and the fixture settled which side
// was in error. See testdata/PROVENANCE.md for the extraction.
func TestVersionBitsMatchStandard(t *testing.T) {
	want := map[int]string{
		7:  "000111110010010100",
		8:  "001000010110111100",
		9:  "001001101010011001",
		10: "001010010011010011",
		11: "001011101111110110",
		12: "001100011101100010",
	}
	for v := 7; v <= maxVersion; v++ {
		w, ok := want[v]
		if !ok {
			t.Fatalf("version %d has no expected version information string in this test", v)
		}
		if s := bitString(versionBits(v), 18); s != w {
			t.Errorf("version %d: version bits %s, standard says %s", v, s, w)
		}
	}
}

func bitString(v uint32, n int) string {
	var b strings.Builder
	for i := n - 1; i >= 0; i-- {
		if v&(1<<uint(i)) != 0 {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

// TestReedSolomonKnownAnswer uses the version 1 level M worked example that
// ISO/IEC 18004 annex I carries: sixteen data codewords in, ten error
// correction codewords out.
func TestReedSolomonKnownAnswer(t *testing.T) {
	data := []byte{32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17}
	want := []byte{196, 35, 39, 119, 235, 215, 231, 226, 93, 23}
	got := reedSolomon(data, 10)
	if len(got) != len(want) {
		t.Fatalf("got %d error correction codewords, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("error correction codewords %v, want %v", got, want)
		}
	}
}

func TestEncodeRefusesEmpty(t *testing.T) {
	if _, err := Encode(nil); !errors.Is(err, ErrEmpty) {
		t.Errorf("Encode(nil) error %v, want ErrEmpty", err)
	}
}

func TestEncodeRefusesOverlongData(t *testing.T) {
	limit := versions[maxVersion].byteCapacity(maxVersion)
	if _, err := Encode(make([]byte, limit+1)); !errors.Is(err, ErrTooLong) {
		t.Errorf("%d bytes: error %v, want ErrTooLong", limit+1, err)
	}
	// And the last byte that does fit must still encode, or the boundary is
	// off by one in the other direction.
	if _, err := Encode(make([]byte, limit)); err != nil {
		t.Errorf("%d bytes should fit the largest symbol this package builds: %v", limit, err)
	}
}

// TestLongestWiFiJoinStringFits is the capacity claim the package comment
// makes, checked rather than asserted: the longest join string 802.11 allows,
// with every character needing an escape, has to fit. This failed at version
// 10 by three bytes, which is why the tables go to 12.
func TestLongestWiFiJoinStringFits(t *testing.T) {
	ssid := strings.Repeat(";", 32) // 32 octets is the SSID maximum
	pass := strings.Repeat(";", 63) // 63 characters is the WPA passphrase maximum
	join := WiFiJoin(ssid, pass, false)
	m, err := Encode([]byte(join))
	if err != nil {
		t.Fatalf("the longest legal join string (%d bytes) must fit: %v", len(join), err)
	}
	t.Logf("worst case is %d bytes, encoded as a version %d symbol", len(join), (m.Size()-17)/4)
}

func TestWiFiJoinEscaping(t *testing.T) {
	cases := []struct {
		name       string
		ssid, pass string
		hidden     bool
		want       string
	}{
		{
			name: "plain",
			ssid: "Caspian", pass: "hunter2hunter2",
			want: "WIFI:T:WPA;S:Caspian;P:hunter2hunter2;H:false;;",
		},
		{
			// A semicolon in a passphrase is the case that joins the wrong
			// network or sends a truncated key if it is not escaped.
			name: "specials are escaped",
			ssid: `Cafe;Bar`, pass: `a,b:c\d"e`,
			want: `WIFI:T:WPA;S:Cafe\;Bar;P:a\,b\:c\\d\"e;H:false;;`,
		},
		{
			// A value that is all hex digits is read as a raw key unless it is
			// quoted.
			name: "hex-looking values are quoted",
			ssid: "cafe", pass: "beef1234",
			want: `WIFI:T:WPA;S:"cafe";P:"beef1234";H:false;;`,
		},
		{
			name: "hidden",
			ssid: "Caspian", pass: "hunter2hunter2", hidden: true,
			want: "WIFI:T:WPA;S:Caspian;P:hunter2hunter2;H:true;;",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := WiFiJoin(c.ssid, c.pass, c.hidden); got != c.want {
				t.Errorf("WiFiJoin\n got %s\nwant %s", got, c.want)
			}
		})
	}
}

// TestSVGCarriesNoInputText is a security property, not a formatting one. The
// panel inlines this fragment into a page as trusted markup, so a byte of the
// passphrase reaching the markup would be both an injection point and a
// disclosure in the page source.
func TestSVGCarriesNoInputText(t *testing.T) {
	const pass = "sun-rope-glass-mint-7412"
	const ssid = "Caspian-a41f"
	m, err := Encode([]byte(WiFiJoin(ssid, pass, false)))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	svg := m.SVG()
	for _, secret := range []string{pass, ssid, "WIFI:", "sun-rope"} {
		if strings.Contains(svg, secret) {
			t.Errorf("the SVG contains %q from the encoded data", secret)
		}
	}
	if !strings.HasPrefix(svg, "<svg ") || !strings.HasSuffix(svg, "</svg>") {
		t.Errorf("SVG is not a single element: %.40s...", svg)
	}
	if strings.Count(svg, "<svg") != 1 {
		t.Error("expected exactly one svg element")
	}
}

// TestSVGModuleCountMatchesMatrix guards the renderer against the matrix. A
// path with the wrong number of subpaths is a code nothing can read, and it
// looks perfectly plausible on screen.
func TestSVGModuleCountMatchesMatrix(t *testing.T) {
	m, err := Encode([]byte("HELLO"))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dark := 0
	for y := 0; y < m.Size(); y++ {
		for x := 0; x < m.Size(); x++ {
			if m.At(x, y) {
				dark++
			}
		}
	}
	if got := strings.Count(m.SVG(), "h1v1h-1z"); got != dark {
		t.Errorf("SVG draws %d modules, the matrix has %d dark", got, dark)
	}
}

func TestAtIsSafeOutsideTheSymbol(t *testing.T) {
	m, err := Encode([]byte("HELLO"))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {m.Size(), 0}, {0, m.Size()}} {
		if m.At(p[0], p[1]) {
			t.Errorf("At(%d,%d) is dark outside the symbol", p[0], p[1])
		}
	}
}
