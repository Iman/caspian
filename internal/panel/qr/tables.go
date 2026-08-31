// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package qr

// maxVersion is the largest symbol this package builds. See the package
// comment for why it stops here and what widening it would take.
//
// It is 12 rather than 10 because of a measurement, not a guess. The worst
// case this product can hand the encoder is a 32-octet SSID and a 63-character
// WPA passphrase in which every character needs a backslash escape, which is
// 216 bytes of join string. Version 10 at level M holds 213, so version 10 was
// three bytes short and TestLongestWiFiJoinStringFits failed on it. Version 12
// holds 287.
const maxVersion = 12

// versionCode is the block structure of one version at error correction level
// M, from ISO/IEC 18004 tables 7 to 9.
//
// Only level M is tabulated. Adding another level means adding another table,
// not editing this one, because every number here except totalCodewords
// changes with the level.
type versionCode struct {
	// totalCodewords is data plus error correction, which is fixed by the
	// version alone.
	totalCodewords int

	// ecWords is the number of error correction codewords in every block.
	ecWords int

	// A version has one or two block sizes. blocks1 blocks hold words1 data
	// codewords each, blocks2 blocks hold words2. blocks2 is zero when they
	// are all the same size.
	blocks1, words1 int
	blocks2, words2 int
}

func (c versionCode) dataCodewords() int {
	return c.blocks1*c.words1 + c.blocks2*c.words2
}

// byteCapacity is how many bytes of user data fit, after the four-bit mode
// indicator and the character count indicator are taken off the front.
func (c versionCode) byteCapacity(version int) int {
	return (c.dataCodewords()*8 - 4 - countBits(version)) / 8
}

// versions is indexed by version number, so index 0 is unused.
//
// Each row was checked against its own arithmetic by TestVersionTableIsSelf
// Consistent: totalCodewords minus ecWords times the block count must equal
// the data codewords the block fields describe, and byteCapacity must match
// the published byte-mode capacity for level M.
var versions = [maxVersion + 1]versionCode{
	1:  {totalCodewords: 26, ecWords: 10, blocks1: 1, words1: 16},
	2:  {totalCodewords: 44, ecWords: 16, blocks1: 1, words1: 28},
	3:  {totalCodewords: 70, ecWords: 26, blocks1: 1, words1: 44},
	4:  {totalCodewords: 100, ecWords: 18, blocks1: 2, words1: 32},
	5:  {totalCodewords: 134, ecWords: 24, blocks1: 2, words1: 43},
	6:  {totalCodewords: 172, ecWords: 16, blocks1: 4, words1: 27},
	7:  {totalCodewords: 196, ecWords: 18, blocks1: 4, words1: 31},
	8:  {totalCodewords: 242, ecWords: 22, blocks1: 2, words1: 38, blocks2: 2, words2: 39},
	9:  {totalCodewords: 292, ecWords: 22, blocks1: 3, words1: 36, blocks2: 2, words2: 37},
	10: {totalCodewords: 346, ecWords: 26, blocks1: 4, words1: 43, blocks2: 1, words2: 44},
	11: {totalCodewords: 404, ecWords: 30, blocks1: 1, words1: 50, blocks2: 4, words2: 51},
	12: {totalCodewords: 466, ecWords: 22, blocks1: 6, words1: 36, blocks2: 2, words2: 37},
}

// remainderBits is the number of unused bits after the last codeword, which
// are written as light modules. ISO/IEC 18004 table 1.
var remainderBits = [maxVersion + 1]int{
	1: 0, 2: 7, 3: 7, 4: 7, 5: 7, 6: 7, 7: 0, 8: 0, 9: 0, 10: 0, 11: 0, 12: 0,
}

// alignment holds the row and column centres of the alignment patterns for
// each version. Every combination of a centre row and a centre column carries
// a pattern except the three that would overlap a finder. ISO/IEC 18004 annex
// E.
var alignment = [maxVersion + 1][]int{
	1:  nil,
	2:  {6, 18},
	3:  {6, 22},
	4:  {6, 26},
	5:  {6, 30},
	6:  {6, 34},
	7:  {6, 22, 38},
	8:  {6, 24, 42},
	9:  {6, 26, 46},
	10: {6, 28, 50},
	11: {6, 30, 54},
	12: {6, 32, 58},
}

// ---------------------------------------------------------------------------
// Reed-Solomon over GF(256)
// ---------------------------------------------------------------------------

// The field is GF(2^8) with the primitive polynomial x^8+x^4+x^3+x^2+1, which
// is 0x11D, and 2 as the generator. Those two choices are fixed by ISO/IEC
// 18004 section 8.5.2; they are not tunable.
const gfPrimitive = 0x11D

var (
	gfExp [512]byte // gfExp[i] is 2^i, doubled in length so a sum of two logs never needs a modulo
	gfLog [256]byte
)

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(x)
		gfLog[x] = byte(i)
		x <<= 1
		if x&0x100 != 0 {
			x ^= gfPrimitive
		}
	}
	for i := 255; i < 512; i++ {
		gfExp[i] = gfExp[i-255]
	}
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// generatorPoly returns the coefficients of the degree-n generator polynomial,
// highest power first: the product of (x - 2^i) for i in 0..n-1.
func generatorPoly(n int) []byte {
	poly := []byte{1}
	for i := 0; i < n; i++ {
		next := make([]byte, len(poly)+1)
		for j, c := range poly {
			next[j] ^= c
			next[j+1] ^= gfMul(c, gfExp[i])
		}
		poly = next
	}
	return poly
}

// reedSolomon returns the n error correction codewords for one block: the
// remainder of the block's polynomial divided by the generator polynomial.
func reedSolomon(data []byte, n int) []byte {
	gen := generatorPoly(n)
	rem := make([]byte, len(data)+n)
	copy(rem, data)
	for i := 0; i < len(data); i++ {
		lead := rem[i]
		if lead == 0 {
			continue
		}
		for j, g := range gen {
			rem[i+j] ^= gfMul(g, lead)
		}
	}
	return rem[len(data):]
}
