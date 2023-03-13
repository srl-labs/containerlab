#!/bin/sh
# Copyright 2020 Nokia
# Licensed under the BSD 3-Clause License.
# SPDX-License-Identifier: BSD-3-Clause

# arguments
# $1 - container runtime: [docker, containerd]
# $2 - test suite to execute

# set containerlab binary path to a value of CLAB_BIN env variable
# unless it is not set, then use 'containerlab' as a default value
if [[ -z "${CLAB_BIN}" ]]; then
  export CLAB_BIN="containerlab"
fi

echo "Running tests with containerlab binary at $(which ${CLAB_BIN}) path and selected runtime: $1"

robot --consolecolors on -r none --variable runtime:$1 -l ./tests/out/$(basename $2)-$1-log --output ./tests/out/$(basename $2)-$1-out.xml $2