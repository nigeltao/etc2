#!/bin/bash -eu
# Copyright 2025 The Etc2 Authors.
#
# Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
# https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
# modified, or distributed except according to those terms.
#
# SPDX-License-Identifier: Apache-2.0

wget -q -O water-lillies.617x599.jpeg \
    'https://upload.wikimedia.org/wikipedia/commons/thumb/9/99/Water-Lilies-and-Japanese-Bridge-%281897-1899%29-Monet.jpg/617px-Water-Lilies-and-Japanese-Bridge-%281897-1899%29-Monet.jpg'
convert water-lillies.617x599.jpeg -resize 64x64 water-lillies.64x62.png
pngcrush -ow -brute -rem alla water-lillies.64x62.png
rm water-lillies.617x599.jpeg
