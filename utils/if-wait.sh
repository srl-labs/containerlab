#!/bin/sh

# Validate CLAB_INTFS environment variable: it must be a non-negative integer.
# A value of 0 is legitimate (e.g. when only eth0 is waited for via CLAB_WAIT_ETH0).
REQUIRED_INTFS_NUM=${CLAB_INTFS:-0}
if ! echo "$REQUIRED_INTFS_NUM" | grep -qE '^[0-9]+$'; then
    echo "Warning: CLAB_INTFS=\"$CLAB_INTFS\" is not a valid number, treating as 0"
    REQUIRED_INTFS_NUM=0
fi

SLEEP=0
TIMEOUT=300  # 5 minute timeout
WAIT_TIME=0

int_calc() {
    if [ ! -d "/sys/class/net/" ]; then
        echo "Error: /sys/class/net/ not accessible"
        AVAIL_INTFS_NUM=0
        return 1
    fi
    
    # More comprehensive interface pattern including common container interfaces
    AVAIL_INTFS_NUM=$(ls -1 /sys/class/net/ 2>/dev/null | grep -cE '^(eth[1-9]|et[0-9]|ens|eno|enp|e[1-9]|net[0-9])')
    return 0
}

# Optionally also wait for eth0. eth0 is not matched by int_calc's pattern (it is
# normally the management interface, present from the start); when it is instead
# wired as a link it must be waited for too. Opt in with CLAB_WAIT_ETH0=1.
WAIT_ETH0=${CLAB_WAIT_ETH0:-0}

# eth0_ready is true unless we are asked to wait for eth0 and it is not present.
eth0_ready() {
    [ "$WAIT_ETH0" != "1" ] || [ -e /sys/class/net/eth0 ]
}

# Wait for the required data interfaces and, when requested, eth0.
if [ "$REQUIRED_INTFS_NUM" -gt 0 ] || [ "$WAIT_ETH0" = "1" ]; then
    WAIT_DESC="$REQUIRED_INTFS_NUM interface(s)"
    [ "$WAIT_ETH0" = "1" ] && WAIT_DESC="$WAIT_DESC + eth0"
    echo "Waiting for $WAIT_DESC to be connected (timeout: ${TIMEOUT}s)"

    while [ "$WAIT_TIME" -lt "$TIMEOUT" ]; do
        if ! int_calc; then
            echo "Failed to check interfaces, continuing..."
            break
        fi

        if [ "$AVAIL_INTFS_NUM" -ge "$REQUIRED_INTFS_NUM" ] && eth0_ready; then
            echo "Found $AVAIL_INTFS_NUM interfaces (required: $REQUIRED_INTFS_NUM)"
            break
        fi

        echo "Connected $AVAIL_INTFS_NUM interfaces out of $REQUIRED_INTFS_NUM (waited ${WAIT_TIME}s)"
        sleep 1
        WAIT_TIME=$((WAIT_TIME + 1))
    done

    if [ "$WAIT_TIME" -ge "$TIMEOUT" ]; then
        echo "Warning: Timeout reached, proceeding with $AVAIL_INTFS_NUM interfaces"
    fi
else
    echo "No interfaces to wait for, skipping interface wait"
fi

if [ "$SLEEP" -ne 0 ]; then
    echo "Sleeping $SLEEP seconds before boot"
    sleep $SLEEP
fi
