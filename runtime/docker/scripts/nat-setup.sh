#!/bin/sh
#
# NAT Setup Script for ContainerLab Tailscale integration
# Applies iptables NETMAP rules for 1:1 NAT translation
#
# This script is injected into the container startup command to ensure
# NAT rules persist across container restarts.
#
# NOTE: This is a template file. Variables will be replaced by Go's text/template.
#       The { { . Variable } } syntax is shell script compatible (within quotes).
#

# Configuration - these will be replaced by Go template
MGMT_SUBNET="{{.MgmtSubnet}}"
NAT_SUBNET="{{.NatSubnet}}"

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Verify iptables is available
if ! command_exists iptables; then
    echo "ERROR: iptables not found in container" >&2
    exit 1
fi

echo "Starting containerboot in background..."
/usr/local/bin/containerboot &
CONTAINERBOOT_PID=$!

# Wait for Tailscale to initialize
echo "Waiting for Tailscale to initialize..."
sleep 2

# Verify tailscale0 interface exists
RETRY_COUNT=0
MAX_RETRIES=10
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if ip link show tailscale0 >/dev/null 2>&1; then
        echo "Tailscale interface ready"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    echo "Waiting for tailscale0 interface... ($RETRY_COUNT/$MAX_RETRIES)"
    sleep 1
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo "WARNING: tailscale0 interface not found after $MAX_RETRIES attempts" >&2
    echo "NAT rules may not work correctly" >&2
fi

echo "Applying NAT rules..."
echo "  Management subnet: $MGMT_SUBNET"
echo "  NAT subnet: $NAT_SUBNET"

# DNAT: Translate incoming traffic to NAT subnet -> real mgmt subnet
if ! iptables -t nat -A PREROUTING -d "$NAT_SUBNET" -j NETMAP --to "$MGMT_SUBNET"; then
    echo "ERROR: Failed to apply DNAT rule" >&2
    exit 1
fi

# SNAT: Translate outgoing traffic from mgmt subnet -> NAT subnet (only via tailscale0)
if ! iptables -t nat -A POSTROUTING -s "$MGMT_SUBNET" -o tailscale0 -j NETMAP --to "$NAT_SUBNET"; then
    echo "ERROR: Failed to apply SNAT rule" >&2
    exit 1
fi

# Allow forwarding between subnets
if ! iptables -A FORWARD -s "$MGMT_SUBNET" -d "$NAT_SUBNET" -j ACCEPT; then
    echo "ERROR: Failed to apply forward rule (mgmt->nat)" >&2
    exit 1
fi

if ! iptables -A FORWARD -s "$NAT_SUBNET" -d "$MGMT_SUBNET" -j ACCEPT; then
    echo "ERROR: Failed to apply forward rule (nat->mgmt)" >&2
    exit 1
fi

echo "NAT rules applied successfully"

# List applied rules for debugging
echo "Active NAT rules:"
iptables -t nat -L -n -v | grep -E "NETMAP|Chain"

# Wait for containerboot (keep container running)
wait $CONTAINERBOOT_PID
