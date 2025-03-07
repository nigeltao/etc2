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
	ErrNotAPKMFile = errors.New("pkm: not a PKM file")
)

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

	retFormat = etc2.Format(buf[7])
	if (retFormat < 0) || (retFormat.ETCVersion() != etcVersion) {
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
