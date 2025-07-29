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
	"io"
)

// EncodeOptions are optional arguments to Encode. The zero value is valid and
// means to use the default configuration.
//
// There are no fields for now, but there may be some in the future.
type EncodeOptions struct {
}

// Encode writes src to dst in the ETC format f.
//
// options may be nil, which means to use the default configuration.
func Encode(dst io.Writer, src image.Image, f Format, options *EncodeOptions) error {
	if (dst == nil) || (src == nil) || (f.ETCVersion() == 0) {
		return ErrBadArgument
	}

	// Strip the sRGB bit. This encoder treats RGB and sRGB equally.
	f &^= formatBitSRGBColorSpace

	b := src.Bounds()
	bW, bH := b.Dx(), b.Dy()
	if (bW > 65532) || (bH > 65532) {
		return ErrImageIsTooLarge
	}

	e, bufJ := &encoder{}, 0
	extract := f.makeExtract(&e.pixels, src)

	for blockY := 0; blockY < bH; blockY += 4 {
		for blockX := 0; blockX < bW; blockX += 4 {
			extract(blockX, blockY)

			if (f & formatBitDepth11) != 0 {
				panic("TODO")

			} else if f == FormatETC2RGBA8 {
				panic("TODO")

			} else {
				writeU64BE(e.buf[bufJ:], e.encodeColor(f))
				bufJ += 8
			}

			if bufJ >= encoderBufferSize {
				if _, err := dst.Write(e.buf[:]); err != nil {
					return err
				}
				bufJ = 0
			}
		}
	}

	if bufJ > 0 {
		if _, err := dst.Write(e.buf[:bufJ]); err != nil {
			return err
		}
	}
	return nil
}

const encoderBufferSize = 4096 - 64 - 64

type encoder struct {
	pixels [64]byte
	work   [64]byte
	buf    [encoderBufferSize]byte
}

func (e *encoder) calculateBlockLoss(formatIsOneBitAlpha bool) (loss int32) {
	for x := range 4 {
		for y := range 4 {
			i := (16 * y) + (4 * x)
			if formatIsOneBitAlpha && (e.pixels[i+3] < 0x80) {
				continue
			}
			d0 := int32(e.pixels[i+0]) - int32(e.work[i+0])
			d1 := int32(e.pixels[i+1]) - int32(e.work[i+1])
			d2 := int32(e.pixels[i+2]) - int32(e.work[i+2])
			loss += 0 +
				(weightValuesI32[0] * d0 * d0) +
				(weightValuesI32[1] * d1 * d1) +
				(weightValuesI32[2] * d2 * d2)
		}
	}
	return loss
}

func (e *encoder) encodeColor(f Format) uint64 {
	bestCode, bestLoss := uint64(0), maxInt32

	formatIsOneBitAlpha := f == FormatETC2RGBA1
	if formatIsOneBitAlpha {
		panic("TODO")

	} else {
		codeA := e.encodeRGBSansAlpha(reduceAverage)
		decodeColor(&e.work, codeA, false)
		lossA := e.calculateBlockLoss(formatIsOneBitAlpha)
		bestCode, bestLoss = codeA, lossA

		codeQ := e.encodeRGBSansAlpha(reduceQuantize)
		decodeColor(&e.work, codeQ, false)
		lossQ := e.calculateBlockLoss(formatIsOneBitAlpha)
		if bestLoss > lossQ {
			bestCode, bestLoss = codeQ, lossQ
		}

		if (f & formatBitsETC2) != formatBitsETC2 {
			return bestCode
		}
	}

	panic("TODO")
}

func (e *encoder) encodeRGBSansAlpha(reduce reduceFunc) uint64 {
	bestCode, bestLoss := uint64(0), maxInt32
	for flipBit := range 2 {
		rgbAvgs0 := e.calculateRGBAverages((2 * flipBit) + 0)
		rgbAvgs1 := e.calculateRGBAverages((2 * flipBit) + 1)

		base0 := reduce(rgbAvgs0, true)
		base1 := reduce(rgbAvgs1, true)

		diff0 := (base1[0] >> 3) - (base0[0] >> 3)
		diff1 := (base1[1] >> 3) - (base0[1] >> 3)
		diff2 := (base1[2] >> 3) - (base0[2] >> 3)

		if (-4 <= diff0) && (diff0 <= +3) &&
			(-4 <= diff1) && (diff1 <= +3) &&
			(-4 <= diff2) && (diff2 <= +3) {
			const diffBit = 1

			table0, indexes0, loss0 := e.encodeHalfBlock((2*flipBit)+0, &base0)
			table1, indexes1, loss1 := e.encodeHalfBlock((2*flipBit)+1, &base1)
			loss := loss0 + loss1

			if bestLoss > loss {
				bestLoss = loss
				bestCode = 0 |
					(uint64(base0[0]>>3) << (64 - 5)) |
					(uint64(diff0&7) << (59 - 3)) |
					(uint64(base0[1]>>3) << (56 - 5)) |
					(uint64(diff1&7) << (51 - 3)) |
					(uint64(base0[2]>>3) << (48 - 5)) |
					(uint64(diff2&7) << (43 - 3)) |
					(uint64(table0) << (40 - 3)) |
					(uint64(table1) << (37 - 3)) |
					(uint64(diffBit) << (34 - 1)) |
					(uint64(flipBit) << (33 - 1)) |
					uint64(indexes1) |
					uint64(indexes0)
			}

		} else {
			const diffBit = 0

			base0 = reduce(rgbAvgs0, false)
			base1 = reduce(rgbAvgs1, false)

			table0, indexes0, loss0 := e.encodeHalfBlock((2*flipBit)+0, &base0)
			table1, indexes1, loss1 := e.encodeHalfBlock((2*flipBit)+1, &base1)
			loss := loss0 + loss1

			if bestLoss > loss {
				bestLoss = loss
				bestCode = 0 |
					(uint64(base0[0]>>4) << (64 - 4)) |
					(uint64(base1[0]>>4) << (60 - 4)) |
					(uint64(base0[1]>>4) << (56 - 4)) |
					(uint64(base1[1]>>4) << (52 - 4)) |
					(uint64(base0[2]>>4) << (48 - 4)) |
					(uint64(base1[2]>>4) << (44 - 4)) |
					(uint64(table0) << (40 - 3)) |
					(uint64(table1) << (37 - 3)) |
					(uint64(diffBit) << (34 - 1)) |
					(uint64(flipBit) << (33 - 1)) |
					uint64(indexes1) |
					uint64(indexes0)
			}
		}
	}
	return bestCode
}

func (e *encoder) calculateRGBAverages(orientation int) [3]float64 {
	sums := [3]int32{}
	for i := range 8 {
		offset := perOrientationPixelsOffsets[orientation][i]
		sums[0] += int32(e.pixels[offset+0])
		sums[1] += int32(e.pixels[offset+1])
		sums[2] += int32(e.pixels[offset+2])
	}
	return [3]float64{
		float64(sums[0]) / 8,
		float64(sums[1]) / 8,
		float64(sums[2]) / 8,
	}
}

func (e *encoder) encodeHalfBlock(orientation int, base *[3]int32) (table uint32, indexes uint32, loss int32) {
	loss = maxInt32
	for t := range uint32(8) {
		indexes0, loss0 := e.encodeHalfBlock1(orientation, base, t)
		if loss > loss0 {
			table, indexes, loss = t, indexes0, loss0
		}
	}
	return table, indexes, loss
}

func (e *encoder) encodeHalfBlock1(orientation int, base *[3]int32, table uint32) (indexes uint32, loss int32) {
	for i := range 8 {
		offset := perOrientationPixelsOffsets[orientation][i]
		orig0 := int32(e.pixels[offset+0])
		orig1 := int32(e.pixels[offset+1])
		orig2 := int32(e.pixels[offset+2])

		bestOneLoss := maxInt32
		bestJ := uint8(0)
		for _, j := range scramble {
			delta0 := int32(clamp[1023&(uint32(base[0])+modifiers[table][j])]) - orig0
			delta1 := int32(clamp[1023&(uint32(base[1])+modifiers[table][j])]) - orig1
			delta2 := int32(clamp[1023&(uint32(base[2])+modifiers[table][j])]) - orig2
			oneLoss := 0 +
				(weightValuesI32[0] * delta0 * delta0) +
				(weightValuesI32[1] * delta1 * delta1) +
				(weightValuesI32[2] * delta2 * delta2)
			if bestOneLoss > oneLoss {
				bestJ, bestOneLoss = j, oneLoss
			}
		}

		shift := perOrientationShifts[orientation][i]
		indexes |= uint32(bestJ&2) << (shift + 0x0F)
		indexes |= uint32(bestJ&1) << (shift + 0x00)
		loss += bestOneLoss
	}
	return indexes, loss
}

type reduceFunc func(rgbAvgs [3]float64, produce5BitColor bool) [3]int32

func reduceAverage(rgbAvgs [3]float64, produce5BitColor bool) [3]int32 {
	if produce5BitColor {
		r := int32(((rgbAvgs[0] * 31) / 255) + 0.5)
		g := int32(((rgbAvgs[1] * 31) / 255) + 0.5)
		b := int32(((rgbAvgs[2] * 31) / 255) + 0.5)
		return [3]int32{
			(r << 3) | (r >> 2),
			(g << 3) | (g >> 2),
			(b << 3) | (b >> 2),
		}
	} else {
		r := int32(((rgbAvgs[0] * 15) / 255) + 0.5)
		g := int32(((rgbAvgs[1] * 15) / 255) + 0.5)
		b := int32(((rgbAvgs[2] * 15) / 255) + 0.5)
		return [3]int32{
			(r << 4) | r,
			(g << 4) | g,
			(b << 4) | b,
		}
	}
}

func reduceQuantize(rgbAvgs [3]float64, produce5BitColor bool) (ret [3]int32) {
	corners := [3][2]int32{}

	if produce5BitColor {
		rLo := int32((rgbAvgs[0] * 31) / 255)
		gLo := int32((rgbAvgs[1] * 31) / 255)
		bLo := int32((rgbAvgs[2] * 31) / 255)

		rHi := min(31, rLo+1)
		gHi := min(31, gLo+1)
		bHi := min(31, bLo+1)

		corners = [3][2]int32{
			{(rLo << 3) | (rLo >> 2), (rHi << 3) | (rHi >> 2)},
			{(gLo << 3) | (gLo >> 2), (gHi << 3) | (gHi >> 2)},
			{(bLo << 3) | (bLo >> 2), (bHi << 3) | (bHi >> 2)},
		}

	} else {
		rLo := int32((rgbAvgs[0] * 15) / 255)
		gLo := int32((rgbAvgs[1] * 15) / 255)
		bLo := int32((rgbAvgs[2] * 15) / 255)

		rHi := min(15, rLo+1)
		gHi := min(15, gLo+1)
		bHi := min(15, bLo+1)

		corners = [3][2]int32{
			{(rLo << 4) | rLo, (rHi << 4) | rHi},
			{(gLo << 4) | gLo, (gHi << 4) | gHi},
			{(bLo << 4) | bLo, (bHi << 4) | bHi},
		}
	}

	deltas := [3][2]float64{
		{float64(corners[0][0]) - rgbAvgs[0], float64(corners[0][1]) - rgbAvgs[0]},
		{float64(corners[1][0]) - rgbAvgs[1], float64(corners[1][1]) - rgbAvgs[1]},
		{float64(corners[2][0]) - rgbAvgs[2], float64(corners[2][1]) - rgbAvgs[2]},
	}

	bestLoss := maxFloat64
	for i := range 8 {
		ir := (i >> 0) & 1
		ig := (i >> 1) & 1
		ib := (i >> 2) & 1
		drg := deltas[0][ir] - deltas[1][ig]
		dgb := deltas[1][ig] - deltas[2][ib]
		dbr := deltas[2][ib] - deltas[0][ir]
		loss := 0 +
			(weightValuesF64[0] * weightValuesF64[1] * drg * drg) +
			(weightValuesF64[1] * weightValuesF64[2] * dgb * dgb) +
			(weightValuesF64[2] * weightValuesF64[0] * dbr * dbr)
		if bestLoss > loss {
			bestLoss = loss
			ret[0] = corners[0][ir]
			ret[1] = corners[1][ig]
			ret[2] = corners[2][ib]
		}
	}
	return ret
}

func writeU64BE(buf []byte, x uint64) {
	buf = buf[:8]
	buf[0] = uint8(x >> 56)
	buf[1] = uint8(x >> 48)
	buf[2] = uint8(x >> 40)
	buf[3] = uint8(x >> 32)
	buf[4] = uint8(x >> 24)
	buf[5] = uint8(x >> 16)
	buf[6] = uint8(x >> 8)
	buf[7] = uint8(x >> 0)
}

// numOrientations counts four orientations of a 2×4 or 4×2 half-block.
//
//   - 0: 2×4 tall and thin,  not-flipped, left side.
//   - 1: 2×4 tall and thin,  not-flipped, right side.
//   - 2: 4×2 short and wide, yes-flipped, top side.
//   - 3: 4×2 short and wide, yes-flipped, bottom side.
const numOrientations = 4

var perOrientationPixelsOffsets = [numOrientations][8]uint8{
	{0x00, 0x10, 0x20, 0x30, 0x04, 0x14, 0x24, 0x34},
	{0x08, 0x18, 0x28, 0x38, 0x0C, 0x1C, 0x2C, 0x3C},
	{0x00, 0x10, 0x04, 0x14, 0x08, 0x18, 0x0C, 0x1C},
	{0x20, 0x30, 0x24, 0x34, 0x28, 0x38, 0x2C, 0x3C},
}

var perOrientationShifts = [numOrientations][8]uint8{
	{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
	{0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
	{0x00, 0x01, 0x04, 0x05, 0x08, 0x09, 0x0C, 0x0D},
	{0x02, 0x03, 0x06, 0x07, 0x0A, 0x0B, 0x0E, 0x0F},
}

var scramble = [4]uint8{3, 2, 0, 1}

const (
	maxFloat64 = float64(0x1p1023 * (1 + (1 - 0x1p-52))) // 1.79769313486231570814527423731704356798070e+308
	maxInt32   = int32(0x7FFF_FFFF)                      // 2147483647
)

var (
	weightValuesF64 = [3]float64{299, 587, 114}
	weightValuesI32 = [3]int32{299, 587, 114}
)
