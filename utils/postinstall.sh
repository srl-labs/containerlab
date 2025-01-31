#!/bin/sh

# this post install script is used for setting up the clab_admins group
# and to count the number of installations of containerlab when the installation is done via apt or yum package manager

# exit if sh shell is not found
if [ ! -e /bin/sh ]; then
    exit 0
fi

chmod 4755 /usr/bin/containerlab

if [ ! -f /etc/containerlab/suid_setup_done ]; then
    groupadd -r clab_admins
    usermod -aG clab_admins "$SUDO_USER"
    touch /etc/containerlab/suid_setup_done
    echo "Please run the command 'sudo usermod -aG clab_admins <insert your username here> && newgrp clab_admins' to ensure that you are part of the Container admin group. You can check this by running 'groups'."
fi

# exit at this point if no /etc/apt/sources.list.d/netdevops.list or /etc/yum.repos.d/yum.fury.io_netdevops_.repo is found
# no need to count these installs
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

if type "curl" > /dev/null 2>&1; then
    curl --max-time 2 -sL -o /dev/null "$REPO_URL" || true
    exit 0
elif type "wget" > /dev/null 2>&1; then
    wget -T 2 -q -O /dev/null "$REPO_URL" || true
    exit 0
fi
