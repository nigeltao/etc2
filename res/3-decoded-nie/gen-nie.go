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
	"image/png"
	"os"
	"strings"

	"github.com/nigeltao/etc2/internal/nie"
)

const srcDirName = "../2-decoded-png"

func main() {
	if err := main1(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func main1() error {
	entries, err := os.ReadDir(srcDirName)
	if err != nil {
		return fmt.Errorf("os.ReadDir: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".png") {
			continue
		}
		if err := do(name); err != nil {
			return err
		}
	}
	return nil
}

func do(name string) error {
	f, err := os.Open(srcDirName + "/" + name)
	if err != nil {
		return fmt.Errorf("os.Open: %v", err)
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("png.Decode: %v", err)
	}
	enc, err := nie.EncodeBN8(src)
	if err != nil {
		return fmt.Errorf("nie.EncodeBN8: %v", err)
	}
	err = os.WriteFile(name[:len(name)-3]+"nie", enc, 0666)
	if err != nil {
		return fmt.Errorf("os.WriteFile: %v", err)
	}
	return nil
}
