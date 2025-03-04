// Copyright 2025 The Etc2 Authors.
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
// modified, or distributed except according to those terms.
//
// SPDX-License-Identifier: Apache-2.0

//go:build ignore

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func main() {
	if err := main1(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func main1() error {
	f, err := opentype.Parse(goitalic.TTF)
	if err != nil {
		return fmt.Errorf("opentype.Parse: %v", err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    200,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return fmt.Errorf("opentype.NewFace: %v", err)
	}

	if err := do(face, "36.png"); err != nil {
		return err
	}
	if err := do(face, "49.png"); err != nil {
		return err
	}

	return nil
}

func do(face font.Face, filename string) error {
	digit0 := image.NewRGBA(image.Rect(0, 0, 256, 256))
	{
		d := font.Drawer{
			Dst:  digit0,
			Src:  image.White,
			Face: face,
			Dot:  fixed.P(4, 224),
		}
		d.DrawString(filename[0:1])

		for i := range digit0.Pix {
			digit0.Pix[i] ^= 0xFF
		}
	}

	digit1 := image.NewRGBA(image.Rect(0, 0, 256, 256))
	{
		d := font.Drawer{
			Dst:  digit1,
			Src:  image.White,
			Face: face,
			Dot:  fixed.P(4+112, 224-48),
		}
		d.DrawString(filename[1:2])
	}

	circ := image.NewRGBA(image.Rect(0, 0, 256, 256))
	{
		const cx, cy = 30, 50
		for y := range 256 {
			dy := y - cy
			for x := range 256 {
				dx := x - cx
				distance := int64(math.Sqrt(float64((dx * dx) + (dy * dy))))
				v := 0xFF - uint8(max(0x00, min(0xFF, distance)))
				circ.SetRGBA(x, y, color.RGBA{v, v / 3, 0, v})
			}
		}
	}

	grad := image.NewRGBA(image.Rect(0, 0, 256, 256))
	{
		for y := range 256 {
			for x := range 256 {
				grad.SetRGBA(x, y, color.RGBA{0x00, uint8(x), uint8(y), 0xFF})
			}
		}
	}

	large := image.NewRGBA(image.Rect(0, 0, 256, 256))
	draw.DrawMask(large, large.Bounds(), circ, circ.Bounds().Min, digit0, digit0.Bounds().Min, draw.Over)
	draw.DrawMask(large, large.Bounds(), grad, grad.Bounds().Min, digit1, digit1.Bounds().Min, draw.Over)

	small := image.Image(nil)
	if filename[0] == '3' {
		m := image.NewNRGBA64(image.Rect(0, 0, 16, 16))
		for y := range 16 {
			for x := range 16 {
				sum := [4]uint32{}
				for v := range 16 {
					for u := range 16 {
						at := large.RGBAAt((16*x)+u, (16*y)+v)
						sum[0] += uint32(at.R) * 0x101
						sum[1] += uint32(at.G) * 0x101
						sum[2] += uint32(at.B) * 0x101
						sum[3] += uint32(at.A) * 0x101
					}
				}
				m.SetNRGBA64(x, y, color.NRGBA64{
					uint16((sum[0] + 128) / 256),
					uint16((sum[1] + 128) / 256),
					uint16((sum[2] + 128) / 256),
					0xFFFF,
				})
			}
		}
		small = m

	} else {
		m := image.NewRGBA(image.Rect(0, 0, 16, 16))
		for y := range 16 {
			for x := range 16 {
				sum := [4]uint32{}
				for v := range 16 {
					for u := range 16 {
						at := large.RGBAAt((16*x)+u, (16*y)+v)
						sum[0] += uint32(at.R)
						sum[1] += uint32(at.G)
						sum[2] += uint32(at.B)
						sum[3] += uint32(at.A)
					}
				}
				m.SetRGBA(x, y, color.RGBA{
					uint8((sum[0] + 128) / 256),
					uint8((sum[1] + 128) / 256),
					uint8((sum[2] + 128) / 256),
					uint8((sum[3] + 128) / 256),
				})
			}
		}
		small = m
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("os.Create: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, small); err != nil {
		return fmt.Errorf("png.Encode: %v", err)
	}
	return nil
}
