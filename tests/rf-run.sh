#!/bin/bash
# Copyright 2020 Nokia
# Licensed under the BSD 3-Clause License.
# SPDX-License-Identifier: BSD-3-Clause

# arguments
# $1 - runtime: [docker, podman, clabernetes]
# $2 - test suite to execute
# $3... - optional arguments passed to robot

set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <runtime> <test suite>"
  exit 1
fi

runtime=$1
suite=$2
shift 2
extra_robot_args=("$@")

if [ "${runtime}" = "c9s" ]; then
  runtime=clabernetes
fi

# set containerlab binary path to a value of CLAB_BIN env variable
# unless it is not set, then use 'containerlab' as a default value
if [ -z "${CLAB_BIN:-}" ]; then
  CLAB_BIN=containerlab
fi

export AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY

echo "Running tests with containerlab binary at $(command -v "${CLAB_BIN}") path and selected runtime: ${runtime}"

COV_DIR=/tmp/clab-tests/coverage

# coverage output directory
mkdir -p "${COV_DIR}"

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

robot_args=()
if [ "${runtime}" = "clabernetes" ]; then
  robot_args+=(--include clabernetes)
fi

GOCOVERDIR=${COV_DIR} uv run --project . -m robot \
  --consolecolors on \
  -r none \
  --variable CLAB_BIN:${CLAB_BIN} \
  --variable runtime:${runtime} \
  -l ./tests/out/$(get_logname "${suite}")-${runtime}-log \
  --output ./tests/out/$(basename "${suite}")-${runtime}-out.xml \
  "${extra_robot_args[@]}" \
  "${robot_args[@]}" \
  "${suite}"
