#!/bin/sh

SLEEP=0

int_calc() {
	# Count the number of interfaces that are up matching the pattern
	# the pattern is a regex that matches the interface
	# et - matches et and eth
	# e[0-9] - matches srlinux e1* interfaces
	MYINT=$(ls -1 /sys/class/net/ | grep -E '^et|^ens|^eno|^e[0-9]' | wc -l)
}

int_calc

echo "Waiting for all $CLAB_INTFS interfaces to be connected"
while [ "$MYINT" -lt "$CLAB_INTFS" ]; do
	echo "Connected $MYINT interfaces out of $CLAB_INTFS"
	sleep 1
	int_calc
done

echo "Sleeping $SLEEP seconds before boot"
sleep $SLEEP
