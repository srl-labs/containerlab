#!/bin/sh
#
# CoreDNS Installation Script for ContainerLab Tailscale integration
#
# This script installs CoreDNS and optionally Python3 (when NAT is enabled)
# into the Tailscale container for DNS resolution services.
#

COREDNS_VERSION="{{.CoreDNSVersion}}"
NEEDS_PYTHON="{{.NeedsPython}}"

echo "Installing CoreDNS version $COREDNS_VERSION..."

# Install packages (wget and optionally python3) in one command
if [ "$NEEDS_PYTHON" = "true" ]; then
    echo "Installing wget and Python3..."
    apk add --no-cache wget python3
else
    apk add --no-cache wget
fi

# Download and extract CoreDNS in one pipeline (faster than saving to file)
echo "Downloading CoreDNS binary..."
wget -qO- https://github.com/coredns/coredns/releases/download/v${COREDNS_VERSION}/coredns_${COREDNS_VERSION}_linux_amd64.tgz | \
    tar -xzC /usr/local/bin/

# Set permissions and create config directory
chmod +x /usr/local/bin/coredns
mkdir -p /etc/coredns

echo "CoreDNS installation completed successfully"
