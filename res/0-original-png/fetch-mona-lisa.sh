#!/bin/bash -eu
# Copyright 2025 The Etc2 Authors.
#
# Licensed under the Apache License, Version 2.0 <LICENSE-APACHE or
# https://www.apache.org/licenses/LICENSE-2.0>. This file may not be copied,
# modified, or distributed except according to those terms.
#
# SPDX-License-Identifier: Apache-2.0

wget -q -O mona-lisa.322x480.jpeg \
    'https://upload.wikimedia.org/wikipedia/commons/thumb/e/ec/Mona_Lisa%2C_by_Leonardo_da_Vinci%2C_from_C2RMF_retouched.jpg/322px-Mona_Lisa%2C_by_Leonardo_da_Vinci%2C_from_C2RMF_retouched.jpg'
convert mona-lisa.322x480.jpeg -resize 32x32 mona-lisa.21x32.png
pngcrush -ow -brute -rem alla mona-lisa.21x32.png
rm mona-lisa.322x480.jpeg
