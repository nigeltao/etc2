// Copyright 2025 The Etc2 Authors.
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
// modified, or distributed except according to those terms.
//
// SPDX-License-Identifier: Apache-2.0

package pkm

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/nigeltao/etc2/internal/nie"
	"github.com/nigeltao/etc2/lib/etc2"
)

func TestDecode(tt *testing.T) {
	testCases := []string{
		"36.etc2-r11s",
		"36.etc2-r11u",
		"36.etc2-rg11s",
		"36.etc2-rg11u",
		"49.etc2-rgb",
		"49.etc2-rgba1",
		"49.etc2-rgba8",
		"49.etc2-srgb",
		"49.etc2-srgba1",
		"49.etc2-srgba8",
		"dice.80x60.etc2-rgb",
		"dice.80x60.etc2-rgba1",
		"dice.80x60.etc2-rgba8",
		"lincoln.24x32.etc1",
		"lincoln.24x32.etc2-r11u",
		"lincoln.24x32.etc2-rgb",
		"mona-lisa.21x32.etc1",
		"mona-lisa.21x32.etc2-rgb",
		"pearl-earring.54x64.etc1",
		"pearl-earring.54x64.etc2-rgb",
		"water-lillies.64x62.etc1",
		"water-lillies.64x62.etc2-rgb",
	}

	for _, tc := range testCases {
		srcBytes, err := os.ReadFile("../../res/1-encoded-pkm/" + tc + ".pkm")
		if err != nil {
			tt.Errorf("tc=%q: os.ReadFile(pkm): %v", tc, err)
			continue
		}

		srcImage, err := Decode(bytes.NewReader(srcBytes))
		if err != nil {
			tt.Errorf("tc=%q: Decode: %v", tc, err)
			continue
		}

		got, err := nie.EncodeBN8(srcImage)
		if err != nil {
			tt.Errorf("tc=%q: nie.EncodeBN8: %v", tc, err)
			continue
		}

		want, err := os.ReadFile("../../res/3-decoded-nie/" + tc + ".nie")
		if err != nil {
			tt.Errorf("tc=%q: os.ReadFile(nie): %v", tc, err)
			continue
		}

		if bytes.Equal(got, want) {
			continue
		} else if len(got) != len(want) {
			tt.Errorf("tc=%q: lengths: got %d, want %d", tc, len(got), len(want))
			continue
		}

		byteOffset := 0
		for byteOffset = range got {
			if got[byteOffset] != want[byteOffset] {
				break
			}
		}

		n := byteOffset &^ 7
		tt.Errorf("tc=%q: NIE output differs at byte offset 0x%04X (%d), got vs want:\n% 02X\n% 02X",
			tc, byteOffset, byteOffset, got[n:n+8], want[n:n+8])
	}
}

func TestEncode(tt *testing.T) {
	testCases := []struct {
		filename string
		format   etc2.Format
	}{
		{"36", etc2.FormatETC2R11Unsigned},
		{"36", etc2.FormatETC2RG11Unsigned},
		{"36", etc2.FormatETC2R11Signed},
		{"36", etc2.FormatETC2RG11Signed},
		{"49", etc2.FormatETC2RGB},
		{"49", etc2.FormatETC2RGBA8},
		{"49", etc2.FormatETC2SRGB},
		{"49", etc2.FormatETC2SRGBA8},
		{"dice.80x60", etc2.FormatETC2RGB},
		{"dice.80x60", etc2.FormatETC2RGBA8},
		{"lincoln.24x32", etc2.FormatETC1},
		{"lincoln.24x32", etc2.FormatETC2RGB},
		{"lincoln.24x32", etc2.FormatETC2R11Unsigned},
		{"mona-lisa.21x32", etc2.FormatETC1},
		{"mona-lisa.21x32", etc2.FormatETC2RGB},
		{"pearl-earring.54x64", etc2.FormatETC1},
		{"pearl-earring.54x64", etc2.FormatETC2RGB},
		{"water-lillies.64x62", etc2.FormatETC1},
		{"water-lillies.64x62", etc2.FormatETC2RGB},
	}

	cachedSrcImages := map[string]image.Image{}

	for _, tc := range testCases {
		tcString := tc.filename + "." + formatString(tc.format)

		srcImage := cachedSrcImages[tc.filename]
		if srcImage == nil {
			srcBytes, err := os.ReadFile("../../res/0-original-png/" + tc.filename + ".png")
			if err != nil {
				tt.Errorf("tc=%q: os.ReadFile(png): %v", tcString, err)
				continue
			}

			srcImage, err = png.Decode(bytes.NewReader(srcBytes))
			if err != nil {
				tt.Errorf("tc=%q: Decode: %v", tcString, err)
				continue
			}

			cachedSrcImages[tc.filename] = srcImage
		}

		buf := &bytes.Buffer{}
		options := &EncodeOptions{
			Format: tc.format,
		}
		if err := Encode(buf, srcImage, options); err != nil {
			tt.Errorf("tc=%q: Encode: %v", tcString, err)
			continue
		}
		got := buf.Bytes()

		want, err := os.ReadFile("../../res/1-encoded-pkm/" + tcString + ".pkm")
		if err != nil {
			tt.Errorf("tc=%q: os.ReadFile(pkm): %v", tcString, err)
			continue
		}

		if bytes.Equal(got, want) {
			continue
		} else if len(got) != len(want) {
			tt.Errorf("tc=%q: lengths: got %d, want %d", tcString, len(got), len(want))
			continue
		}

		byteOffset := 0
		for byteOffset = range got {
			if got[byteOffset] != want[byteOffset] {
				break
			}
		}

		n := byteOffset &^ 7
		tt.Errorf("tc=%q: PKM output differs at byte offset 0x%06X (%d), got vs want:\n% 02X\n% 02X",
			tcString, byteOffset, byteOffset, got[n:n+8], want[n:n+8])
	}
}

func formatString(f etc2.Format) string {
	switch f {
	case etc2.FormatETC1:
		return "etc1"
	case etc2.FormatETC1S:
		return "etc1s"

	case etc2.FormatETC2RGB:
		return "etc2-rgb"
	case etc2.FormatETC2RGBA8:
		return "etc2-rgba8"
	case etc2.FormatETC2RGBA1:
		return "etc2-rgba1"

	case etc2.FormatETC2SRGB:
		return "etc2-srgb"
	case etc2.FormatETC2SRGBA8:
		return "etc2-srgba8"
	case etc2.FormatETC2SRGBA1:
		return "etc2-srgba1"

	case etc2.FormatETC2R11Unsigned:
		return "etc2-r11u"
	case etc2.FormatETC2RG11Unsigned:
		return "etc2-rg11u"
	case etc2.FormatETC2R11Signed:
		return "etc2-r11s"
	case etc2.FormatETC2RG11Signed:
		return "etc2-rg11s"
	}

	return "invalid"
}
