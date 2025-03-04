// Copyright 2025 The Etc2 Authors.
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
// modified, or distributed except according to those terms.
//
// SPDX-License-Identifier: Apache-2.0

// ----------------

// Package nie implements the NIE (Naive) image file format.
//
// It is an incomplete implementation (and hence an internal package), only
// providing what's needed by the github.com/nigeltao/etc2 module.
//
// NIE is specified at
// https://github.com/google/wuffs/blob/main/doc/spec/nie-spec.md
package nie

import (
	"errors"
	"image"
	"image/color"
)

var (
	ErrBadArgument          = errors.New("nie: bad argument")
	ErrUnsupportedImageType = errors.New("nie: unsupported image type")
)

// EncodeBN8 encodes m as a NIE file in BGRA order, non-premultiplied alpha, 8
// bytes per pixel (16 bits per channel).
func EncodeBN8(m image.Image) (ret []byte, retErr error) {
	b := m.Bounds()
	ret = append(ret, 0x6E, 0xC3, 0xAF, 0x45, 0xFF, 'b', 'n', '8')
	ret = appendU32LE(ret, uint32(b.Dx()))
	ret = appendU32LE(ret, uint32(b.Dy()))

	switch m := m.(type) {
	case *image.Gray:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				at := m.GrayAt(x, y)
				ret = append(ret,
					uint8(at.Y), uint8(at.Y),
					uint8(at.Y), uint8(at.Y),
					uint8(at.Y), uint8(at.Y),
					0xFF, 0xFF,
				)
			}
		}
		return ret, nil

	case *image.Gray16:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				at := m.Gray16At(x, y)
				ret = append(ret,
					uint8(at.Y>>0), uint8(at.Y>>8),
					uint8(at.Y>>0), uint8(at.Y>>8),
					uint8(at.Y>>0), uint8(at.Y>>8),
					0xFF, 0xFF,
				)
			}
		}
		return ret, nil

	case *image.NRGBA:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				at := m.NRGBAAt(x, y)
				ret = append(ret,
					uint8(at.B), uint8(at.B),
					uint8(at.G), uint8(at.G),
					uint8(at.R), uint8(at.R),
					uint8(at.A), uint8(at.A),
				)
			}
		}
		return ret, nil

	case *image.NRGBA64:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				at := m.NRGBA64At(x, y)
				ret = append(ret,
					uint8(at.B>>0), uint8(at.B>>8),
					uint8(at.G>>0), uint8(at.G>>8),
					uint8(at.R>>0), uint8(at.R>>8),
					uint8(at.A>>0), uint8(at.A>>8),
				)
			}
		}
		return ret, nil

	case *image.RGBA:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				at := m.RGBAAt(x, y)
				if (at.A != 0x00) && (at.A != 0xFF) {
					return nil, ErrUnsupportedImageType
				}
				ret = append(ret,
					uint8(at.B), uint8(at.B),
					uint8(at.G), uint8(at.G),
					uint8(at.R), uint8(at.R),
					uint8(at.A), uint8(at.A),
				)
			}
		}
		return ret, nil

	case *image.RGBA64:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				at := m.RGBA64At(x, y)
				if (at.A != 0x0000) && (at.A != 0xFFFF) {
					return nil, ErrUnsupportedImageType
				}
				ret = append(ret,
					uint8(at.B>>0), uint8(at.B>>8),
					uint8(at.G>>0), uint8(at.G>>8),
					uint8(at.R>>0), uint8(at.R>>8),
					uint8(at.A>>0), uint8(at.A>>8),
				)
			}
		}
		return ret, nil

	case *image.Paletted:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				at := m.Palette[m.ColorIndexAt(x, y)]
				switch at := at.(type) {
				case color.NRGBA:
					ret = append(ret,
						uint8(at.B), uint8(at.B),
						uint8(at.G), uint8(at.G),
						uint8(at.R), uint8(at.R),
						uint8(at.A), uint8(at.A),
					)
				case color.RGBA:
					if (at.A != 0x00) && (at.A != 0xFF) {
						return nil, ErrUnsupportedImageType
					}
					ret = append(ret,
						uint8(at.B), uint8(at.B),
						uint8(at.G), uint8(at.G),
						uint8(at.R), uint8(at.R),
						uint8(at.A), uint8(at.A),
					)
				}
			}
		}
		return ret, nil
	}

	return nil, ErrUnsupportedImageType
}

func appendU32LE(b []byte, u uint32) []byte {
	return append(b,
		uint8(u>>0),
		uint8(u>>8),
		uint8(u>>16),
		uint8(u>>24),
	)
}
