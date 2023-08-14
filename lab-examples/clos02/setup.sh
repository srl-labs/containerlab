#!/bin/bash

CFG_DIR=./configs
SRL_PASSWORD=NokiaSrl1!

configure_SRL() {
  OUT=$(gnmic -a clab-clos02-$1 --timeout 30s -u admin -p $SRL_PASSWORD -e json_ietf --skip-verify set --update-path / --update-file $CFG_DIR/$1.yaml 2>&1)
  echo $OUT | grep -q -e '\"operation\": \"UPDATE\"'
  if [ $? -eq 0 ]; then
    docker exec clab-clos02-$1 sr_cli "save startup" > /dev/null
  else
    echo "Error: Unable to push config into clab-clos02-$1."
  fi
  echo $OUT > /dev/null
}

configure_CLIENT() {
  docker cp $CFG_DIR/$1.sh clab-clos02-$1:/tmp/
  docker exec clab-clos02-$1 bash /tmp/$1.sh 2>/dev/null
}

echo
PIDS=""
NE=("leaf1" "leaf2" "leaf3" "leaf4" "spine1" "spine2" "spine3" "spine4" "superspine1" "superspine2")
CLIENT=("client1" "client2" "client3" "client4")

for VARIANT in ${NE[@]}; do
  ( configure_SRL $VARIANT ) &
  REF=$!
  echo "[$REF] Configuring $VARIANT..."
  PIDS+=" $REF"
done

for VARIANT in ${CLIENT[@]}; do
  ( configure_CLIENT $VARIANT ) &
  REF=$!
  echo "[$REF] Configuring $VARIANT..."
  PIDS+=" $REF"
done

echo
for p in $PIDS; do
  if wait $p; then
    echo "Process $p success"
  else
    echo "Process $p fail"
  fi
done
echo