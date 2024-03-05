#!/bin/bash

# Function to check if the nvued process is running
check_process() {
    nv config show > /dev/null
    return $?
}

# Wait loop
echo "Waiting for the process to start..."
while ! check_process; do
    sleep 1
done

echo "Process started. Applying NVUE config..."
/usr/bin/nv config replace /home/cumulus/nvue.yml
/usr/bin/nv config apply --assume-yes

echo "nv config applied:"
echo "#####################################"
/usr/bin/nv config show
echo "#####################################"
