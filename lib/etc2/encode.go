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
				signed := (f & formatBitDepth11Signed) != 0
				if (f & formatBitDepth11TwoChannel) != 0 {
					writeU64BE(e.buf[bufJ+0:], e.encode11(0x00, signed))
					writeU64BE(e.buf[bufJ+8:], e.encode11(0x20, signed))
					bufJ += 16
				} else {
					writeU64BE(e.buf[bufJ+0:], e.encode11(0x00, signed))
					bufJ += 8
				}

			} else if f == FormatETC2RGBA8 {
				writeU64BE(e.buf[bufJ+0:], e.encodeAlpha())
				writeU64BE(e.buf[bufJ+8:], e.encodeColor(f))
				bufJ += 16

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

func (e *encoder) hasTransparentPixelsWhenUsingOneBitAlpha() bool {
	for i := range 16 {
		if e.pixels[(4*i)+3] < 0x80 {
			return true
		}
	}
	return false
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
		codeA := e.encodeRGBWithAlpha(true)
		decodeColor(&e.work, codeA, true)
		lossA := e.calculateBlockLoss(formatIsOneBitAlpha)
		bestCode, bestLoss = codeA, lossA

		codeT := e.encodeT(true, false)
		decodeColor(&e.work, codeT, true)
		lossT := e.calculateBlockLoss(formatIsOneBitAlpha)
		if bestLoss > lossT {
			bestCode, bestLoss = codeT, lossT
		}

		codeH := e.encodeH(true, false)
		decodeColor(&e.work, codeH, true)
		lossH := e.calculateBlockLoss(formatIsOneBitAlpha)
		if bestLoss > lossH {
			bestCode, bestLoss = codeH, lossH
		}

		if e.hasTransparentPixelsWhenUsingOneBitAlpha() {
			return bestCode
		}

		codeB := e.encodeRGBWithAlpha(false)
		decodeColor(&e.work, codeB, true)
		lossB := e.calculateBlockLoss(formatIsOneBitAlpha)
		if bestLoss > lossB {
			bestCode, bestLoss = codeB, lossB
		}

	} else {
		codeA := e.encodeRGBSansAlpha(reduceAverage, f == FormatETC1S)
		decodeColor(&e.work, codeA, false)
		lossA := e.calculateBlockLoss(formatIsOneBitAlpha)
		bestCode, bestLoss = codeA, lossA

		if f == FormatETC1S {
			return bestCode
		}

		codeQ := e.encodeRGBSansAlpha(reduceQuantize, false)
		decodeColor(&e.work, codeQ, false)
		lossQ := e.calculateBlockLoss(formatIsOneBitAlpha)
		if bestLoss > lossQ {
			bestCode, bestLoss = codeQ, lossQ
		}

		if (f & formatBitsETC2) != formatBitsETC2 {
			return bestCode
		}
	}

	codeP := e.encodePlanar()
	decodeColor(&e.work, codeP, false)
	lossP := e.calculateBlockLoss(formatIsOneBitAlpha)
	if bestLoss > lossP {
		bestCode, bestLoss = codeP, lossP
	}

	const goHarderT, goHarderH = 1, 2
	goHarder := 0

	codeT := e.encodeT(false, false)
	decodeColor(&e.work, codeT, false)
	lossT := e.calculateBlockLoss(formatIsOneBitAlpha)
	if bestLoss > lossT {
		bestCode, bestLoss = codeT, lossT
		goHarder = goHarderT
	}

	codeH := e.encodeH(false, false)
	decodeColor(&e.work, codeH, false)
	lossH := e.calculateBlockLoss(formatIsOneBitAlpha)
	if bestLoss > lossH {
		bestCode, bestLoss = codeH, lossH
		goHarder = goHarderH
	}

	switch goHarder {
	case goHarderT:
		codeU := e.encodeT(false, true)
		decodeColor(&e.work, codeU, false)
		lossU := e.calculateBlockLoss(formatIsOneBitAlpha)
		if bestLoss > lossU {
			bestCode, bestLoss = codeU, lossU
		}

	case goHarderH:
		codeI := e.encodeH(false, true)
		decodeColor(&e.work, codeI, false)
		lossI := e.calculateBlockLoss(formatIsOneBitAlpha)
		if bestLoss > lossI {
			bestCode, bestLoss = codeI, lossI
		}
	}

	return bestCode
}

func (e *encoder) encodeRGBWithAlpha(isTransparent bool) uint64 {
	normErr := int32(0)
	flipErr := int32(0)
	normCode := uint64(0)
	flipCode := uint64(0)

	for flipBit := range 2 {
		avgColors := [2][3]float64{}
		for c := range 3 {
			totalWeights := [2]float64{}

			for x := range 4 {
				for y := range 4 {
					i := (4 * y) + x

					alpha := int32(e.pixels[(4*i)+3])
					weight := 1.0
					if alpha < 0x80 {
						weight = 0.0001
					}

					j := 0
					if flipBit == 0 {
						j = x >> 1
					} else {
						j = y >> 1
					}
					totalWeights[j] += weight
					avgColors[j][c] += weight * float64(e.pixels[(4*i)+c])
				}
			}

			avgColors[0][c] /= totalWeights[0]
			avgColors[1][c] /= totalWeights[1]
		}
		avgColorQuant0 := reduceQuantize(avgColors[0], true)
		avgColorQuant1 := reduceQuantize(avgColors[1], true)

		encColor0 := [3]int32{
			avgColorQuant0[0] >> 3,
			avgColorQuant0[1] >> 3,
			avgColorQuant0[2] >> 3,
		}
		encColor1 := [3]int32{
			avgColorQuant1[0] >> 3,
			avgColorQuant1[1] >> 3,
			avgColorQuant1[2] >> 3,
		}
		diff := [3]int32{
			max(-4, min(+3, encColor1[0]-encColor0[0])),
			max(-4, min(+3, encColor1[1]-encColor0[1])),
			max(-4, min(+3, encColor1[2]-encColor0[2])),
		}
		encColor1 = [3]int32{
			encColor0[0] + diff[0],
			encColor0[1] + diff[1],
			encColor0[2] + diff[2],
		}
		avgColorQuant0 = [3]int32{
			(encColor0[0] << 3) | (encColor0[0] >> 2),
			(encColor0[1] << 3) | (encColor0[1] >> 2),
			(encColor0[2] << 3) | (encColor0[2] >> 2),
		}
		avgColorQuant1 = [3]int32{
			(encColor1[0] << 3) | (encColor1[0] >> 2),
			(encColor1[1] << 3) | (encColor1[1] >> 2),
			(encColor1[2] << 3) | (encColor1[2] >> 2),
		}

		code := 0 |
			(uint64(encColor0[0]) << 59) |
			(uint64(diff[0]&7) << 56) |
			(uint64(encColor0[1]) << 51) |
			(uint64(diff[1]&7) << 48) |
			(uint64(encColor0[2]) << 43) |
			(uint64(diff[2]&7) << 40)
		if !isTransparent {
			code |= 1 << 33
		}

		bestError := [2]int32{maxInt32, maxInt32}
		bestTable := [2]int32{}
		bestIndexesLSB := [16]int32{}
		bestIndexesMSB := [16]int32{}

		for table := range 8 {
			tabError := [2]int32{}
			pixelIndexesLSB := [16]int32{}
			pixelIndexesMSB := [16]int32{}

			for x := range 4 {
				for y := range 4 {
					i := (4 * y) + x
					transparentPixel := e.pixels[(4*i)+3] < 0x80

					baseCol := [3]int32{}
					half := 0
					if ((flipBit == 0) && (x < 2)) || ((flipBit == 1) && (y < 2)) {
						baseCol = avgColorQuant0
					} else {
						half = 1
						baseCol = avgColorQuant1
					}

					bestJ, bestErrJ := 0, maxInt32
					for j := range 4 {
						if (j == 1) && isTransparent {
							continue
						}
						errJ := int32(0)
						for c := range 3 {
							mod := int32(modifiers[table][scramble[j]])
							col := max(0, min(255, baseCol[c]+mod))
							if (j == 2) && isTransparent {
								col = baseCol[c]
							}
							errCol := col - int32(e.pixels[(4*i)+c])
							errJ += errCol * errCol
						}
						if errJ < bestErrJ {
							bestJ, bestErrJ = j, errJ
						}
					}

					if transparentPixel {
						bestJ, bestErrJ = 1, 0
					}
					tabError[half] += bestErrJ

					pixelIndex := int32(scramble[bestJ])
					pixelIndexesLSB[(4*x)+y] = pixelIndex & 1
					pixelIndexesMSB[(4*x)+y] = pixelIndex >> 1
				}
			}

			for half := range 2 {
				if tabError[half] >= bestError[half] {
					continue
				}
				bestError[half] = tabError[half]
				bestTable[half] = int32(table)

				for i := range 16 {
					y := i % 4
					x := i / 4
					thisHalf := 0
					if flipBit == 0 {
						thisHalf = x >> 1
					} else {
						thisHalf = y >> 1
					}
					if half != thisHalf {
						continue
					}
					bestIndexesLSB[i] = pixelIndexesLSB[i]
					bestIndexesMSB[i] = pixelIndexesMSB[i]
				}
			}
		}

		code |= (uint64(bestTable[0]) << 37)
		code |= (uint64(bestTable[1]) << 34)
		for i := range 16 {
			code |= uint64(bestIndexesMSB[i]) << (i + 16)
			code |= uint64(bestIndexesLSB[i]) << (i + 0)
		}

		if flipBit == 0 {
			normErr = bestError[0] + bestError[1]
			normCode = code
		} else {
			flipErr = bestError[0] + bestError[1]
			flipCode = code | (1 << 32)
		}
	}

	if normErr <= flipErr {
		return normCode
	}
	return flipCode
}

func (e *encoder) encodeRGBSansAlpha(reduce reduceFunc, formatIsETC1S bool) uint64 {
	bestCode, bestLoss := uint64(0), maxInt32
	for flipBit := range 2 {
		rgbAvgs0 := e.calculateRGBAverages((2 * flipBit) + 0)
		rgbAvgs1 := e.calculateRGBAverages((2 * flipBit) + 1)

		base0, base1 := [3]int32{}, [3]int32{}
		if !formatIsETC1S {
			base0 = reduce(rgbAvgs0, true)
			base1 = reduce(rgbAvgs1, true)
		} else if flipBit == 0 {
			base0 = reduceETC1SProduce5BitColor(rgbAvgs0, rgbAvgs1)
			base1 = base0
		} else {
			break
		}

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

func reduceETC1SProduce5BitColor(rgbAvgs0 [3]float64, rgbAvgs1 [3]float64) [3]int32 {
	rgbAvgs0[0] = (rgbAvgs0[0] + rgbAvgs1[0]) / 2
	rgbAvgs0[1] = (rgbAvgs0[1] + rgbAvgs1[1]) / 2
	rgbAvgs0[2] = (rgbAvgs0[2] + rgbAvgs1[2]) / 2
	r := int32(((rgbAvgs0[0] * 31) / 255) + 0.5)
	g := int32(((rgbAvgs0[1] * 31) / 255) + 0.5)
	b := int32(((rgbAvgs0[2] * 31) / 255) + 0.5)
	return [3]int32{
		(r << 3) | (r >> 2),
		(g << 3) | (g >> 2),
		(b << 3) | (b >> 2),
	}
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

func (e *encoder) encodePlanar() uint64 {
	// Use Least Squares to find the vector x that minimizes |ax - b|**2, for
	// the Red, Green and Blue channels independently.
	//
	// a is a fixed 16×3 matrix, but it's more convenient to work with its
	// transpose: z = a' is a 3×16 matrix.
	//
	// z[1] captures the 4-pixel-wide horizontal gradient. z[2] captures the
	// 4-pixel-high vertical gradient. z[0] makes the three z values sum to 1.
	//
	// b is a 16×1 matrix holding e.pixels values.
	//
	// x is the 3×1 matrix [colorO[channel]; colorH[channel]; colorV[channel]].
	//
	// This is equivalent to solving x = inv(a' × a) × (a' × b) which breaks
	// down as computing d = (a' × b) and we can precompute c = inv(a' × a).
	//
	// In summary: d = (z × b); x = (c × d).

	zMatrix := [3][16]float64{{
		+1.00, +0.75, +0.50, +0.25,
		+0.75, +0.50, +0.25, +0.00,
		+0.50, +0.25, +0.00, -0.25,
		+0.25, +0.00, -0.25, -0.50,
	}, {
		+0.00, +0.25, +0.50, +0.75,
		+0.00, +0.25, +0.50, +0.75,
		+0.00, +0.25, +0.50, +0.75,
		+0.00, +0.25, +0.50, +0.75,
	}, {
		+0.00, +0.00, +0.00, +0.00,
		+0.25, +0.25, +0.25, +0.25,
		+0.50, +0.50, +0.50, +0.50,
		+0.75, +0.75, +0.75, +0.75,
	}}
	bMatrix := [16][1]float64{}
	cMatrix := [3][3]float64{
		{+0.2875, -0.0125, -0.0125},
		{-0.0125, +0.4875, -0.3125},
		{-0.0125, -0.3125, +0.4875},
	}
	dMatrix := [3][1]float64{}
	xMatrix := [3][1]float64{}

	colorO := [3]float64{}
	colorH := [3]float64{}
	colorV := [3]float64{}

	for channel := range 3 {
		for i := range 16 {
			bMatrix[i][0] = float64(e.pixels[(4*i)+channel])
		}

		// dMatrix = zMatrix × bMatrix.
		for a := range 3 {
			for b := range 1 {
				sum := float64(0)
				for i := range 16 {
					sum += zMatrix[a][i] * bMatrix[i][b]
				}
				dMatrix[a][b] = sum
			}
		}

		// xMatrix = cMatrix × dMatrix.
		for c := range 3 {
			for d := range 1 {
				sum := float64(0)
				for i := range 3 {
					sum += cMatrix[c][i] * dMatrix[i][d]
				}
				xMatrix[c][d] = sum
			}
		}

		colorO[channel] = max(0x00, min(0xFF, xMatrix[0][0]))
		colorH[channel] = max(0x00, min(0xFF, xMatrix[1][0]))
		colorV[channel] = max(0x00, min(0xFF, xMatrix[2][0]))
	}

	// Quantize to 676.
	colorOR6 := int32(((colorO[0] * 0x3F) / 0xFF) + 0.5)
	colorOG7 := int32(((colorO[1] * 0x7F) / 0xFF) + 0.5)
	colorOB6 := int32(((colorO[2] * 0x3F) / 0xFF) + 0.5)
	colorHR6 := int32(((colorH[0] * 0x3F) / 0xFF) + 0.5)
	colorHG7 := int32(((colorH[1] * 0x7F) / 0xFF) + 0.5)
	colorHB6 := int32(((colorH[2] * 0x3F) / 0xFF) + 0.5)
	colorVR6 := int32(((colorV[0] * 0x3F) / 0xFF) + 0.5)
	colorVG7 := int32(((colorV[1] * 0x7F) / 0xFF) + 0.5)
	colorVB6 := int32(((colorV[2] * 0x3F) / 0xFF) + 0.5)

	// Pack using Planar mode's idiosyncratic bit pattern.

	code := 0 |
		(uint64(colorOR6) << (63 - (6 + 0))) |
		(uint64(colorOG7&0x40) << (57 - (1 + 6))) |
		(uint64(colorOG7&0x3F) << (55 - (6 + 0))) |
		(uint64(colorOB6&0x20) << (49 - (1 + 5))) |
		(uint64(colorOB6&0x18) << (45 - (2 + 3))) |
		(uint64(colorOB6&0x07) << (42 - (3 + 0))) |
		(uint64(colorHR6&0x3E) << (39 - (5 + 1))) |
		(uint64(colorHR6&0x01) << (33 - (1 + 0))) |
		(uint64(colorHG7) << (32 - 7)) |
		(uint64(colorHB6) << (25 - 6)) |
		(uint64(colorVR6) << (19 - 6)) |
		(uint64(colorVG7) << (13 - 7)) |
		(uint64(colorVB6)) |
		(1 << 33) // Diff bit.

	// Ensure diff-red does not overflow.
	code |= (((code >> 62) & 1) ^ 1) << 63

	// Ensure diff-green does not overflow.
	code |= (((code >> 54) & 1) ^ 1) << 55

	// Ensure diff-blue overflows.
	{
		a := (code >> 44) & 1
		b := (code >> 43) & 1
		c := (code >> 41) & 1
		d := (code >> 40) & 1
		if 0 != ((a & c) | (^a & b & c & d) | (a & b & ^c & d)) {
			code |= 7 << 45
		} else {
			code |= 1 << 42
		}
	}

	return code
}

func (e *encoder) encodeT(formatIsOneBitAlpha bool, goHarder bool) uint64 {
	bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss := uint32(0), uint32(0), uint32(0), maxInt32
	bestCluster := (*[2][3]uint8)(nil)

	if goHarder {
		{
			cluster00 := clusterfy(&e.pixels, 0.0)
			convert8BitTo4Bit(&cluster00)
			bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss = e.calculateError59T(cluster00, formatIsOneBitAlpha)
			bestCluster = &cluster00
		}

		{
			cluster05 := clusterfy(&e.pixels, 0.5)
			convert8BitTo4Bit(&cluster05)
			swap05, which05, pixelIndexes05, blockLoss05 := e.calculateError59T(cluster05, formatIsOneBitAlpha)
			if bestBlockLoss > blockLoss05 {
				bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss = swap05, which05, pixelIndexes05, blockLoss05
				bestCluster = &cluster05
			}
		}

		{
			cluster10 := clusterfy(&e.pixels, 1.0)
			convert8BitTo4Bit(&cluster10)
			swap10, which10, pixelIndexes10, blockLoss10 := e.calculateError59T(cluster10, formatIsOneBitAlpha)
			if bestBlockLoss > blockLoss10 {
				bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss = swap10, which10, pixelIndexes10, blockLoss10
				bestCluster = &cluster10
			}
		}

	} else {
		cluster05 := clusterfy(&e.pixels, 0.5)
		convert8BitTo4Bit(&cluster05)
		bestSwap, bestWhich, bestPixelIndexes, _ = e.calculateError59T(cluster05, formatIsOneBitAlpha)
		bestCluster = &cluster05
	}

	if bestSwap > 0 {
		bestCluster[0][0], bestCluster[1][0] = bestCluster[1][0], bestCluster[0][0]
		bestCluster[0][1], bestCluster[1][1] = bestCluster[1][1], bestCluster[0][1]
		bestCluster[0][2], bestCluster[1][2] = bestCluster[1][2], bestCluster[0][2]
	}

	// Pack using T mode's idiosyncratic bit pattern.

	code := 0 |
		(uint64(bestCluster[0][0]&0x0C) << 57) |
		(uint64(bestCluster[0][0]&0x03) << 56) |
		(uint64(bestCluster[0][1]) << 52) |
		(uint64(bestCluster[0][2]) << 48) |
		(uint64(bestCluster[1][0]) << 44) |
		(uint64(bestCluster[1][1]) << 40) |
		(uint64(bestCluster[1][2]) << 36) |
		(uint64(bestWhich&0x06) << 33) |
		(uint64(bestWhich&0x01) << 32) |
		uint64(bestPixelIndexes)
	if !formatIsOneBitAlpha {
		code |= (1 << 33) // Diff bit.
	}

	// Ensure diff-red overflows.
	{
		a := (code >> 60) & 1
		b := (code >> 59) & 1
		c := (code >> 57) & 1
		d := (code >> 56) & 1
		if 0 != ((a & c) | (^a & b & c & d) | (a & b & ^c & d)) {
			code |= 7 << 61
		} else {
			code |= 1 << 58
		}
	}

	return code
}

func (e *encoder) calculateError59T(rgb444 [2][3]uint8, formatIsOneBitAlpha bool) (
	bestSwap uint32,
	bestWhich uint32,
	bestPixelIndexes uint32,
	bestBlockLoss int32) {

	bestBlockLoss = maxInt32
	for swap := range 2 {
		if swap > 0 {
			rgb444[0][0], rgb444[1][0] = rgb444[1][0], rgb444[0][0]
			rgb444[0][1], rgb444[1][1] = rgb444[1][1], rgb444[0][1]
			rgb444[0][2], rgb444[1][2] = rgb444[1][2], rgb444[0][2]
		}

		colors := [4][3]uint8{{
			rgb444[0][0] * 0x11,
			rgb444[0][1] * 0x11,
			rgb444[0][2] * 0x11,
		}, {}, {
			rgb444[1][0] * 0x11,
			rgb444[1][1] * 0x11,
			rgb444[1][2] * 0x11,
		}, {}}

		for which := range 8 {
			delta := uint32(thModifiers[which])
			colors[1][0] = clamp[(uint32(colors[2][0])+delta)&1023]
			colors[1][1] = clamp[(uint32(colors[2][1])+delta)&1023]
			colors[1][2] = clamp[(uint32(colors[2][2])+delta)&1023]
			colors[3][0] = clamp[(uint32(colors[2][0])-delta)&1023]
			colors[3][1] = clamp[(uint32(colors[2][1])-delta)&1023]
			colors[3][2] = clamp[(uint32(colors[2][2])-delta)&1023]

			pixelIndexes := uint32(0)
			blockLoss := int32(0)
			for i := range 16 {
				bestJ, bestOneLoss := 0, maxInt32
				if formatIsOneBitAlpha && (e.pixels[(4*i)+3] < 0x80) {
					bestJ, bestOneLoss = 2, 0
				} else {
					for j := range 4 {
						if formatIsOneBitAlpha && (j == 2) {
							continue
						}
						delta0 := int32(e.pixels[(4*i)+0]) - int32(colors[j][0])
						delta1 := int32(e.pixels[(4*i)+1]) - int32(colors[j][1])
						delta2 := int32(e.pixels[(4*i)+2]) - int32(colors[j][2])

						oneLoss := 0 +
							(weightValuesI32[0] * delta0 * delta0) +
							(weightValuesI32[1] * delta1 * delta1) +
							(weightValuesI32[2] * delta2 * delta2)
						if bestOneLoss > oneLoss {
							bestJ, bestOneLoss = j, oneLoss
						}
					}
				}

				shift := wholeBlockShifts[i]
				pixelIndexes |= uint32(bestJ&2) << (shift + 0x0F)
				pixelIndexes |= uint32(bestJ&1) << (shift + 0x00)
				blockLoss += bestOneLoss
			}

			if bestBlockLoss > blockLoss {
				bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss =
					uint32(swap), uint32(which), pixelIndexes, blockLoss
			}
		}
	}

	return bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss
}

func (e *encoder) encodeH(formatIsOneBitAlpha bool, goHarder bool) uint64 {
	bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss := uint32(0), uint32(0), uint32(0), maxInt32
	bestCluster := (*[2][3]uint8)(nil)

	if goHarder {
		{
			cluster00 := clusterfy(&e.pixels, 0.0)
			convert8BitTo4Bit(&cluster00)
			sort4BitColors(&cluster00)
			bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss = e.calculateError58H(cluster00, formatIsOneBitAlpha)
			bestCluster = &cluster00
		}

		{
			cluster05 := clusterfy(&e.pixels, 0.5)
			convert8BitTo4Bit(&cluster05)
			sort4BitColors(&cluster05)
			swap05, which05, pixelIndexes05, blockLoss05 := e.calculateError58H(cluster05, formatIsOneBitAlpha)
			if bestBlockLoss > blockLoss05 {
				bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss = swap05, which05, pixelIndexes05, blockLoss05
				bestCluster = &cluster05
			}
		}

		{
			cluster10 := clusterfy(&e.pixels, 1.0)
			convert8BitTo4Bit(&cluster10)
			sort4BitColors(&cluster10)
			swap10, which10, pixelIndexes10, blockLoss10 := e.calculateError58H(cluster10, formatIsOneBitAlpha)
			if bestBlockLoss > blockLoss10 {
				bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss = swap10, which10, pixelIndexes10, blockLoss10
				bestCluster = &cluster10
			}
		}

	} else {
		cluster05 := clusterfy(&e.pixels, 0.5)
		convert8BitTo4Bit(&cluster05)
		sort4BitColors(&cluster05)
		bestSwap, bestWhich, bestPixelIndexes, _ = e.calculateError58H(cluster05, formatIsOneBitAlpha)
		bestCluster = &cluster05
	}

	if bestSwap > 0 {
		bestCluster[0][0], bestCluster[1][0] = bestCluster[1][0], bestCluster[0][0]
		bestCluster[0][1], bestCluster[1][1] = bestCluster[1][1], bestCluster[0][1]
		bestCluster[0][2], bestCluster[1][2] = bestCluster[1][2], bestCluster[0][2]
	}

	bestPixelIndexes = sort4BitColorsWithPixelIndexes(bestCluster, bestWhich, bestPixelIndexes)

	// Pack using H mode's idiosyncratic bit pattern.

	code := 0 |
		(uint64(bestCluster[0][0]) << 59) |
		(uint64(bestCluster[0][1]&0x0E) << 55) |
		(uint64(bestCluster[0][1]&0x01) << 52) |
		(uint64(bestCluster[0][2]&0x08) << 48) |
		(uint64(bestCluster[0][2]&0x07) << 47) |
		(uint64(bestCluster[1][0]) << 43) |
		(uint64(bestCluster[1][1]) << 39) |
		(uint64(bestCluster[1][2]) << 35) |
		(uint64(bestWhich&0x04) << 32) |
		(uint64(bestWhich&0x02) << 31) |
		uint64(bestPixelIndexes)
	if !formatIsOneBitAlpha {
		code |= (1 << 33) // Diff bit.
	}

	// Ensure diff-red does not overflow.
	code |= (((code >> 62) & 1) ^ 1) << 63

	// Ensure diff-green overflows.
	{
		a := (code >> 52) & 1
		b := (code >> 51) & 1
		c := (code >> 49) & 1
		d := (code >> 48) & 1
		if 0 != ((a & c) | (^a & b & c & d) | (a & b & ^c & d)) {
			code |= 7 << 53
		} else {
			code |= 1 << 50
		}
	}

	return code
}

func (e *encoder) calculateError58H(rgb444 [2][3]uint8, formatIsOneBitAlpha bool) (
	bestSwap uint32,
	bestWhich uint32,
	bestPixelIndexes uint32,
	bestBlockLoss int32) {

	bestBlockLoss = maxInt32

	rgb444Packed := [2]uint32{
		(uint32(rgb444[0][0]) << 8) | (uint32(rgb444[0][1]) << 4) | (uint32(rgb444[0][2])),
		(uint32(rgb444[1][0]) << 8) | (uint32(rgb444[1][1]) << 4) | (uint32(rgb444[1][2])),
	}

	baseColors := [2][3]uint8{{
		rgb444[0][0] * 0x11,
		rgb444[0][1] * 0x11,
		rgb444[0][2] * 0x11,
	}, {
		rgb444[1][0] * 0x11,
		rgb444[1][1] * 0x11,
		rgb444[1][2] * 0x11,
	}}
	colors := [4][3]uint8{}

	for which := range 8 {
		alphaIndex := -1
		if formatIsOneBitAlpha {
			alphaIndex = 2
			if (rgb444Packed[0] >= rgb444Packed[1]) != ((which & 1) == 1) {
				alphaIndex = 0
			}
		}

		delta := uint32(thModifiers[which])
		colors[0][0] = clamp[(uint32(baseColors[0][0])+delta)&1023]
		colors[0][1] = clamp[(uint32(baseColors[0][1])+delta)&1023]
		colors[0][2] = clamp[(uint32(baseColors[0][2])+delta)&1023]
		colors[1][0] = clamp[(uint32(baseColors[0][0])-delta)&1023]
		colors[1][1] = clamp[(uint32(baseColors[0][1])-delta)&1023]
		colors[1][2] = clamp[(uint32(baseColors[0][2])-delta)&1023]
		colors[2][0] = clamp[(uint32(baseColors[1][0])+delta)&1023]
		colors[2][1] = clamp[(uint32(baseColors[1][1])+delta)&1023]
		colors[2][2] = clamp[(uint32(baseColors[1][2])+delta)&1023]
		colors[3][0] = clamp[(uint32(baseColors[1][0])-delta)&1023]
		colors[3][1] = clamp[(uint32(baseColors[1][1])-delta)&1023]
		colors[3][2] = clamp[(uint32(baseColors[1][2])-delta)&1023]

		pixelIndexes := uint32(0)
		blockLoss := int32(0)
		for i := range 16 {
			alpha := e.pixels[(4*i)+3]

			bestJ, bestOneLoss := 0, maxInt32
			for j := range 4 {
				oneLoss := int32(0)

				{
					if !formatIsOneBitAlpha {
						// No-op.
					} else if (j == alphaIndex) && (alpha >= 0x80) {
						oneLoss = 0
						goto haveOneLoss
					} else if (j == alphaIndex) || (alpha >= 0x80) {
						oneLoss = maxInt32
						goto haveOneLoss
					}

					delta0 := int32(e.pixels[(4*i)+0]) - int32(colors[j][0])
					delta1 := int32(e.pixels[(4*i)+1]) - int32(colors[j][1])
					delta2 := int32(e.pixels[(4*i)+2]) - int32(colors[j][2])

					oneLoss = 0 +
						(weightValuesI32[0] * delta0 * delta0) +
						(weightValuesI32[1] * delta1 * delta1) +
						(weightValuesI32[2] * delta2 * delta2)
				}

			haveOneLoss:
				if bestOneLoss > oneLoss {
					bestJ, bestOneLoss = j, oneLoss
				}
			}

			shift := wholeBlockShifts[i]
			pixelIndexes |= uint32(bestJ&2) << (shift + 0x0F)
			pixelIndexes |= uint32(bestJ&1) << (shift + 0x00)
			blockLoss += bestOneLoss
		}

		if bestBlockLoss > blockLoss {
			bestWhich, bestPixelIndexes, bestBlockLoss =
				uint32(which), pixelIndexes, blockLoss
		}
	}

	return bestSwap, bestWhich, bestPixelIndexes, bestBlockLoss
}

func clusterfy(pixels *[64]byte, intensity float64) (ret [2][3]uint8) {
	const (
		k1OverSqrt2 = 0.70710678118654752440084436210484903928483593768847403658833986899536623923
		k1OverSqrt3 = 0.57735026918962576450914878050195745564760175127012687601860232648397767230
		k1OverSqrt6 = 0.40824829046386301636621401245098189866099124677611168807211542787516006290
		k2OverSqrt6 = 0.81649658092772603273242802490196379732198249355222337614423085575032012581
	)

	changeBasisToQRS := intensity != 1

	originalColors := [48]float64{} // With possible change of basis to QRS, not RGB.
	mins := [3]float64{+512, +512, +512}
	maxs := [3]float64{-512, -512, -512}

	for i := range 16 {
		rgb0 := float64(pixels[(4*i)+0])
		rgb1 := float64(pixels[(4*i)+1])
		rgb2 := float64(pixels[(4*i)+2])

		qrs0 := rgb0
		qrs1 := rgb1
		qrs2 := rgb2
		if changeBasisToQRS {
			qrs0 = (+k1OverSqrt3 * rgb0) + (+k1OverSqrt3 * rgb1) + (+k1OverSqrt3 * rgb2)
			qrs1 = (+k1OverSqrt2 * rgb0) + (-k1OverSqrt2 * rgb1)
			qrs2 = (+k1OverSqrt6 * rgb0) + (+k1OverSqrt6 * rgb1) + (-k2OverSqrt6 * rgb2)
		}

		mins[0] = min(mins[0], qrs0)
		mins[1] = min(mins[1], qrs1)
		mins[2] = min(mins[2], qrs2)

		maxs[0] = max(maxs[0], qrs0)
		maxs[1] = max(maxs[1], qrs1)
		maxs[2] = max(maxs[2], qrs2)

		originalColors[(3*i)+0] = qrs0
		originalColors[(3*i)+1] = qrs1
		originalColors[(3*i)+2] = qrs2
	}

	maxsMinusMins := [3]float64{
		maxs[0] - mins[0],
		maxs[1] - mins[1],
		maxs[2] - mins[2],
	}

	// Run a k-means iterative-refinement algorithm (with k=2), from up to 10
	// randomly chosen starting places, to split the originalColors into two
	// clusters. The k-means algorithm is also known as Lloyd's algorithm.
	// Running k-means N times, with a slight perturbation on each of the N
	// bifurcations, producing (2 ** N) clusters, is also known as the
	// Linde–Buzo–Gray algorithm, but when N=1 here, it's simpler to describe
	// this as k-means instead of LBG.

	distortion := 512 * 512 * 3 * 16.0
	bestDistortion, bestColors := distortion, [2][3]float64{}

seedLoop:
	for seed := range 10 {
		currentColors := [2][3]float64{{
			((float64(randomInt31Values[(6*seed)+0]) / 0x7FFF_FFFF) * maxsMinusMins[0]) + mins[0],
			((float64(randomInt31Values[(6*seed)+1]) / 0x7FFF_FFFF) * maxsMinusMins[1]) + mins[1],
			((float64(randomInt31Values[(6*seed)+2]) / 0x7FFF_FFFF) * maxsMinusMins[2]) + mins[2],
		}, {
			((float64(randomInt31Values[(6*seed)+3]) / 0x7FFF_FFFF) * maxsMinusMins[0]) + mins[0],
			((float64(randomInt31Values[(6*seed)+4]) / 0x7FFF_FFFF) * maxsMinusMins[1]) + mins[1],
			((float64(randomInt31Values[(6*seed)+5]) / 0x7FFF_FFFF) * maxsMinusMins[2]) + mins[2],
		}}

		for _ = range 10 {
			oldDistortion := distortion
			distortion = 0

			numA, blockMask := 0, [16]uint8{}
			for i := range 16 {
				oc0 := originalColors[(3*i)+0]
				oc1 := originalColors[(3*i)+1]
				oc2 := originalColors[(3*i)+2]
				a0 := oc0 - currentColors[0][0]
				a1 := oc1 - currentColors[0][1]
				a2 := oc2 - currentColors[0][2]
				b0 := oc0 - currentColors[1][0]
				b1 := oc1 - currentColors[1][1]
				b2 := oc2 - currentColors[1][2]

				if !changeBasisToQRS {
					a0 = oc0 - round(currentColors[0][0])
					a1 = oc1 - round(currentColors[0][1])
					a2 = oc2 - round(currentColors[0][2])
					b0 = oc0 - round(currentColors[1][0])
					b1 = oc1 - round(currentColors[1][1])
					b2 = oc2 - round(currentColors[1][2])
				}

				errorA := (intensity * a0 * a0) + (a1 * a1) + (a2 * a2)
				errorB := (intensity * b0 * b0) + (b1 * b1) + (b2 * b2)
				if errorA < errorB {
					blockMask[i] = 0
					distortion += errorA
					numA++
				} else {
					blockMask[i] = 1
					distortion += errorB
				}
			}

			if bestDistortion > distortion {
				bestDistortion, bestColors = distortion, currentColors
			}

			if (numA == 0) || (numA == 16) {
				continue seedLoop
			} else if distortion == 0 {
				break seedLoop
			} else if distortion == oldDistortion {
				continue seedLoop
			}

			currentColors = [2][3]float64{}
			for i, bm := range blockMask {
				currentColors[bm][0] += originalColors[(3*i)+0]
				currentColors[bm][1] += originalColors[(3*i)+1]
				currentColors[bm][2] += originalColors[(3*i)+2]
			}
			currentColors[0][0] /= float64(numA)
			currentColors[0][1] /= float64(numA)
			currentColors[0][2] /= float64(numA)
			currentColors[1][0] /= float64(16 - numA)
			currentColors[1][1] /= float64(16 - numA)
			currentColors[1][2] /= float64(16 - numA)
		}
	}

	for i := range 2 {
		qrs0 := bestColors[i][0]
		qrs1 := bestColors[i][1]
		qrs2 := bestColors[i][2]

		rgb0 := qrs0
		rgb1 := qrs1
		rgb2 := qrs2
		if changeBasisToQRS {
			rgb0 = (+k1OverSqrt3 * qrs0) + (+k1OverSqrt2 * qrs1) + (+k1OverSqrt6 * qrs2)
			rgb1 = (+k1OverSqrt3 * qrs0) + (-k1OverSqrt2 * qrs1) + (+k1OverSqrt6 * qrs2)
			rgb2 = (+k1OverSqrt3 * qrs0) + (-k2OverSqrt6 * qrs2)
		}

		ret[i][0] = uint8(clamp[1023&int32(rgb0+0.5)])
		ret[i][1] = uint8(clamp[1023&int32(rgb1+0.5)])
		ret[i][2] = uint8(clamp[1023&int32(rgb2+0.5)])
	}
	return ret
}

func convert8BitTo4Bit(a *[2][3]uint8) {
	for i := range 2 {
		for j := range 3 {
			a[i][j] = uint8(((uint32(a[i][j]) + 8) * 15) / 255)
		}
	}
}

func sort4BitColors(a *[2][3]uint8) {
	c0 := (256 * uint32(a[0][0])) + (16 * uint32(a[0][1])) + uint32(a[0][2])
	c1 := (256 * uint32(a[1][0])) + (16 * uint32(a[1][1])) + uint32(a[1][2])

	if c0 < c1 {
		return
	} else if c0 > c1 {
		c0, c1 = c1, c0
	} else if c0 == 0 {
		c1 = c0 + 1
	} else {
		c0 = c1 - 1
	}

	a[0][0] = uint8(15 & (c0 >> 8))
	a[0][1] = uint8(15 & (c0 >> 4))
	a[0][2] = uint8(15 & (c0 >> 0))
	a[1][0] = uint8(15 & (c1 >> 8))
	a[1][1] = uint8(15 & (c1 >> 4))
	a[1][2] = uint8(15 & (c1 >> 0))
}

func sort4BitColorsWithPixelIndexes(a *[2][3]uint8, which uint32, pixelIndexes uint32) uint32 {
	c0 := (256 * uint32(a[0][0])) + (16 * uint32(a[0][1])) + uint32(a[0][2])
	c1 := (256 * uint32(a[1][0])) + (16 * uint32(a[1][1])) + uint32(a[1][2])

	if (c0 >= c1) == ((which & 1) == 1) {
		return pixelIndexes
	}

	a[0][0] = uint8(15 & (c1 >> 8))
	a[0][1] = uint8(15 & (c1 >> 4))
	a[0][2] = uint8(15 & (c1 >> 0))
	a[1][0] = uint8(15 & (c0 >> 8))
	a[1][1] = uint8(15 & (c0 >> 4))
	a[1][2] = uint8(15 & (c0 >> 0))
	return 0xFFFF_0000 ^ pixelIndexes
}

func (e *encoder) encode11(pixOffset int, signed bool) uint64 {
	h := encode11Helper{}
	bestBase, bestTable, bestMult := 0, 0, 0
	bestLoss := maxUint64
	for base := range 256 {
		for mult := range 16 {
			for table := range 16 {
				h.fill(base, mult, table, signed)
				loss := h.calculate11BlockLoss(&e.pixels, pixOffset, bestLoss)
				if bestLoss > loss {
					bestLoss = loss
					bestBase, bestTable, bestMult = base, table, mult
				}
			}
		}
	}
	h.fill(bestBase, bestMult, bestTable, signed)

	code := 0 |
		(uint64(bestBase) << (64 - 8)) |
		(uint64(bestMult) << (56 - 4)) |
		(uint64(bestTable) << (52 - 4))

	for i := range 16 {
		value := 0 +
			(uint32(e.pixels[pixOffset+(2*i)+0]) << 8) +
			(uint32(e.pixels[pixOffset+(2*i)+1]) << 0)
		bestJ, bestDelta2 := 0, maxUint64
		for j, helperValue := range h {
			delta := int64(value) - int64(helperValue)
			delta2 := uint64(delta * delta)
			if bestDelta2 > delta2 {
				bestJ, bestDelta2 = j, delta2
			}
		}

		x := uint32(i & 3)
		y := uint32(i >> 2)
		shift := (((x ^ 3) * 4) | (y ^ 3)) * 3
		code |= uint64(bestJ) << shift
	}

	return code
}

type encode11Helper [8]uint16

func (h *encode11Helper) calculate11BlockLoss(pixels *[64]byte, pixOffset int, bestLossSoFar uint64) (loss uint64) {
	for i := range 16 {
		value := 0 +
			(uint32(pixels[pixOffset+(2*i)+0]) << 8) +
			(uint32(pixels[pixOffset+(2*i)+1]) << 0)
		bestDelta2 := maxUint64
		for _, helperValue := range h {
			delta := int64(value) - int64(helperValue)
			delta2 := uint64(delta * delta)
			if bestDelta2 > delta2 {
				bestDelta2 = delta2
			}
		}
		loss += bestDelta2
		if loss >= bestLossSoFar {
			return loss
		}
	}
	return loss
}

func (h *encode11Helper) fill(rawBase int, rawMultiplier int, table int, signed bool) {
	multiplier := max(1, 8*int32(rawMultiplier))

	if signed {
		base := 8 * max(int32(int8(rawBase)), -127)
		for i := range h {
			delta := multiplier * int32(alphaModifiers[table][i])

			value11 := int32(max(-1023, min(1023, base+delta)))
			value16 := int32(0)
			if value11 >= 0 {
				value16 = (value11 << 5) | (value11 >> 5)
			} else {
				value11 = -value11
				value16 = (value11 << 5) | (value11 >> 5)
				value16 = -value16
			}
			value16 += 0x8000

			h[i] = uint16(value16)
		}

	} else {
		base := (8 * int32(rawBase)) + 4
		for i := range h {
			delta := multiplier * int32(alphaModifiers[table][i])

			value11 := uint32(max(0, min(2047, base+delta)))
			value16 := (value11 << 5) | (value11 >> 6)

			h[i] = uint16(value16)
		}
	}
}

func (e *encoder) encodeAlpha() uint64 {
	alphaSum := int32(0)
	for i := range 16 {
		a := int32(e.pixels[(4*i)+3])
		alphaSum += a
	}
	alpha := (alphaSum + 8) / 16

	maxDist := int32(0)
	for i := range 16 {
		a := int32(e.pixels[(4*i)+3])
		d := a - alpha
		if d < 0 {
			d = -d
		}
		maxDist = max(maxDist, d)
	}
	approxPos := min(255, ((maxDist*255)/160)-4)
	tableLo := max(0, approxPos-15)
	tableHi := max(0, min(255, approxPos+15))

	bestSum := maxInt32
	bestTable := int32(0)
	bestAlpha := int32(0)
	prevAlpha := alpha

	for table := tableLo; (table < tableHi) && (bestSum > 0); table++ {
		tableAlpha := prevAlpha
		tableBestSum := maxInt32

		for alphaScale := int32(16); alphaScale > 0; alphaScale /= 4 {
			alphaLo, alphaHi := int32(0), int32(0)
			if alphaScale == 16 {
				alphaLo = max(0, min(255, tableAlpha-(alphaScale*4)))
				alphaHi = max(0, min(255, tableAlpha+(alphaScale*4)))
			} else {
				alphaLo = max(0, min(255, tableAlpha-(alphaScale*2)))
				alphaHi = max(0, min(255, tableAlpha+(alphaScale*2)))
			}
			for alpha := alphaLo; alpha <= alphaHi; alpha += alphaScale {
				sum := int32(0)

			xLoop:
				for x := range 4 {
					for y := range 4 {
						bestDiff := maxInt32
						i := (4 * y) + x
						a := int32(e.pixels[(4*i)+3])

						if a > alpha {
							for index := 7; index >= 4; index-- {
								d1 := adjustAlpha(alpha, table, index) - a
								d2 := d1 * d1
								if bestDiff >= d2 {
									bestDiff = d2
								} else {
									break
								}
							}
						} else {
							for index := 0; index <= 3; index++ {
								d1 := adjustAlpha(alpha, table, index) - a
								d2 := d1 * d1
								if bestDiff > d2 {
									bestDiff = d2
								} else {
									break
								}
							}
						}

						sum += bestDiff
						if sum > bestSum {
							break xLoop
						}
					}
				}

				if tableBestSum > sum {
					tableBestSum = sum
					tableAlpha = alpha
				}
				if bestSum > sum {
					bestSum = sum
					bestTable = table
					bestAlpha = alpha
				}
			}
		}
	}

	code := 0 |
		(uint64(bestAlpha) << 56) |
		(uint64(bestTable) << 48)

	for i := range 16 {
		bestIndex, bestD2 := 0, maxInt32
		a := int32(e.pixels[(4*i)+3])
		for index := range 8 {
			d1 := adjustAlpha(bestAlpha, bestTable, index) - a
			d2 := d1 * d1
			if bestD2 > d2 {
				bestIndex, bestD2 = index, d2
			}
		}

		x := uint32(i & 3)
		y := uint32(i >> 2)
		shift := (((x ^ 3) * 4) | (y ^ 3)) * 3
		code |= uint64(bestIndex) << shift
	}

	return code
}

func adjustAlpha(rawAlpha int32, table int32, index int) int32 {
	multiplier := int32(table >> 4)
	delta := multiplier * int32(alphaModifiers[table&15][index])
	return int32(clamp[1023&(rawAlpha+delta)])
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

// Debian 12 Bookworm (2023) with gcc 12.
//
// RAND_MAX is 0x7FFF_FFFF.
//
//	#include <stdio.h>
//	#include <stdlib.h>
//	int main(int argc, char** argv) {
//		srand(10000);
//		for (int i = 0; i < 64; i++) {
//			int r = rand();
//			printf("0x%04X_%04X,", r >> 16, r & 0xFFFF);
//		}
//		return 0;
//	}
//
// Obligatory XKCD "Random Number" reference: https://xkcd.com/221/
var randomInt31Values = [64]int32{
	0x237D_6C33, 0x1E31_56EE, 0x3A5F_2C08, 0x4762_399F, 0x61A7_1336, 0x1C03_E9BD, 0x171F_1561, 0x5B60_8B64,
	0x3715_262C, 0x1AE2_A7DA, 0x1471_C6E4, 0x05BF_AFFC, 0x109A_D212, 0x22D1_1B88, 0x57AF_DA7F, 0x7AF7_A412,
	0x1D28_9D00, 0x3068_A7C7, 0x6384_F670, 0x2181_A9BA, 0x7144_A3E5, 0x2C14_116A, 0x1F59_A28C, 0x04BC_A8D6,
	0x75C9_072A, 0x35DA_8117, 0x451F_0311, 0x68C5_C3DA, 0x6FE7_F216, 0x2653_16D3, 0x2872_63E0, 0x1365_5E4A,
	0x4484_6DC2, 0x62D1_8FE8, 0x5AC7_97E9, 0x262B_80F8, 0x7ED5_79A5, 0x71E6_AD4A, 0x018C_0C5D, 0x35EA_9FD1,
	0x0CC9_5525, 0x15FD_D341, 0x3BAA_4FCD, 0x1D64_2737, 0x38CE_EECA, 0x135A_2A4D, 0x185B_CB49, 0x55F7_8BCA,
	0x43C2_D214, 0x7BE0_C1B9, 0x7779_3584, 0x3507_75F9, 0x27F4_D323, 0x16D2_D811, 0x39C4_1ED0, 0x1DBD_DA4E,
	0x4CAD_5928, 0x7EE3_21E1, 0x0683_9E28, 0x3C95_4B3F, 0x2536_38B5, 0x2EF6_0208, 0x4FFA_A989, 0x69BA_A677,
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

var wholeBlockShifts = [16]uint8{
	0x00, 0x04, 0x08, 0x0C,
	0x01, 0x05, 0x09, 0x0D,
	0x02, 0x06, 0x0A, 0x0E,
	0x03, 0x07, 0x0B, 0x0F,
}

var scramble = [4]uint8{3, 2, 0, 1}

func round(arg float64) float64 {
	if arg < 0 {
		return float64(int32(arg - 0.5))
	}
	return float64(int32(arg + 0.5))
}

const (
	maxFloat64 = float64(0x1p1023 * (1 + (1 - 0x1p-52))) // 1.79769313486231570814527423731704356798070e+308
	maxInt32   = int32(0x7FFF_FFFF)                      // 2147483647
	maxUint64  = uint64(0xFFFF_FFFF_FFFF_FFFF)           // 18446744073709551615
)

var (
	weightValuesF64 = [3]float64{299, 587, 114}
	weightValuesI32 = [3]int32{299, 587, 114}
)
