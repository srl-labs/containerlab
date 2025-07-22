#!/bin/bash
# Copyright 2020 Nokia
# Licensed under the BSD 3-Clause License.
# SPDX-License-Identifier: BSD-3-Clause

# arguments
# $1 - container runtime: [docker, podman]
# $2 - test suite to execute

# set containerlab binary path to a value of CLAB_BIN env variable
# unless it is not set, then use 'containerlab' as a default value
if [ -z "${CLAB_BIN}" ]; then
  CLAB_BIN=containerlab
fi

export AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY

echo "Running tests with containerlab binary at $(which ${CLAB_BIN}) path and selected runtime: $1"

COV_DIR=/tmp/clab-tests/coverage

# coverage output directory
mkdir -p ${COV_DIR}

# parses the dir or file name passed to the rf-run.sh script
# and in case of a directory, it returns the name of the directory
# in case of a file it returns the name of the file's dir catenated with file name without extension
function get_logname() {
  path=$1
  filename=$(basename "$path")
  if [[ "$filename" == *.* ]]; then
    dirname=$(dirname "$path")
    basename=$(basename "$path" | cut -d. -f1)
    echo "${dirname##*/}-${basename}"
  else
    echo "${filename}"
  fi
}

# activate venv
source .venv/bin/activate

GOCOVERDIR=${COV_DIR} robot --consolecolors on -r none --variable CLAB_BIN:${CLAB_BIN} --variable runtime:$1 -l ./tests/out/$(get_logname $2)-$1-log --output ./tests/out/$(basename $2)-$1-out.xml $2
