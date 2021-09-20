#!/bin/bash

CFG_DIR=./configs

configure_SRL () {
  OUT=$(gnmic -a clab-clos02-$1 --timeout 30s -u admin -p admin -e json_ietf --skip-verify set --update-path / --update-file $CFG_DIR/$1.yaml 2>&1)
  echo $OUT | grep -q -e '\"operation\": \"UPDATE\"'
  if [ $? -eq 0 ]; then
    SAVE=$(docker exec -it clab-clos02-$1 sr_cli "save startup" 2>&1)
  else
    echo "Error: Unable to push config. Check if NE is reachable with gRPC enabled."
  fi
}

if [ $# -eq 0 ]; then
  echo "Error: No topology file passed."
else
  DEPLOY=$(containerlab deploy --topo $1 2>&1)
  echo $DEPLOY | grep -q -e 'Adding containerlab host entries'
  if [ $? -eq 0 ]; then
    echo
    echo "Containerlab (clos02) created:"
    containerlab inspect --topo $1
    echo
    echo "01 Configuring leaf1..."
    configure_SRL "leaf1"
    echo "02 Configuring leaf2..."
    configure_SRL "leaf2"
    echo "03 Configuring leaf3..."
    configure_SRL "leaf3"
    echo "04 Configuring leaf4..."
    configure_SRL "leaf4"
    echo "05 Configuring spine1..."
    configure_SRL "spine1"
    echo "06 Configuring spine2..."
    configure_SRL "spine2"
    echo "07 Configuring spine3..."
    configure_SRL "spine3"
    echo "08 Configuring spine4..."
    configure_SRL "spine4"
    echo "09 Configuring superspine1..."
    configure_SRL "superspine1"
    echo "10 Configuring superspine2..."
    configure_SRL "superspine2"
    echo
    echo "11 Configuring client1 IP addressing..."
    docker cp $CFG_DIR/client1.sh clab-clos02-client1:/tmp/
    docker exec -it clab-clos02-client1 bash /tmp/client1.sh
    echo "12 Configuring client2 IP addressing..."
    docker cp $CFG_DIR/client2.sh clab-clos02-client2:/tmp/
    docker exec -it clab-clos02-client2 bash /tmp/client2.sh
    echo "13 Configuring client3 IP addressing..."
    docker cp $CFG_DIR/client3.sh clab-clos02-client3:/tmp/
    docker exec -it clab-clos02-client3 bash /tmp/client3.sh
    echo "14 Configuring client4 IP addressing..."
    docker cp $CFG_DIR/client4.sh clab-clos02-client4:/tmp/
    docker exec -it clab-clos02-client4 bash /tmp/client4.sh
    echo
    echo "Success: Setup finished"
    echo
  else
    echo "Error: Not a valid topology file."
  fi
fi