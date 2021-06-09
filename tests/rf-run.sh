#!/bin/sh
# Copyright 2020 Nokia
# Licensed under the BSD 3-Clause License.
# SPDX-License-Identifier: BSD-3-Clause


robot --consolecolors on -r none -l ./tests/out/$(basename $1)-log --output ./tests/out/$(basename $1)-out $1