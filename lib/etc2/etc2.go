// Copyright 2025 The Etc2 Authors.
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
// modified, or distributed except according to those terms.
//
// SPDX-License-Identifier: Apache-2.0

// ----------------

// Package etc2 implements the ETC (Ericsson Texture Compression) image file
// format, supporting versions 1 and 2.
//
// ETC is often wrapped in .pkm container files (iPACKMAN was an earlier name
// for ETC), which prepends a small (16 byte) header stating width, height and
// format. ETC can also appear in .ktx (Khronos Texture) files.
//
// ETC is specified at
// https://registry.khronos.org/DataFormat/specs/1.3/dataformat.1.3.html#ETC2
package etc2

import (
	"errors"
	"image"
	"image/color"
)

var (
	ErrBadArgument  = errors.New("etc2: bad argument")
	ErrBadImageType = errors.New("etc2: bad image type")
)

// SubsettableImage is an image.Image that also has a SubImage method, like all
// of the Go standard library's image types.
type SubsettableImage interface {
	image.Image
	SubImage(r image.Rectangle) image.Image
}

// AlphaModel is a Format's transparency model.
type AlphaModel uint8

const (
	AlphaModelOpaque = AlphaModel(0)
	AlphaModel1Bit   = AlphaModel(1)
	AlphaModel8Bit   = AlphaModel(2)
)

// Format gives the "color type" specialization of the ETC family.
//
// The "RGBA" in these constants' names match those used by other ETC
// documentation but note that it uses non-premultiplied alpha. The
// corresponding image and color types from Go's standard library are called
// NRGBA, not RGBA.
//
// Go's standard library also doesn't discriminate between what the ETC
// documentation calls RGB and sRGB.
type Format uint8

const (
	FormatInvalid = Format(0x00)

	FormatETC1S = Format(0x40)
	FormatETC1  = Format(0x80)

	FormatETC2RGB   = Format(0xC0)
	FormatETC2RGBA1 = Format(0xC1)
	FormatETC2RGBA8 = Format(0xC2)

	FormatETC2SRGB   = Format(0xC4)
	FormatETC2SRGBA1 = Format(0xC5)
	FormatETC2SRGBA8 = Format(0xC6)

	FormatETC2R11Unsigned  = Format(0xC8)
	FormatETC2R11Signed    = Format(0xD8)
	FormatETC2RG11Unsigned = Format(0xE8)
	FormatETC2RG11Signed   = Format(0xF8)
)

const (
	formatBit1BitAlpha         = Format(0x01)
	formatBit8BitAlpha         = Format(0x02)
	formatBitSRGBColorSpace    = Format(0x04)
	formatBitDepth11           = Format(0x08)
	formatBitDepth11Signed     = Format(0x10)
	formatBitDepth11TwoChannel = Format(0x20)

	formatBitsETC1S = Format(0x40)
	formatBitsETC1  = Format(0x80)
	formatBitsETC2  = Format(0xC0)
)

// AlphaModel returns the Format's transparency model.
func (f Format) AlphaModel() AlphaModel {
	switch f & (formatBit1BitAlpha | formatBit8BitAlpha) {
	case formatBit1BitAlpha:
		return AlphaModel1Bit
	case formatBit8BitAlpha:
		return AlphaModel8Bit
	}
	return AlphaModelOpaque
}

// BytesPerBlock returns the Format-dependent number of bytes used to encode
// each 4×4 pixel block.
func (f Format) BytesPerBlock() int {
	if f == FormatInvalid {
		return 0
	} else if 0 != (f & (formatBit8BitAlpha | formatBitDepth11TwoChannel)) {
		return 16
	}
	return 8
}

// ETCVersion returns 0, 1 or 2 depending on whether the Format is invalid,
// from ETC1 or from ETC2.
func (f Format) ETCVersion() int {
	if f < formatBitsETC1S {
		return 0
	} else if f < formatBitsETC2 {
		return 1
	}
	return 2
}

// ColorModel returns the Go standard library's color model that best matches
// the Format.
func (f Format) ColorModel() color.Model {
	if f == FormatInvalid {
		return nil
	} else if 0 != (f & formatBit8BitAlpha) {
		return color.NRGBAModel
	} else if 0 == (f & formatBitDepth11) {
		return color.RGBAModel
	} else if 0 != (f & formatBitDepth11TwoChannel) {
		return color.RGBA64Model
	}
	return color.Gray16Model
}

// NewImage returns an image.Image, whose concrete type is one of the standard
// library's image types, that's suitable for the Format.
//
// The requested width and height will be rounded up to a multiple of 4.
//
// It returns an error if the width or height is negative or above 65536.
func (f Format) NewImage(width int, height int) (SubsettableImage, error) {
	if (width < 0) || (width >= 65536) ||
		(height < 0) || (height >= 65536) {
		return nil, ErrBadArgument
	}
	r := image.Rect(0, 0, (width+3)&^3, (height+3)&^3)

	if f == FormatInvalid {
		return nil, ErrBadArgument
	} else if 0 != (f & formatBit8BitAlpha) {
		return image.NewNRGBA(r), nil
	} else if 0 == (f & formatBitDepth11) {
		return image.NewRGBA(r), nil
	} else if 0 != (f & formatBitDepth11TwoChannel) {
		return image.NewRGBA64(r), nil
	}
	return image.NewGray16(r), nil
}

// OpenGLInternalFormat returns the OpenGL internalFormat enum value for f, suitable
// for passing to the glCompressedTexImage2D function.
func (f Format) OpenGLInternalFormat() uint32 {
	switch f {
	case FormatETC1S, FormatETC1:
		return 0x8D64 // GL_ETC1_RGB8_OES

	case FormatETC2RGB:
		return 0x9274 // GL_COMPRESSED_RGB8_ETC2
	case FormatETC2RGBA8:
		return 0x9278 // GL_COMPRESSED_RGBA8_ETC2_EAC
	case FormatETC2RGBA1:
		return 0x9276 // GL_COMPRESSED_RGB8_PUNCHTHROUGH_ALPHA1_ETC2

	case FormatETC2SRGB:
		return 0x9275 // GL_COMPRESSED_SRGB8_ETC2
	case FormatETC2SRGBA8:
		return 0x9279 // GL_COMPRESSED_SRGB8_ALPHA8_ETC2_EAC
	case FormatETC2SRGBA1:
		return 0x9277 // GL_COMPRESSED_SRGB8_PUNCHTHROUGH_ALPHA1_ETC2

	case FormatETC2R11Unsigned:
		return 0x9270 // GL_COMPRESSED_R11_EAC
	case FormatETC2R11Signed:
		return 0x9271 // GL_COMPRESSED_SIGNED_R11_EAC
	case FormatETC2RG11Unsigned:
		return 0x9272 // GL_COMPRESSED_RG11_EAC
	case FormatETC2RG11Signed:
		return 0x9273 // GL_COMPRESSED_SIGNED_RG11_EAC
	}

	return 0
}

// PKMFormat returns the PKM file format's enum value for f.
func (f Format) PKMFormat() uint8 {
	switch f {
	case FormatETC1S, FormatETC1:
		return 0x00

	case FormatETC2RGB:
		return 0x01
	case FormatETC2RGBA1:
		return 0x04
	case FormatETC2RGBA8:
		return 0x03

	case FormatETC2SRGB:
		return 0x09
	case FormatETC2SRGBA1:
		return 0x0B
	case FormatETC2SRGBA8:
		return 0x0A

	case FormatETC2R11Unsigned:
		return 0x05
	case FormatETC2R11Signed:
		return 0x07
	case FormatETC2RG11Unsigned:
		return 0x06
	case FormatETC2RG11Signed:
		return 0x08
	}

	return 0
}
