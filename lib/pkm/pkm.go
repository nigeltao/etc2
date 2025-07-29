// Copyright 2025 The Etc2 Authors.
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
// modified, or distributed except according to those terms.
//
// SPDX-License-Identifier: Apache-2.0

// ----------------

// Package pkm implements the PKM container format for ETC textures.
package pkm

import (
	"errors"
	"image"
	"io"

	"github.com/nigeltao/etc2/lib/etc2"
)

// Magic is the byte string prefix of every PKM image file.
const Magic = "PKM "

func init() {
	image.RegisterFormat("pkm", Magic, Decode, DecodeConfig)
}

var (
	ErrBadArgument     = errors.New("pkm: bad argument")
	ErrNotAPKMFile     = errors.New("pkm: not a PKM file")
	ErrImageIsTooLarge = errors.New("pkm: image is too large")
)

var pkmToETC2Formats = [12]etc2.Format{
	0x00: etc2.FormatETC1,
	0x01: etc2.FormatETC2RGB,
	0x02: etc2.FormatInvalid,
	0x03: etc2.FormatETC2RGBA8,
	0x04: etc2.FormatETC2RGBA1,
	0x05: etc2.FormatETC2R11Unsigned,
	0x06: etc2.FormatETC2RG11Unsigned,
	0x07: etc2.FormatETC2R11Signed,
	0x08: etc2.FormatETC2RG11Signed,
	0x09: etc2.FormatETC2SRGB,
	0x0A: etc2.FormatETC2SRGBA8,
	0x0B: etc2.FormatETC2SRGBA1,
}

func decodeConfig(r io.Reader) (retFormat etc2.Format, retConfig image.Config, retErr error) {
	buf := [16]byte{}
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, image.Config{}, err
	} else if (buf[0] != Magic[0]) ||
		(buf[1] != Magic[1]) ||
		(buf[2] != Magic[2]) ||
		(buf[3] != Magic[3]) ||
		(buf[5] != 0x30) ||
		(buf[6] != 0x00) {
		return 0, image.Config{}, ErrNotAPKMFile
	}

	etcVersion := 0
	switch buf[4] {
	case 0x31, 0x32:
		etcVersion = int(buf[4]) & 0x03
	default:
		return 0, image.Config{}, ErrNotAPKMFile
	}

	if f := int(buf[7]); f < len(pkmToETC2Formats) {
		retFormat = pkmToETC2Formats[f]
	}
	if retFormat.ETCVersion() != etcVersion {
		return 0, image.Config{}, ErrNotAPKMFile
	}

	roundedUpWidth := (uint32(buf[8]) << 8) | uint32(buf[9])
	roundedUpHeight := (uint32(buf[10]) << 8) | uint32(buf[11])
	width := (uint32(buf[12]) << 8) | uint32(buf[13])
	height := (uint32(buf[14]) << 8) | uint32(buf[15])

	if (((width + 3) &^ 3) != roundedUpWidth) ||
		(((height + 3) &^ 3) != roundedUpHeight) {
		return 0, image.Config{}, ErrNotAPKMFile
	}

	return retFormat, image.Config{
		ColorModel: retFormat.ColorModel(),
		Width:      int(width),
		Height:     int(height),
	}, nil
}

// DecodeConfig reads a PKM image configuration from r.
func DecodeConfig(r io.Reader) (image.Config, error) {
	_, config, err := decodeConfig(r)
	return config, err
}

// Decode reads a PKM image from r.
func Decode(r io.Reader) (image.Image, error) {
	format, config, err := decodeConfig(r)
	if err != nil {
		return nil, err
	}
	m, err := format.NewImage(config.Width, config.Height)
	if err != nil {
		return nil, err
	}
	b := m.Bounds()
	if err = format.Decode(m, r, b.Dx()/4, b.Dy()/4); err != nil {
		return nil, err
	}
	return m.SubImage(image.Rect(0, 0, config.Width, config.Height)), err
}

// EncodeOptions are optional arguments to Encode. The zero value is valid and
// means to use the default configuration.
type EncodeOptions struct {
	// If zero, the default is to use etc2.FormatETC2RGB.
	Format etc2.Format
}

// Encode writes src to w in the PKM format.
//
// options may be nil, which means to use the default configuration.
func Encode(w io.Writer, src image.Image, options *EncodeOptions) error {
	b := src.Bounds()
	bW, bH := b.Dx(), b.Dy()
	if (bW > 65532) || (bH > 65532) {
		return ErrImageIsTooLarge
	}

	f := etc2.FormatETC2RGB
	if (options != nil) && (options.Format != 0) {
		f = options.Format
	}
	version := f.ETCVersion()
	if version == 0 {
		return ErrBadArgument
	}

	buf := [16]byte{}
	copy(buf[:4], Magic)
	buf[0x04] = 0x30 | uint8(version)
	buf[0x05] = 0x30
	buf[0x06] = 0x00
	buf[0x07] = byte(f.PKMFormat())

	roundedUpW := (bW + 3) &^ 3
	roundedUpH := (bH + 3) &^ 3
	buf[0x08] = uint8(roundedUpW >> 8)
	buf[0x09] = uint8(roundedUpW >> 0)
	buf[0x0A] = uint8(roundedUpH >> 8)
	buf[0x0B] = uint8(roundedUpH >> 0)
	buf[0x0C] = uint8(bW >> 8)
	buf[0x0D] = uint8(bW >> 0)
	buf[0x0E] = uint8(bH >> 8)
	buf[0x0F] = uint8(bH >> 0)
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}

	return etc2.Encode(w, src, f, nil)
}
