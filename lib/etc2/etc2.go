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
// A non-negative numerical int8 value matches that used in the PKM file
// format.
//
// Negative values have no counterpart in the KTX or PKM file formats. They can
// be passed to Encode (they represent a subset of a larger format; ETC1S is a
// subset of ETC1) but are not used by Decode.
//
// The "RGBA" in these constants' names match those used by other ETC
// documentation but note that it uses non-premultiplied alpha. The
// corresponding image and color types from Go's standard library are called
// NRGBA, not RGBA.
//
// Go's standard library also doesn't discriminate between what the ETC
// documentation calls RGB and sRGB.
type Format int8

const (
	FormatETC1S = Format(-1)

	FormatETC1 = Format(0x00)

	FormatETC2RGB   = Format(0x01)
	FormatETC2RGBA  = Format(0x03)
	FormatETC2RGBA1 = Format(0x04)

	FormatETC2UnsignedR11  = Format(0x05)
	FormatETC2UnsignedRG11 = Format(0x06)
	FormatETC2SignedR11    = Format(0x07)
	FormatETC2SignedRG11   = Format(0x08)

	FormatETC2SRGB   = Format(0x09)
	FormatETC2SRGBA  = Format(0x0A)
	FormatETC2SRGBA1 = Format(0x0B)
)

// AlphaModel returns the Format's transparency model.
func (f Format) AlphaModel() AlphaModel {
	switch f {
	case FormatETC1S,
		FormatETC1,
		FormatETC2RGB,
		FormatETC2SRGB,
		FormatETC2UnsignedR11,
		FormatETC2UnsignedRG11,
		FormatETC2SignedR11,
		FormatETC2SignedRG11:
		return AlphaModelOpaque

	case FormatETC2RGBA,
		FormatETC2SRGBA:
		return AlphaModel8Bit

	case FormatETC2RGBA1,
		FormatETC2SRGBA1:
		return AlphaModel1Bit
	}

	return 0
}

// BytesPerBlock returns the Format-dependent number of bytes used to encode
// each 4×4 pixel block.
func (f Format) BytesPerBlock() int {
	switch f {
	case FormatETC1S,
		FormatETC1,
		FormatETC2RGB,
		FormatETC2RGBA1,
		FormatETC2UnsignedR11,
		FormatETC2SignedR11,
		FormatETC2SRGB,
		FormatETC2SRGBA1:
		return 8

	case FormatETC2RGBA,
		FormatETC2UnsignedRG11,
		FormatETC2SignedRG11,
		FormatETC2SRGBA:
		return 16
	}

	return 0
}

// ETCVersion returns 0, 1 or 2 depending on whether the Format is invalid,
// from ETC1 or from ETC2.
func (f Format) ETCVersion() int {
	switch f {
	case FormatETC1S,
		FormatETC1:
		return 1

	case FormatETC2RGB,
		FormatETC2RGBA,
		FormatETC2RGBA1,
		FormatETC2UnsignedR11,
		FormatETC2UnsignedRG11,
		FormatETC2SignedR11,
		FormatETC2SignedRG11,
		FormatETC2SRGB,
		FormatETC2SRGBA,
		FormatETC2SRGBA1:
		return 2
	}

	return 0
}

// ColorModel returns the Go standard library's color model that best matches
// the Format.
func (f Format) ColorModel() color.Model {
	switch f {
	case FormatETC1S,
		FormatETC1,
		FormatETC2RGB,
		FormatETC2RGBA1,
		FormatETC2SRGB,
		FormatETC2SRGBA1:
		return color.RGBAModel

	case FormatETC2RGBA,
		FormatETC2SRGBA:
		return color.NRGBAModel

	case FormatETC2UnsignedR11,
		FormatETC2SignedR11:
		return color.Gray16Model

	case FormatETC2UnsignedRG11,
		FormatETC2SignedRG11:
		return color.RGBA64Model
	}

	return nil
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

	switch f {
	case FormatETC1S,
		FormatETC1,
		FormatETC2RGB,
		FormatETC2RGBA1,
		FormatETC2SRGB,
		FormatETC2SRGBA1:
		return image.NewRGBA(r), nil

	case FormatETC2RGBA,
		FormatETC2SRGBA:
		return image.NewNRGBA(r), nil

	case FormatETC2UnsignedR11,
		FormatETC2SignedR11:
		return image.NewGray16(r), nil

	case FormatETC2UnsignedRG11,
		FormatETC2SignedRG11:
		return image.NewRGBA64(r), nil
	}

	return nil, ErrBadArgument
}

// OpenGLInternalFormat returns the OpenGL internalFormat enum value for f, suitable
// for passing to the glCompressedTexImage2D function.
func (f Format) OpenGLInternalFormat() uint32 {
	switch f {
	case FormatETC1S,
		FormatETC1:
		return 0x8D64 // GL_ETC1_RGB8_OES
	case FormatETC2RGB:
		return 0x9274 // GL_COMPRESSED_RGB8_ETC2
	case FormatETC2RGBA:
		return 0x9278 // GL_COMPRESSED_RGBA8_ETC2_EAC
	case FormatETC2RGBA1:
		return 0x9276 // GL_COMPRESSED_RGB8_PUNCHTHROUGH_ALPHA1_ETC2
	case FormatETC2UnsignedR11:
		return 0x9270 // GL_COMPRESSED_R11_EAC
	case FormatETC2UnsignedRG11:
		return 0x9272 // GL_COMPRESSED_RG11_EAC
	case FormatETC2SignedR11:
		return 0x9271 // GL_COMPRESSED_SIGNED_R11_EAC
	case FormatETC2SignedRG11:
		return 0x9273 // GL_COMPRESSED_SIGNED_RG11_EAC
	case FormatETC2SRGB:
		return 0x9275 // GL_COMPRESSED_SRGB8_ETC2
	case FormatETC2SRGBA:
		return 0x9279 // GL_COMPRESSED_SRGB8_ALPHA8_ETC2_EAC
	case FormatETC2SRGBA1:
		return 0x9277 // GL_COMPRESSED_SRGB8_PUNCHTHROUGH_ALPHA1_ETC2
	}

	return 0
}
