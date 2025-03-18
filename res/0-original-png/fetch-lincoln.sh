#!/bin/bash -eu
# Copyright 2025 The Etc2 Authors.
#
# Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
# https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
# modified, or distributed except according to those terms.
#
# SPDX-License-Identifier: Apache-2.0

wget -q -O lincoln.449x599.jpeg \
    'https://upload.wikimedia.org/wikipedia/commons/thumb/5/57/Abraham_Lincoln_1863_Portrait_%283x4_cropped%29.jpg/449px-Abraham_Lincoln_1863_Portrait_%283x4_cropped%29.jpg'
convert lincoln.449x599.jpeg -resize 32x32 lincoln.24x32.png
pngcrush -ow -brute -rem alla lincoln.24x32.png
rm lincoln.449x599.jpeg
