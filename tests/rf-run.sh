#!/bin/sh
# Copyright 2020 Nokia
# Licensed under the BSD 3-Clause License.
# SPDX-License-Identifier: BSD-3-Clause

# arguments
# $1 - container runtime: [docker, containerd]
# $2 - test suite to execute

robot --consolecolors on -r none --variable runtime:$1 -l ./tests/out/$(basename $2)-$1-log --output ./tests/out/$(basename $2)-$1-out $2