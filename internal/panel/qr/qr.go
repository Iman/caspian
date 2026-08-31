// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

// Package qr encodes one QR symbol, in byte mode, at error correction level M,
// for versions 1 to 12.
//
// # Why this exists rather than a dependency
//
// The panel has to print a WiFi join code so a phone can join the hotspot with
// its camera (docs/2026-08-29-design.md section 5.2), and the panel is forbidden
// to fetch anything (section 5.7), so a hosted image or a remote generator is
// out. go.mod carries no QR library and the instruction for this package was
// not to add one, so the encoder is here.
//
// # What it deliberately does not do
//
// It encodes ONE case: 8-bit byte mode, error correction level M, versions 1 to
// 12, no ECI, no structured append, no Kanji or alphanumeric compaction, and no
// Micro QR. Version 12 at level M holds 287 bytes, against a worst case of 216
// for a WiFi join string: a 32-octet SSID and a 63-character WPA passphrase in
// which every single character needs a backslash escape. That number is
// measured by TestLongestWiFiJoinStringFits rather than estimated, and it is
// why the tables do not stop at version 10, where it did not fit.
//
// Encode returns ErrTooLong above that rather than silently truncating, which
// is the seam to widen if this package is ever asked to carry more: add rows to
// the tables in tables.go and, above version 26, a second character-count width.
//
// Level M was chosen over L because the symbol is read off a screen at arm's
// length, often at an angle, and M recovers about 15 percent of the symbol
// against L's 7. It was chosen over Q and H because those grow the symbol for a
// robustness nobody needs when the code is on a screen a foot away.
//
// # How it was checked
//
// Three ways, because each one misses what the others catch.
//
// The tables are checked against their own arithmetic and against the published
// capacities, which catches a transposed digit. The output is compared module
// for module against libqrencode 4.1.1, an independent implementation, over one
// fixture per version, which catches a misreading of the standard. And every
// fixture was rendered to PNG and decoded back to its exact input by OpenCV
// 5.0.0, which is the only check that speaks to what a phone will do; see
// testdata/PROVENANCE.md.
//
// One difference from libqrencode is expected and is not a defect: the two
// disagree about which data mask to choose for 6 of the 14 fixtures. Mask
// choice is a readability heuristic and every mask yields a valid symbol. The
// details, and why the golden comparison is done with the mask stripped off,
// are in testdata/PROVENANCE.md.
package qr

import (
	"errors"
	"fmt"
)

// ErrTooLong means the data does not fit in a version 10 symbol at level M.
var ErrTooLong = errors.New("qr: too much data for a version 10 symbol")

// ErrEmpty means there was nothing to encode. An empty QR code is legal in the
// standard and useless in this product, so it is refused: a blank join code on
// the panel would look like a rendering fault.
var ErrEmpty = errors.New("qr: nothing to encode")

// Matrix is a square grid of modules. A true module is dark.
type Matrix struct {
	size int
	mods []bool
}

// Size is the number of modules along one edge.
func (m *Matrix) Size() int { return m.size }

// At reports whether the module at column x, row y is dark. Coordinates
// outside the symbol report false, so a caller drawing a quiet zone does not
// have to bounds-check.
func (m *Matrix) At(x, y int) bool {
	if x < 0 || y < 0 || x >= m.size || y >= m.size {
		return false
	}
	return m.mods[y*m.size+x]
}

func (m *Matrix) set(x, y int, dark bool) { m.mods[y*m.size+x] = dark }

// Encode returns the module matrix for data.
//
// data is treated as raw bytes. The standard's byte mode is defined over
// ISO/IEC 8859-1, and every reader in practice sniffs UTF-8, which is what a
// WiFi join string carrying a non-ASCII SSID will be. Nothing here transcodes,
// so what the caller passes is what a phone reads.
func Encode(data []byte) (*Matrix, error) {
	if len(data) == 0 {
		return nil, ErrEmpty
	}
	v, err := chooseVersion(len(data))
	if err != nil {
		return nil, err
	}
	code := versions[v]

	bits := encodeData(data, v, code.dataCodewords())
	codewords := interleave(bits, code)

	m := newMatrix(v)
	fn := newMatrix(v) // records which modules are function patterns
	drawFunctionPatterns(m, fn, v)
	placeData(m, fn, codewords, remainderBits[v])

	best, bestMask := chooseMask(m, fn, v)
	drawFormat(best, bestMask)
	if v >= 7 {
		drawVersion(best, v)
	}
	return best, nil
}

// chooseVersion returns the smallest version whose byte-mode capacity at level
// M holds n bytes.
func chooseVersion(n int) (int, error) {
	for v := 1; v <= maxVersion; v++ {
		if n <= versions[v].byteCapacity(v) {
			return v, nil
		}
	}
	return 0, fmt.Errorf("%w: %d bytes, limit %d", ErrTooLong, n, versions[maxVersion].byteCapacity(maxVersion))
}

// ---------------------------------------------------------------------------
// Bit stream
// ---------------------------------------------------------------------------

type bitBuf struct {
	bytes []byte
	nbits int
}

func (b *bitBuf) push(value uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		bit := (value>>uint(i))&1 == 1
		if b.nbits%8 == 0 {
			b.bytes = append(b.bytes, 0)
		}
		if bit {
			b.bytes[b.nbits/8] |= 1 << uint(7-b.nbits%8)
		}
		b.nbits++
	}
}

// encodeData builds the data codeword stream: mode indicator, character count,
// the bytes, a terminator, byte alignment, then the alternating pad codewords
// the standard names.
func encodeData(data []byte, version, dataCodewords int) []byte {
	var b bitBuf
	b.push(modeByte, 4)
	b.push(uint32(len(data)), countBits(version))
	for _, c := range data {
		b.push(uint32(c), 8)
	}

	// Terminator: up to four zero bits, and fewer when the stream is nearly
	// full. A version chosen by chooseVersion always leaves room for the data,
	// so capacity-nbits cannot be negative here; min keeps it from becoming a
	// negative push if that ever stops being true.
	capacity := dataCodewords * 8
	b.push(0, max(0, min(4, capacity-b.nbits)))

	// Pad to a byte boundary.
	if rem := b.nbits % 8; rem != 0 {
		b.push(0, 8-rem)
	}

	// Pad codewords, 0xEC and 0x11 alternating, from ISO/IEC 18004 section
	// 8.4.9. The values are fixed by the standard and are not arbitrary
	// filler: a reader uses them to confirm it read the length correctly.
	pad := []byte{0xEC, 0x11}
	for i := 0; len(b.bytes) < dataCodewords; i++ {
		b.bytes = append(b.bytes, pad[i%2])
	}
	return b.bytes
}

const modeByte = 0b0100

// countBits is the width of the character count indicator in byte mode: 8 bits
// up to version 9, 16 from version 10. This package stops at 10, so the second
// case has exactly one member; it is written as a range anyway because getting
// it wrong produces a symbol that scans as garbage rather than failing.
func countBits(version int) int {
	if version <= 9 {
		return 8
	}
	return 16
}

// interleave splits the data codewords into blocks, appends the error
// correction codewords for each, and reorders both into the single stream the
// standard places into the symbol.
//
// The reordering is what makes a burst of damage survivable: consecutive
// modules in the symbol come from different blocks, so a scratch that destroys
// a run of modules costs each block a few codewords rather than costing one
// block all of them.
func interleave(data []byte, c versionCode) []byte {
	blocks := make([][]byte, 0, c.blocks1+c.blocks2)
	ec := make([][]byte, 0, c.blocks1+c.blocks2)

	off := 0
	for i := 0; i < c.blocks1; i++ {
		blk := data[off : off+c.words1]
		off += c.words1
		blocks = append(blocks, blk)
		ec = append(ec, reedSolomon(blk, c.ecWords))
	}
	for i := 0; i < c.blocks2; i++ {
		blk := data[off : off+c.words2]
		off += c.words2
		blocks = append(blocks, blk)
		ec = append(ec, reedSolomon(blk, c.ecWords))
	}

	out := make([]byte, 0, c.totalCodewords)
	maxData := max(c.words1, c.words2)
	for i := 0; i < maxData; i++ {
		for _, blk := range blocks {
			if i < len(blk) {
				out = append(out, blk[i])
			}
		}
	}
	for i := 0; i < c.ecWords; i++ {
		for _, blk := range ec {
			out = append(out, blk[i])
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Function patterns
// ---------------------------------------------------------------------------

func newMatrix(version int) *Matrix {
	size := 17 + 4*version
	return &Matrix{size: size, mods: make([]bool, size*size)}
}

// drawFunctionPatterns writes every module whose value is fixed by the
// standard, and records in fn which modules those are so that data placement
// skips them. The format and version areas are reserved in fn here and written
// later, once the mask is known.
func drawFunctionPatterns(m, fn *Matrix, version int) {
	size := m.size

	// Three finder patterns with their separators.
	for _, p := range [][2]int{{0, 0}, {size - 7, 0}, {0, size - 7}} {
		drawFinder(m, fn, p[0], p[1])
	}

	// Timing patterns: alternating modules on row 6 and column 6.
	for i := 8; i < size-8; i++ {
		dark := i%2 == 0
		m.set(i, 6, dark)
		fn.set(i, 6, true)
		m.set(6, i, dark)
		fn.set(6, i, true)
	}

	// Alignment patterns, skipping the three that would sit on a finder.
	centres := alignment[version]
	for _, cy := range centres {
		for _, cx := range centres {
			if (cx == 6 && cy == 6) ||
				(cx == 6 && cy == size-7) ||
				(cx == size-7 && cy == 6) {
				continue
			}
			drawAlignment(m, fn, cx, cy)
		}
	}

	// The dark module. Always set, always at this position.
	m.set(8, size-8, true)
	fn.set(8, size-8, true)

	// Reserve the format information areas.
	for i := 0; i <= 8; i++ {
		if i != 6 {
			fn.set(i, 8, true)
			fn.set(8, i, true)
		}
	}
	for i := 0; i < 8; i++ {
		fn.set(size-1-i, 8, true)
		fn.set(8, size-1-i, true)
	}

	// Reserve the version information areas.
	if version >= 7 {
		for i := 0; i < 6; i++ {
			for j := 0; j < 3; j++ {
				fn.set(size-11+j, i, true)
				fn.set(i, size-11+j, true)
			}
		}
	}
}

func drawFinder(m, fn *Matrix, ox, oy int) {
	// The separator is the one-module light border, so the loop runs from -1
	// to 7 and clips. Drawing it explicitly rather than relying on the zero
	// value matters because fn has to record those modules as function
	// patterns too.
	for dy := -1; dy <= 7; dy++ {
		for dx := -1; dx <= 7; dx++ {
			x, y := ox+dx, oy+dy
			if x < 0 || y < 0 || x >= m.size || y >= m.size {
				continue
			}
			inRing := dx >= 0 && dx <= 6 && dy >= 0 && dy <= 6 &&
				(dx == 0 || dx == 6 || dy == 0 || dy == 6)
			inCore := dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4
			m.set(x, y, inRing || inCore)
			fn.set(x, y, true)
		}
	}
}

func drawAlignment(m, fn *Matrix, cx, cy int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			dark := dx == -2 || dx == 2 || dy == -2 || dy == 2 || (dx == 0 && dy == 0)
			m.set(cx+dx, cy+dy, dark)
			fn.set(cx+dx, cy+dy, true)
		}
	}
}

// ---------------------------------------------------------------------------
// Data placement
// ---------------------------------------------------------------------------

// placeData walks the symbol in the standard's order: two-module-wide columns
// from the right edge leftwards, alternating upwards and downwards, skipping
// column 6 because the vertical timing pattern occupies it, and skipping every
// module already claimed by a function pattern.
func placeData(m, fn *Matrix, codewords []byte, remainder int) {
	size := m.size
	bit := 0
	total := len(codewords)*8 + remainder

	up := true
	for right := size - 1; right >= 1; right -= 2 {
		if right == 6 {
			// Column 6 is the timing pattern, so the pair of columns shifts
			// one to the left for the rest of the walk.
			right = 5
		}
		for i := 0; i < size; i++ {
			y := i
			if up {
				y = size - 1 - i
			}
			for dx := 0; dx < 2; dx++ {
				x := right - dx
				if fn.At(x, y) {
					continue
				}
				dark := false
				if bit < len(codewords)*8 {
					dark = codewords[bit/8]&(1<<uint(7-bit%8)) != 0
				}
				// Remainder bits past the last codeword stay light, which is
				// what the zero value already gives; the branch above simply
				// stops reading off the end of the slice.
				m.set(x, y, dark)
				bit++
				if bit >= total {
					return
				}
			}
		}
		up = !up
	}
}

// ---------------------------------------------------------------------------
// Masking
// ---------------------------------------------------------------------------

// maskFn reports whether the module at (x, y) is inverted by mask pattern n.
// The eight formulas are from ISO/IEC 18004 table 10. Note the convention: i is
// the row and j the column, so the arguments are (y, x) in that order.
func maskFn(n, x, y int) bool {
	i, j := y, x
	switch n {
	case 0:
		return (i+j)%2 == 0
	case 1:
		return i%2 == 0
	case 2:
		return j%3 == 0
	case 3:
		return (i+j)%3 == 0
	case 4:
		return (i/2+j/3)%2 == 0
	case 5:
		return (i*j)%2+(i*j)%3 == 0
	case 6:
		return ((i*j)%2+(i*j)%3)%2 == 0
	case 7:
		return ((i+j)%2+(i*j)%3)%2 == 0
	}
	return false
}

// chooseMask applies each of the eight masks to a copy of the symbol, scores
// it, and returns the copy with the lowest penalty.
//
// The format bits are written into each candidate before scoring. They have to
// be: penalty rule 3 looks for a particular pattern anywhere in a row or
// column, and the format area is part of those rows and columns, so scoring a
// symbol with a blank format area scores a symbol that will never exist.
func chooseMask(m, fn *Matrix, version int) (*Matrix, int) {
	var best *Matrix
	bestScore := -1
	bestMask := 0
	for n := 0; n < 8; n++ {
		cand := &Matrix{size: m.size, mods: append([]bool(nil), m.mods...)}
		for y := 0; y < cand.size; y++ {
			for x := 0; x < cand.size; x++ {
				if !fn.At(x, y) && maskFn(n, x, y) {
					cand.set(x, y, !cand.At(x, y))
				}
			}
		}
		drawFormat(cand, n)
		if version >= 7 {
			drawVersion(cand, version)
		}
		s := penalty(cand)
		if bestScore < 0 || s < bestScore {
			best, bestScore, bestMask = cand, s, n
		}
	}
	return best, bestMask
}

// Penalty weights from ISO/IEC 18004 table 11.
const (
	penaltyRun      = 3  // N1: a run of five or more, plus one per extra module
	penaltyBlock    = 3  // N2: each 2x2 block of one colour
	penaltyFinder   = 40 // N3: each finder-like pattern in a row or column
	penaltyBalance  = 10 // N4: per 5 percent of deviation from half dark
	penaltyRunStart = 5
)

func penalty(m *Matrix) int {
	return penaltyRuns(m) + penaltyBlocks(m) + penaltyFinderLike(m) + penaltyDarkRatio(m)
}

func penaltyRuns(m *Matrix) int {
	total := 0
	scan := func(get func(a, b int) bool) {
		for a := 0; a < m.size; a++ {
			run := 1
			for b := 1; b < m.size; b++ {
				if get(a, b) == get(a, b-1) {
					run++
					continue
				}
				if run >= penaltyRunStart {
					total += penaltyRun + (run - penaltyRunStart)
				}
				run = 1
			}
			if run >= penaltyRunStart {
				total += penaltyRun + (run - penaltyRunStart)
			}
		}
	}
	scan(func(y, x int) bool { return m.At(x, y) }) // rows
	scan(func(x, y int) bool { return m.At(x, y) }) // columns
	return total
}

func penaltyBlocks(m *Matrix) int {
	total := 0
	for y := 0; y < m.size-1; y++ {
		for x := 0; x < m.size-1; x++ {
			v := m.At(x, y)
			if m.At(x+1, y) == v && m.At(x, y+1) == v && m.At(x+1, y+1) == v {
				total += penaltyBlock
			}
		}
	}
	return total
}

// finderLike is the 1:1:3:1:1 ratio of the finder pattern followed by four
// light modules, and its mirror. Both count, in both directions.
var finderLike = [2][11]bool{
	{true, false, true, true, true, false, true, false, false, false, false},
	{false, false, false, false, true, false, true, true, true, false, true},
}

func penaltyFinderLike(m *Matrix) int {
	total := 0
	check := func(get func(i, j int) bool) {
		for a := 0; a < m.size; a++ {
			for b := 0; b+11 <= m.size; b++ {
				for _, want := range finderLike {
					hit := true
					for k := 0; k < 11; k++ {
						if get(a, b+k) != want[k] {
							hit = false
							break
						}
					}
					if hit {
						total += penaltyFinder
					}
				}
			}
		}
	}
	check(func(y, x int) bool { return m.At(x, y) })
	check(func(x, y int) bool { return m.At(x, y) })
	return total
}

func penaltyDarkRatio(m *Matrix) int {
	dark := 0
	for _, v := range m.mods {
		if v {
			dark++
		}
	}
	total := m.size * m.size
	// Integer arithmetic throughout: percent is floored, which is what the
	// standard's "previous multiple of five" wording asks for, and it avoids
	// a float comparison deciding a mask.
	percent := dark * 100 / total
	dev := percent - 50
	if dev < 0 {
		dev = -dev
	}
	return dev / 5 * penaltyBalance
}

// ---------------------------------------------------------------------------
// Format and version information
// ---------------------------------------------------------------------------

// formatBits returns the 15-bit format information for error correction level
// M and the given mask: five data bits, ten BCH(15,5) check bits, XORed with
// the standard's fixed mask so that an all-zero format is not a legal symbol.
func formatBits(mask int) uint32 {
	const ecLevelM = 0b00
	data := uint32(ecLevelM)<<3 | uint32(mask)
	rem := data << 10
	for i := 14; i >= 10; i-- {
		if rem&(1<<uint(i)) != 0 {
			rem ^= 0b10100110111 << uint(i-10)
		}
	}
	return (data<<10 | rem) ^ 0b101010000010010
}

func drawFormat(m *Matrix, mask int) {
	bits := formatBits(mask)
	size := m.size
	get := func(i int) bool { return bits&(1<<uint(i)) != 0 }

	// First copy, around the top-left finder.
	for i := 0; i <= 5; i++ {
		m.set(8, i, get(i))
	}
	m.set(8, 7, get(6))
	m.set(8, 8, get(7))
	m.set(7, 8, get(8))
	for i := 9; i <= 14; i++ {
		m.set(14-i, 8, get(i))
	}

	// Second copy, split between the other two finders, so that losing one
	// corner does not lose the format.
	for i := 0; i <= 7; i++ {
		m.set(size-1-i, 8, get(i))
	}
	for i := 8; i <= 14; i++ {
		m.set(8, size-15+i, get(i))
	}
	m.set(8, size-8, true) // the dark module, rewritten in case the loop crossed it
}

// versionBits returns the 18-bit version information for version 7 and above:
// six data bits and twelve BCH(18,6) check bits.
func versionBits(version int) uint32 {
	rem := uint32(version) << 12
	for i := 17; i >= 12; i-- {
		if rem&(1<<uint(i)) != 0 {
			rem ^= 0b1111100100101 << uint(i-12)
		}
	}
	return uint32(version)<<12 | rem
}

func drawVersion(m *Matrix, version int) {
	bits := versionBits(version)
	size := m.size
	for i := 0; i < 18; i++ {
		bit := bits&(1<<uint(i)) != 0
		a, b := i/3, i%3
		m.set(size-11+b, a, bit)
		m.set(a, size-11+b, bit)
	}
}
