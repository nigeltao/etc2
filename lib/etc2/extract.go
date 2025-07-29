// Copyright 2025 The Etc2 Authors.
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
// modified, or distributed except according to those terms.
//
// SPDX-License-Identifier: Apache-2.0

package etc2

import (
	"image"
)

// makeExtract returns a closure that extracts the 4×4 block from src with the
// given top-left corner, writing the data to pixels.
//
// Out-of-bound pixels right of and below the image are substituted with the
// nearest in-bound pixel from the right and bottom edges.
func (f Format) makeExtract(pixels *[64]byte, src image.Image) func(blockX int, blockY int) {
	// We use the ITU-R BT.709 constants for conversion from color to gray,
	// which matches the ImageMagick "convert" program (and ImageMagick's
	// MagickCore/colorspace.c) used by
	// https://github.com/nigeltao/ETCPACK.git
	//
	// These RGB-to-gray constants are different from that used by the Go
	// standard library's image/color package (which follows ITU-R BT.601, the
	// same as JFIF): 0.299 0.587 0.114
	//
	// Using BT.709 means that this package's encoder produces exactly the same
	// output as the ETCPACK C++ program (which shells out to "convert").
	const grayR, grayG, grayB, graySum = 212656, 715158, 72186, 1000000

	maxPoint := src.Bounds().Max
	mX1 := maxPoint.X - 1
	mY1 := maxPoint.Y - 1

	if (f & formatBitDepth11) != 0 {
		twoChannel := (f & formatBitDepth11TwoChannel) != 0

		if srcNRGBA, ok := src.(*image.NRGBA); ok {
			return func(blockX int, blockY int) {
				for y := range 4 {
					for x := range 4 {
						i := (8 * y) + (2 * x)
						c := srcNRGBA.NRGBAAt(min(mX1, blockX+x), min(mY1, blockY+y))
						if twoChannel {
							pixels[i+0x00] = c.R
							pixels[i+0x01] = c.R
							pixels[i+0x20] = c.G
							pixels[i+0x21] = c.G
						} else {
							gray := ((graySum / 2) +
								(uint64(c.R) * 0x101 * grayR) +
								(uint64(c.G) * 0x101 * grayG) +
								(uint64(c.B) * 0x101 * grayB)) / graySum
							pixels[i+0x00] = uint8(gray >> 8)
							pixels[i+0x01] = uint8(gray >> 0)
						}
					}
				}
			}

		} else if srcNRGBA64, ok := src.(*image.NRGBA64); ok {
			return func(blockX int, blockY int) {
				for y := range 4 {
					for x := range 4 {
						i := (8 * y) + (2 * x)
						c := srcNRGBA64.NRGBA64At(min(mX1, blockX+x), min(mY1, blockY+y))
						if twoChannel {
							pixels[i+0x00] = uint8(c.R >> 8)
							pixels[i+0x01] = uint8(c.R >> 0)
							pixels[i+0x20] = uint8(c.G >> 8)
							pixels[i+0x21] = uint8(c.G >> 0)
						} else {
							gray := ((graySum / 2) +
								(uint64(c.R) * grayR) +
								(uint64(c.G) * grayG) +
								(uint64(c.B) * grayB)) / graySum
							pixels[i+0x00] = uint8(gray >> 8)
							pixels[i+0x01] = uint8(gray >> 0)
						}
					}
				}
			}

		} else if srcRGBA64, ok := src.(image.RGBA64Image); ok {
			return func(blockX int, blockY int) {
				for y := range 4 {
					for x := range 4 {
						i := (8 * y) + (2 * x)
						c := srcRGBA64.RGBA64At(min(mX1, blockX+x), min(mY1, blockY+y))
						if (c.A != 0x0000) && (c.A != 0xFFFF) {
							c.R = uint16((uint32(c.R) * 0xFFFF) / uint32(c.A))
							c.G = uint16((uint32(c.G) * 0xFFFF) / uint32(c.A))
							c.B = uint16((uint32(c.B) * 0xFFFF) / uint32(c.A))
						}
						if twoChannel {
							pixels[i+0x00] = uint8(c.R >> 8)
							pixels[i+0x01] = uint8(c.R >> 0)
							pixels[i+0x20] = uint8(c.G >> 8)
							pixels[i+0x21] = uint8(c.G >> 0)
						} else {
							gray := ((graySum / 2) +
								(uint64(c.R) * grayR) +
								(uint64(c.G) * grayG) +
								(uint64(c.B) * grayB)) / graySum
							pixels[i+0x00] = uint8(gray >> 8)
							pixels[i+0x01] = uint8(gray >> 0)
						}
					}
				}
			}

		} else {
			return func(blockX int, blockY int) {
				for y := range 4 {
					for x := range 4 {
						i := (8 * y) + (2 * x)
						r, g, b, a := src.At(min(mX1, blockX+x), min(mY1, blockY+y)).RGBA()
						if (a != 0x0000) && (a != 0xFFFF) {
							r = (uint32(r) * 0xFFFF) / uint32(a)
							g = (uint32(g) * 0xFFFF) / uint32(a)
							b = (uint32(b) * 0xFFFF) / uint32(a)
						}
						if twoChannel {
							pixels[i+0x00] = uint8(r >> 8)
							pixels[i+0x01] = uint8(r >> 0)
							pixels[i+0x20] = uint8(g >> 8)
							pixels[i+0x21] = uint8(g >> 0)
						} else {
							gray := ((graySum / 2) +
								(uint64(r) * grayR) +
								(uint64(g) * grayG) +
								(uint64(b) * grayB)) / graySum
							pixels[i+0x00] = uint8(gray >> 8)
							pixels[i+0x01] = uint8(gray >> 0)
						}
					}
				}
			}
		}

	} else {
		if srcNRGBA, ok := src.(*image.NRGBA); ok {
			return func(blockX int, blockY int) {
				for y := range 4 {
					for x := range 4 {
						i := (16 * y) + (4 * x)
						c := srcNRGBA.NRGBAAt(min(mX1, blockX+x), min(mY1, blockY+y))
						pixels[i+0] = c.R
						pixels[i+1] = c.G
						pixels[i+2] = c.B
						pixels[i+3] = c.A
					}
				}
			}

		} else if srcNRGBA64, ok := src.(*image.NRGBA64); ok {
			return func(blockX int, blockY int) {
				for y := range 4 {
					for x := range 4 {
						i := (16 * y) + (4 * x)
						c := srcNRGBA64.NRGBA64At(min(mX1, blockX+x), min(mY1, blockY+y))
						pixels[i+0] = uint8(c.R >> 8)
						pixels[i+1] = uint8(c.G >> 8)
						pixels[i+2] = uint8(c.B >> 8)
						pixels[i+3] = uint8(c.A >> 8)
					}
				}
			}

		} else if srcRGBA64, ok := src.(image.RGBA64Image); ok {
			return func(blockX int, blockY int) {
				for y := range 4 {
					for x := range 4 {
						i := (16 * y) + (4 * x)
						c := srcRGBA64.RGBA64At(min(mX1, blockX+x), min(mY1, blockY+y))
						if (c.A != 0x0000) && (c.A != 0xFFFF) {
							c.R = uint16((uint32(c.R) * 0xFFFF) / uint32(c.A))
							c.G = uint16((uint32(c.G) * 0xFFFF) / uint32(c.A))
							c.B = uint16((uint32(c.B) * 0xFFFF) / uint32(c.A))
						}
						pixels[i+0] = uint8(c.R >> 8)
						pixels[i+1] = uint8(c.G >> 8)
						pixels[i+2] = uint8(c.B >> 8)
						pixels[i+3] = uint8(c.A >> 8)
					}
				}
			}

		} else {
			return func(blockX int, blockY int) {
				for y := range 4 {
					for x := range 4 {
						i := (16 * y) + (4 * x)
						r, g, b, a := src.At(min(mX1, blockX+x), min(mY1, blockY+y)).RGBA()
						if (a != 0x0000) && (a != 0xFFFF) {
							r = (uint32(r) * 0xFFFF) / uint32(a)
							g = (uint32(g) * 0xFFFF) / uint32(a)
							b = (uint32(b) * 0xFFFF) / uint32(a)
						}
						pixels[i+0] = uint8(r >> 8)
						pixels[i+1] = uint8(g >> 8)
						pixels[i+2] = uint8(b >> 8)
						pixels[i+3] = uint8(a >> 8)
					}
				}
			}
		}
	}
}
