#!/bin/bash

set -e

# make sure gnmic is installed
gnmic --help >/dev/null
# make sure gomplate is installed
gomplate --help >/dev/null
# make sure curl is installed
curl --help >/dev/null

# generate the variables file based on the number of spines and leaves in the topology
gomplate -f topology_config.gotmpl -d templated01.clab_vars.yaml >vars.yaml

# build targets string
targets=$(docker ps -f label=clab-node-kind=nokia_srlinux -f label=containerlab=templated01 --format {{.Names}} | paste -s -d, -)
# base gnmic command
gnmic_cmd="gnmic --log -a ${targets} --skip-verify -u admin -p NokiaSrl1!"

curl -sLO https://raw.githubusercontent.com/karimra/gnmic/main/examples/set-request-templates/Nokia/SRL/1.interfaces/interfaces_template.gotmpl
curl -sLO https://raw.githubusercontent.com/karimra/gnmic/main/examples/set-request-templates/Nokia/SRL/1.interfaces/subinterfaces_template.gotmpl

# run gNMI interfaces config
$gnmic_cmd \
      set \
      --request-file interfaces_template.gotmpl \
      --request-vars vars.yaml

# run gNMI subinterfaces config
$gnmic_cmd \
      set \
      --request-file subinterfaces_template.gotmpl \
      --request-vars vars.yaml

# delete generated variables file
rm vars.yaml

# delete downloaded templates
rm interfaces_template.gotmpl
rm subinterfaces_template.gotmpl
