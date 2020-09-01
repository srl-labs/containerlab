# container-lab

## Description

Container-lab binary is setting up and destroying labs for networking containers.
Currently supporting standard linux containers as clients and networking containers using Nokia SR-Linux, Arista cEOS

## installation

## Prerequisites for installing CA

go get -u github.com/cloudflare/cfssl/cmd/...

### cloning the repo

git clone https://github.com/srl-wim/container-lab

### using rpm installation

rpm -i contaianerlab-1.0.0.x86_64.rpm

## Usage

### build a lab configuration file

TODO

There are some examples in the labs sub directory

### Deploy the lab

sudo ./containerlab -t labs/wan-topo.yml -a deploy

### destroy the lab

sudo ./containerlab -t labs/wan-topo.yml -a destroy

### generating a graph

containerlab has the option to generate a topology graph using graphviz that can help showing the topology in a graph.

./containerlab -g

dot -Tps graph/wan-topo.dot -o graph/wan-topo.ps
dot -Tpng -Gdpi=300 graph/wan-topo.dot > graph/wan-topo.png

## logging in into the containers

### SRL

#### SRL login to the cli shell

docker exec -ti <container-name> sr_cli

#### SRL login to the bash shell

docker exec -ti <container-name> /bin.bash

### cEOS

docker exec -ti <container-name> Cli
