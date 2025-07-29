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
	maxPoint := src.Bounds().Max
	mX1 := maxPoint.X - 1
	mY1 := maxPoint.Y - 1

	if (f & formatBitDepth11) != 0 {
		panic("TODO")

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
