#!/bin/sh
#
# NAT Setup Script for ContainerLab Tailscale integration
# Applies iptables NETMAP rules for 1:1 NAT translation
#
# This script is injected into the container startup command to ensure
# NAT rules persist across container restarts.
#

# Configuration - these will be replaced by Go template
MGMT_SUBNET="{{.MgmtSubnet}}"
NAT_SUBNET="{{.NatSubnet}}"

echo "Starting containerboot in background..."
/usr/local/bin/containerboot &

# Wait for Tailscale to initialize
echo "Waiting for Tailscale to initialize..."
sleep 2

echo "Applying NAT rules..."
echo "  Management subnet: $MGMT_SUBNET"
echo "  NAT subnet: $NAT_SUBNET"

# DNAT: Translate incoming traffic to NAT subnet -> real mgmt subnet
iptables -t nat -A PREROUTING -d "$NAT_SUBNET" -j NETMAP --to "$MGMT_SUBNET"

# SNAT: Translate outgoing traffic from mgmt subnet -> NAT subnet (only via tailscale0)
iptables -t nat -A POSTROUTING -s "$MGMT_SUBNET" -o tailscale0 -j NETMAP --to "$NAT_SUBNET"

# Allow forwarding between subnets
iptables -A FORWARD -s "$MGMT_SUBNET" -d "$NAT_SUBNET" -j ACCEPT
iptables -A FORWARD -s "$NAT_SUBNET" -d "$MGMT_SUBNET" -j ACCEPT

echo "NAT rules applied successfully"

# Wait for containerboot (keep container running)
wait
