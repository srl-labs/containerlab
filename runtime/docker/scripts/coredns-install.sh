#!/bin/sh
#
# CoreDNS Installation Script for ContainerLab Tailscale integration
#
# This script installs CoreDNS and optionally Python3 (when NAT is enabled)
# into the Tailscale container for DNS resolution services.
#
# NOTE: This is a template file. Variables will be replaced by Go's text/template.
#

set -e  # Exit on any error

COREDNS_VERSION="{{.CoreDNSVersion}}"
NEEDS_PYTHON="{{.NeedsPython}}"

# Function to log errors
log_error() {
    echo "ERROR: $1" >&2
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check if CoreDNS is already installed with the correct version
if [ -f /usr/local/bin/coredns ]; then
    INSTALLED_VERSION=$(/usr/local/bin/coredns -version 2>/dev/null | grep -oP 'CoreDNS-\K[0-9.]+' || echo "unknown")
    if [ "$INSTALLED_VERSION" = "$COREDNS_VERSION" ]; then
        echo "CoreDNS $COREDNS_VERSION already installed, skipping download"
        
        # Still install Python if needed
        if [ "$NEEDS_PYTHON" = "true" ]; then
            if ! command_exists python3; then
                echo "Installing Python3..."
                if ! apk add --no-cache python3; then
                    log_error "Failed to install Python3"
                    exit 1
                fi
            fi
        fi
        
        # Ensure config directory exists
        mkdir -p /etc/coredns
        exit 0
    fi
fi

echo "Installing CoreDNS version $COREDNS_VERSION..."

# Verify apk is available
if ! command_exists apk; then
    log_error "apk package manager not found (this script requires Alpine Linux)"
    exit 1
fi

# Install packages (wget and optionally python3) in one command
if [ "$NEEDS_PYTHON" = "true" ]; then
    echo "Installing wget and Python3..."
    if ! apk add --no-cache wget python3; then
        log_error "Failed to install wget and Python3"
        exit 1
    fi
else
    echo "Installing wget..."
    if ! apk add --no-cache wget; then
        log_error "Failed to install wget"
        exit 1
    fi
fi

# Verify wget was installed
if ! command_exists wget; then
    log_error "wget installation failed"
    exit 1
fi

# Download and extract CoreDNS in one pipeline (faster than saving to file)
echo "Downloading CoreDNS binary..."
COREDNS_URL="https://github.com/coredns/coredns/releases/download/v${COREDNS_VERSION}/coredns_${COREDNS_VERSION}_linux_amd64.tgz"

if ! wget -qO- "$COREDNS_URL" | tar -xzC /usr/local/bin/; then
    log_error "Failed to download or extract CoreDNS from $COREDNS_URL"
    exit 1
fi

# Verify CoreDNS binary was extracted
if [ ! -f /usr/local/bin/coredns ]; then
    log_error "CoreDNS binary not found after extraction"
    exit 1
fi

# Set permissions
if ! chmod +x /usr/local/bin/coredns; then
    log_error "Failed to set executable permissions on CoreDNS binary"
    exit 1
fi

# Create config directory
if ! mkdir -p /etc/coredns; then
    log_error "Failed to create /etc/coredns directory"
    exit 1
fi

# Verify CoreDNS can execute
if ! /usr/local/bin/coredns -version >/dev/null 2>&1; then
    log_error "CoreDNS binary is not executable or corrupted"
    exit 1
fi

echo "CoreDNS installation completed successfully"
echo "Installed version: $(/usr/local/bin/coredns -version | head -1)"
