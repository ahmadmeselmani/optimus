package tui

import "strings"

// Braille art (assets/optimus_*.txt) packs 8 sub-pixels — a 2-wide x
// 4-tall dot grid — into each Unicode Braille Patterns character
// (U+2800 + a bitmask). This file decodes that into a plain pixel bitmap,
// resamples it to any target size, and re-encodes it back to braille text —
// letting the mascot scale to fit whatever space is actually available
// (a terminal window, a header line) instead of picking between a couple of
// fixed-size assets or cropping an arbitrary slice.

const brailleBase = 0x2800

// brailleDotBit maps each (row, col) sub-pixel position in a braille cell's
// 2x4 dot grid to its bit index in the character code, per the standard
// Braille Patterns dot numbering (1-2-3-7 down the left column, 4-5-6-8
// down the right).
var brailleDotBit = [4][2]uint8{
	{0, 3},
	{1, 4},
	{2, 5},
	{6, 7},
}

// decodeBraille converts a rectangular block of braille-pattern text into a
// boolean pixel bitmap, 2 pixels wide and 4 pixels tall per character.
func decodeBraille(lines []string) [][]bool {
	rows := len(lines)
	if rows == 0 {
		return nil
	}
	cols := mascotWidth(lines)
	px := make([][]bool, rows*4)
	for i := range px {
		px[i] = make([]bool, cols*2)
	}
	for r, line := range lines {
		for c, ch := range []rune(line) {
			var bits uint8
			if ch >= brailleBase && ch <= brailleBase+0xFF {
				bits = uint8(ch - brailleBase)
			}
			for dr := 0; dr < 4; dr++ {
				for dc := 0; dc < 2; dc++ {
					px[r*4+dr][c*2+dc] = bits&(1<<brailleDotBit[dr][dc]) != 0
				}
			}
		}
	}
	return px
}

// encodeBraille converts a boolean pixel bitmap back into braille-pattern
// text lines, packing every 2x4 block of pixels into one character.
func encodeBraille(px [][]bool) []string {
	h := len(px)
	if h == 0 {
		return nil
	}
	w := len(px[0])
	rows := (h + 3) / 4
	cols := (w + 1) / 2
	out := make([]string, rows)
	for r := 0; r < rows; r++ {
		var b strings.Builder
		for c := 0; c < cols; c++ {
			var bits uint8
			for dr := 0; dr < 4; dr++ {
				for dc := 0; dc < 2; dc++ {
					y, x := r*4+dr, c*2+dc
					if y < h && x < w && px[y][x] {
						bits |= 1 << brailleDotBit[dr][dc]
					}
				}
			}
			b.WriteRune(rune(brailleBase + int(bits)))
		}
		out[r] = b.String()
	}
	return out
}

// scalePixelsTo resamples a boolean bitmap to an exact targetH x targetW,
// via box averaging: each destination pixel is "on" if at least a third of
// its corresponding source region is on. That threshold (rather than a
// majority) keeps thin strokes from disappearing entirely when downscaling
// by a large factor. Dimensions don't need to share an aspect ratio — a
// disproportionate target stretches or squashes the image, which is fine
// for a small abstract icon.
func scalePixelsTo(src [][]bool, targetH, targetW int) [][]bool {
	srcH := len(src)
	if srcH == 0 || targetH <= 0 || targetW <= 0 {
		return nil
	}
	srcW := len(src[0])
	dst := make([][]bool, targetH)
	for y := 0; y < targetH; y++ {
		dst[y] = make([]bool, targetW)
		y0, y1 := y*srcH/targetH, (y+1)*srcH/targetH
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < targetW; x++ {
			x0, x1 := x*srcW/targetW, (x+1)*srcW/targetW
			if x1 <= x0 {
				x1 = x0 + 1
			}
			on, total := 0, 0
			for sy := y0; sy < y1 && sy < srcH; sy++ {
				for sx := x0; sx < x1 && sx < srcW; sx++ {
					total++
					if src[sy][sx] {
						on++
					}
				}
			}
			dst[y][x] = total > 0 && on*3 >= total
		}
	}
	return dst
}

// scaleMascotToFit downscales art to fit within maxRows x maxCols
// characters, preserving its aspect ratio (never upscales — art that
// already fits is returned unchanged, so a big enough terminal still gets
// full source detail instead of a needless re-encode).
func scaleMascotToFit(lines []string, maxRows, maxCols int) []string {
	srcRows := len(lines)
	srcCols := mascotWidth(lines)
	if srcRows == 0 || srcCols == 0 || maxRows <= 0 || maxCols <= 0 {
		return lines
	}
	if srcRows <= maxRows && srcCols <= maxCols {
		return lines
	}

	scale := min(float64(maxRows)/float64(srcRows), float64(maxCols)/float64(srcCols))
	dstRows, dstCols := max(1, int(float64(srcRows)*scale)), max(1, int(float64(srcCols)*scale))

	px := scalePixelsTo(decodeBraille(lines), dstRows*4, dstCols*2)
	return encodeBraille(px)
}
