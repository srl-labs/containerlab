#!/usr/bin/env bash
# Copyright 2023 Nokia
# Licensed under the BSD 3-Clause License.
# SPDX-License-Identifier: BSD-3-Clause

# this is an example of a bash script that can be used to introduce network impairments


set -o errexit
set -o pipefail

# change this to your own lab name (this is clab-<labname>)
LAB_NAME="clab-netem"

# default values, overridden by command-line arguments
cmd="" # command to be executed, one of [delay]
node=""
interface=""
time="100" # default time delay is 100ms
jitter=""
correlation=""
distribution=""
percent="0"
duration="8760h" # 1 year duration by default

# -----------------------------------------------------------------------------
# Helper functions.
# -----------------------------------------------------------------------------

# usage examples
DELAY_USAGE="Usage: ./netem.sh delay -n <container name without lab prefix> -i <inteface-name> [-t <delay in ms>] [-d <impairment duration with ms/s/m/h suffix (default is 8760h)>] [-j <jitter in ms>] [-c <correlation>] [-r <distribution>]"
LOSS_USAGE="Usage: ./netem.sh delay -n <container name without lab prefix> -i <inteface-name> [-p <loss percent>] [-c <correlation>]"

# Read command-line arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        -n|--node)
            node="$2"
            shift 2
            ;;
        -i|--interface)
            interface="$2"
            shift 2
            ;;
        -t|--time)
            time="$2"
            shift 2
            ;;
        -j|--jitter)
            jitter="$2"
            shift 2
            ;;
        -c|--correlation)
            correlation="$2"
            shift 2
            ;;
        -r|--distribution)
            distribution="$2"
            shift 2
            ;;
        -p|--percent)
            percent="$2"
            shift 2
            ;;
        -d|--duration)
            duration="$2"
            shift 2
            ;;
        delay)
            cmd="delay"
            shift 1
            ;;
        loss)
            cmd="loss"
            shift 1
            ;;
        stop-all)
            cmd="stop-all"
            shift 1
            ;;
        *)
            echo "Unknown flag: $1"
            exit 1
            ;;
    esac
done

# -----------------------------------------------------------------------------
# netem functions.
# Allowed to be customized by a user.
# -----------------------------------------------------------------------------

# introduce a delay on a selected link for a selected container
function delay {
    check-mandatory-parameters ${DELAY_USAGE}

    handle_optional_parameters

    echo "Starting delay function for node \"${node}:${interface}\" with time=${time}ms ${JITTER_ECHO} ${CORRELATION_ECHO} ${DISTRIBUTION_ECHO}"

    sudo docker run -d --name netem-${LAB_NAME}-${RANDOM} -it --rm -v /var/run/docker.sock:/var/run/docker.sock gaiaadm/pumba \
        netem --tc-image ghcr.io/srl-labs/iproute2 --duration ${duration} --interface ${interface} \
        delay --time ${time} \
            ${JITTER_FLAGS} ${CORRELATION_FLAGS} ${DISTRIBUTION_FLAGS} \
            ${LAB_NAME}-${node} > /dev/null
}

# introduce a packet loss on a selected link for a selected container
function loss {
    check-mandatory-parameters ${LOSS_USAGE}

    handle_optional_parameters

    echo "Starting loss function for node \"${node}:${interface}\" with percent=${percent}% ${CORRELATION_ECHO}"

    sudo docker run --name netem-${LAB_NAME}-${RANDOM} -it --rm -v /var/run/docker.sock:/var/run/docker.sock gaiaadm/pumba \
        netem --tc-image ghcr.io/srl-labs/iproute2 --duration ${duration} --interface ${interface} \
        loss --percent ${percent} \
            ${CORRELATION_FLAGS} \
            ${LAB_NAME}-${node} > /dev/null
}

# remove all network emulation containers effectively stopping all link impairments
# started by this script.
function stop-all {
    sudo docker ps -a -q -f name="netem-${LAB_NAME}" | xargs -n1 sudo docker stop
}

function check-mandatory-parameters {
    if [[ -z $node ]]; then
    echo "Error: Node is not set or empty.\nUsage: $1"
    exit 1
    fi

    if [[ -z $interface ]]; then
        echo "Error: Node is not set or empty.\nUsage: $1"
        exit 1
    fi
}

# handling optional parameters
function handle_optional_parameters {
    if [[ ! -z $jitter ]]; then
        JITTER_FLAGS="--jitter ${jitter}" # flag/value to append to the command
        JITTER_ECHO="jitter=${jitter}ms" # echo to print
    fi

    if [[ ! -z $correlation ]]; then
        CORRELATION_FLAGS="--correlation ${correlation}"
        CORRELATION_ECHO="correlation=${correlation}"
    fi

    if [[ ! -z $distribution ]]; then
        DISTRIBUTION_FLAGS="--distribution ${distribution}"
        DISTRIBUTION_ECHO="distribution=${distribution}"
    fi
}

# invoke the command
${cmd}


# -----------------------------------------------------------------------------
# Bash runner functions.
# -----------------------------------------------------------------------------
function help {
  printf "%s <task> [args]\n\nTasks:\n" "${0}"

  compgen -A function | grep -v "^_" | cat -n

  printf "\nExtended help:\n  Each task has comments for general usage\n"
}
