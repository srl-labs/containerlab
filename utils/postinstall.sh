#!/bin/bash

# this post install script is used to count the number of installations of containerlab
# when the installation is done via apt or yum package manager

# exit if bash shell is not found
if [ ! -e /bin/bash ]; then
    exit 0
fi

# exit if no /etc/apt/sources.list.d/netdevops.list or /etc/yum.repos.d/yum.fury.io_netdevops_.repo is found
if [ ! -e /etc/apt/sources.list.d/netdevops.list ] && [ ! -e /etc/yum.repos.d/yum.fury.io_netdevops_.repo ]; then
    exit 0
fi

# run `containerlab version` and parse the version from the output
version=$(containerlab version | awk '/version:/{print $2}')

if [ -z "$version" ]; then
    exit 0
fi

# prefixed with v
rel_version=v${version}

REPO_URL="https://github.com/srl-labs/containerlab/releases/download/${rel_version}/checksums.txt"

if type "curl" &>/dev/null; then
    curl --max-time 2 -sL -o /dev/null $REPO_URL || true
    exit 0
elif type "wget" &>/dev/null; then
    wget -T 2 -q -O /dev/null $REPO_URL || true
    exit 0
fi
