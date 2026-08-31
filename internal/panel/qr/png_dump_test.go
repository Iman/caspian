// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package qr

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestDumpPNG writes every fixture as a PNG so that a real QR decoder can read
// it back.
//
// It exists because no test in this package can prove the one thing that
// actually matters, which is that a phone reads the symbol. The golden fixtures
// prove agreement with another encoder, and the tables prove agreement with the
// standard, but only a decoder closes the loop. There is no decoder in this
// tree and adding a dependency for one is not worth it, so the check is run by
// hand and this test is the half of it that lives in the repo.
//
// It is skipped unless QR_DUMP_DIR is set, so it costs nothing in an ordinary
// run. The other half is in testdata/PROVENANCE.md: the exact commands, and the
// result from the last time they were run. Run both after any change to the
// encoder.
func TestDumpPNG(t *testing.T) {
	dir := os.Getenv("QR_DUMP_DIR")
	if dir == "" {
		t.Skip("set QR_DUMP_DIR to write PNGs for an external decoder; see testdata/PROVENANCE.md")
	}
	// Eight pixels a module, so a decoder is not fighting the sampling. The
	// quiet zone is included because a decoder without one often fails, and a
	// failure caused by the harness would be read as a failure of the encoder.
	const scale = 8
	for name, input := range fixtures {
		m, err := Encode([]byte(input))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		span := (m.Size() + 2*QuietZone) * scale
		img := image.NewGray(image.Rect(0, 0, span, span))
		for i := range img.Pix {
			img.Pix[i] = 0xFF
		}
		for y := 0; y < m.Size(); y++ {
			for x := 0; x < m.Size(); x++ {
				if !m.At(x, y) {
					continue
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						img.SetGray((x+QuietZone)*scale+dx, (y+QuietZone)*scale+dy, color.Gray{})
					}
				}
			}
		}
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, img)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		// The expected text beside each image, so the checking script needs no
		// copy of this table.
		if err := os.WriteFile(filepath.Join(dir, name+".input"), []byte(input), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("wrote %d symbols to %s", len(fixtures), dir)
}
