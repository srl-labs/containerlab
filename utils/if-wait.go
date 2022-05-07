package utils

// IfWaitScript is used in ENTRYPOINT/CMD of the nodes that need to ensure that all
// of the clab links/interfaces are available in the container before calling the main process
var IfWaitScript string = `#!/bin/sh

INTFS=$(echo $CLAB_INTFS)
SLEEP=0

int_calc () 
{
    index=0
    for i in $(ls -1v /sys/class/net/ | grep -E '^et|^ens|^eno|^e[0-9]'); do
      let index=index+1
    done
    MYINT=$index
}

int_calc

echo "Waiting for all $INTFS interfaces to be connected"
while [ "$MYINT" -lt "$INTFS" ]; do
  echo "Connected $MYINT interfaces out of $INTFS"
  sleep 1
  int_calc
done

echo "Sleeping $SLEEP seconds before boot"
sleep $SLEEP
`
