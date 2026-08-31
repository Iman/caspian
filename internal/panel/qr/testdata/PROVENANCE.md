# Where these fixtures came from

Every `.txt` file here is the output of **libqrencode 4.1.1** (`qrencode`), an
independent QR encoder by Kentaro Fukuchi. None of them was produced by this
package, which is the whole point: a golden file this package generated would
only prove it still does what it did yesterday.

Generated on 2026-08-30 on the developer Mac, with:

```
printf '%s' "$DATA" | qrencode -8 -l M -m 0 -t ASCII -r /dev/stdin -o NAME.txt
```

- `-8` forces 8-bit byte mode, which is the only mode this package implements.
- `-l M` is error correction level M, the only level this package implements.
- `-m 0` removes the quiet zone, so the file is exactly the symbol.
- `-t ASCII` writes two characters per module, `##` dark and two spaces light.
- `-r /dev/stdin` with `printf '%s'` avoids the trailing newline that a here
  string or an argument would add. A single extra byte changes the version.

The exact input for each file is in the `fixtures` map in `qr_test.go`. Keeping
the inputs in Go rather than in a file beside the image avoids a second copy
drifting from the first.

## Version coverage

One fixture per version, 1 to 12. The version is `(rows - 17) / 4`.

| Version | Fixtures |
|---|---|
| 1 | hello |
| 2 | v2 |
| 3 | v3 |
| 4 | wifi_short, wifi_typical, escaped |
| 5 | v5 |
| 6 | v6 |
| 7 | wifi_long |
| 8 | v8 |
| 9 | wifi_max |
| 10 | v10 |
| 11 | v11 |
| 12 | v12 |

`TestFixturesCoverEveryVersion` fails if a version loses its fixture.

## The mask disagreement, measured

`qrencode` and this package do not always choose the same data mask. Measured
2026-08-30 over these 14 fixtures: they agreed on 8 and differed on 6.

That is not a defect in either. Mask choice is a readability heuristic and all
eight masks produce a valid symbol, provided the format information says which
one was used. The cause of the difference is penalty rule N3. This package
implements the standard's own worked pattern, the eleven modules `10111010000`
and its mirror, which is what ZXing and the common Go encoders do.
libqrencode's `mask.c` generalises the 1:1:3:1:1 ratio to any scale and asks
for a light run of four times that scale, so it scores some symbols
differently.

`TestMatchesLibqrencodeContent` therefore compares in two parts: the two
symbols must be identical module for module once each has its own mask stripped
off, and byte identical where the mask happens to agree.

## The decoder round trip

The content comparison proves this package agrees with another encoder. It does
not prove a reader can read the result, which is the only property the product
actually needs. That was checked separately, by hand, on 2026-08-30:

```
QR_DUMP_DIR=/some/empty/dir go test ./internal/panel/qr/ -run TestDumpPNG
```

then, with OpenCV 5.0.0 (`cv2.QRCodeDetector`):

```python
import cv2, glob
det = cv2.QRCodeDetector()
for f in sorted(glob.glob('*.png')):
    want = open(f[:-4] + '.input').read()
    txt, _, _ = det.detectAndDecode(cv2.imread(f, cv2.IMREAD_GRAYSCALE))
    print('OK' if txt == want else 'FAIL', f)
```

Result: 14 of 14 decoded back to the exact input, byte for byte, including the
six symbols whose mask differs from libqrencode's.

The decoder was proved working first, on a PNG that libqrencode produced rather
than one of ours, because a decoder that returns nothing for everything would
otherwise have looked like an encoder fault. `zbarimg` 0.23.93 was tried first
and was not used: it segmentation faults on this machine on libqrencode's own
output, so it says nothing about anything.

## The version information strings

The 18-bit version information blocks for versions 7 to 12 in
`TestVersionBitsMatchStandard` were cross-checked against these fixtures, both
copies in each symbol. That check earned its keep: the version 11 string first
written into the test from memory was wrong, and the fixture is what settled it.
