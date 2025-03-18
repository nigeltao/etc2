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
	"os"
	"testing"

	"github.com/nigeltao/etc2/internal/nie"
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
