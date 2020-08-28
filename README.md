# container-lab

## Description

Container-lab binary is setting up and destroying labs for networking containers.
Currently supporting standard linux containers as clients and networkign containers using SR-Linux

## installation

git clone https://github.com/srl-wim/container-lab

rpm -i contaianerlab-1.0.0.x86_64.rpm

## Usage

### build a lab configuration file

TODO

There are some examples in the labs sub directory

### Deploy the lab

sudo ./containerlab -a deploy

### destroy the lab

sudo ./containerlab -a destroy

### generating a graph

containerlab has the option to generate a topology graph using graphviz that can help showing the topology in a graph.

./containerlab -g

dot -Tps graph/wan-topo.dot -o graph/wan-topo.ps
dot -Tpng -Gdpi=300 graph/wan-topo.dot > graph/wan-topo.png
