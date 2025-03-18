#!/bin/bash -eu
# Copyright 2025 The Etc2 Authors.
#
# Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
# https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
# modified, or distributed except according to those terms.
#
# SPDX-License-Identifier: Apache-2.0

wget -q -O pearl-earring.506x599.jpeg \
    'https://upload.wikimedia.org/wikipedia/commons/thumb/0/0f/1665_Girl_with_a_Pearl_Earring.jpg/506px-1665_Girl_with_a_Pearl_Earring.jpg'
convert pearl-earring.506x599.jpeg -resize 64x64 pearl-earring.54x64.png
pngcrush -ow -brute -rem alla pearl-earring.54x64.png
rm pearl-earring.506x599.jpeg
