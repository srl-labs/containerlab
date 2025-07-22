#!/bin/bash

# Roman: copied from https://github.com/containers/podman/discussions/25582#discussioncomment-12803424

# Must be run as root
if [ "$EUID" -ne 0 ]; then
  echo "Please run as root (e.g., sudo $0)"
  exit 1
fi

# Define file paths
PINNING_FILE="/etc/apt/preferences.d/podman-plucky.pref"
SOURCE_LIST="/etc/apt/sources.list.d/plucky.list"

# Write Plucky APT source list
echo "Adding Plucky repo to $SOURCE_LIST..."
echo "deb http://archive.ubuntu.com/ubuntu plucky main universe" > "$SOURCE_LIST"

# Write APT pinning rules
echo "Writing APT pinning rules to $PINNING_FILE..."
cat <<EOF > "$PINNING_FILE"
Package: podman buildah golang-github-containers-common crun libgpgme11t64 libgpg-error0 golang-github-containers-image catatonit conmon containers-storage
Pin: release n=plucky
Pin-Priority: 991

Package: libsubid4 netavark passt aardvark-dns containernetworking-plugins libslirp0 slirp4netns
Pin: release n=plucky
Pin-Priority: 991

Package: *
Pin: release n=plucky
Pin-Priority: 400
EOF

# Update APT cache
echo "Updating APT package list..."
apt update

echo "Plucky pinning setup complete."