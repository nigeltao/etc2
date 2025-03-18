#!/bin/bash -eu
# Copyright 2025 The Etc2 Authors.
#
# Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
# https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
# modified, or distributed except according to those terms.
#
# SPDX-License-Identifier: Apache-2.0

wget -q -O dice.800x600.jpeg \
    'https://upload.wikimedia.org/wikipedia/commons/4/47/PNG_transparency_demonstration_1.png'
convert dice.800x600.jpeg -resize 80x60 dice.80x60.png
pngcrush -ow -brute -rem alla dice.80x60.png
rm dice.800x600.jpeg
