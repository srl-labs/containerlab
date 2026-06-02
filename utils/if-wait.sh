#!/bin/sh

# Validate CLAB_INTFS environment variable
REQUIRED_INTFS_NUM=${CLAB_INTFS:-0}
if ! echo "$REQUIRED_INTFS_NUM" | grep -qE '^[0-9]+$' || [ "$REQUIRED_INTFS_NUM" -eq 0 ]; then
    echo "Warning: CLAB_INTFS not set or invalid, skipping interface wait"
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

# Only wait for interfaces if CLAB_INTFS is set
if [ "$REQUIRED_INTFS_NUM" -gt 0 ]; then
    echo "Waiting for $REQUIRED_INTFS_NUM interfaces to be connected (timeout: ${TIMEOUT}s)"
    
    while [ "$WAIT_TIME" -lt "$TIMEOUT" ]; do
        if ! int_calc; then
            echo "Failed to check interfaces, continuing..."
            break
        fi
        
        if [ "$AVAIL_INTFS_NUM" -ge "$REQUIRED_INTFS_NUM" ]; then
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
fi

if [ "$SLEEP" -ne 0 ]; then
    echo "Sleeping $SLEEP seconds before boot"
    sleep $SLEEP
fi
