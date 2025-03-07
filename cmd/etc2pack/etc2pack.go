// Copyright 2025 The Etc2 Authors.
//
// Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
// https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
// modified, or distributed except according to those terms.
//
// SPDX-License-Identifier: Apache-2.0

// ----------------

// etc2pack decodes and encodes the ETC2 (Ericsson Texture Compression 2) lossy
// image file format.
package main

import (
	"errors"
	"flag"
	"image/png"
	"os"

	"github.com/nigeltao/etc2/internal/nie"
	"github.com/nigeltao/etc2/lib/pkm"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

var (
	decodeFlag = flag.Bool("decode", false, "whether to decode the input")
	encodeFlag = flag.Bool("encode", false, "whether to encode the input")
	outputFlag = flag.String("output", "", "output format")
)

const usageStr = `etc2pack decodes and encodes the ETC2 lossy image file format.

Usage: choose one of

    etc2pack -decode [path]
    etc2pack -encode [path]

The path to the input image file is optional. If omitted, stdin is read.

When decoding you can also pass one of these flags (before the path):

    -output=nie-bn8
    -output=png (this is the default)

When encoding you can also pass one of these flags (before the path):

    -output=ktx
    -output=pkm (this is the default)

The output image (in NIE/PNG or KTX/PKM format) is written to stdout.

Decode inputs KTX/PKM and outputs NIE/PNG.
Encode inputs BMP, GIF, JPEG, PNG, TIFF or WEBP and outputs KTX/PKM.
`

var ErrBadOutputFlag = errors.New("main: bad -output flag")

func main() {
	if err := main1(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func main1() error {
	flag.Usage = func() { os.Stderr.WriteString(usageStr) }
	flag.Parse()

	inFile := os.Stdin
	switch flag.NArg() {
	case 0:
		// No-op.
	case 1:
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			return err
		}
		defer f.Close()
		inFile = f
	default:
		return errors.New("too many filenames; the maximum is one")
	}

	if *decodeFlag && !*encodeFlag {
		return decode(inFile)
	}
	if !*decodeFlag && *encodeFlag {
		return encode(inFile)
	}
	return errors.New("must specify exactly one of -decode, -encode or -help")
}

func decode(inFile *os.File) error {
	switch *outputFlag {
	case "", "nie-bn8", "png":
		// No-op.
	default:
		return ErrBadOutputFlag
	}

	src, err := pkm.Decode(inFile)
	if err != nil {
		return err
	}
	if *outputFlag == "nie-bn8" {
		dst, err := nie.EncodeBN8(src)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(dst)
		return err
	}
	return png.Encode(os.Stdout, src)
}

func encode(inFile *os.File) error {
	panic("TODO")
}
