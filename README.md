# Container-lab

## Description

Containerlab provides a framework for setting up and destroying labs for networking containers. it builds a virtual wiring using veth pairs between the containers to provide virtual topologies. A CA can be provided per lab and when enabled, containerlab generates certificates per device that can be used for various use cases, like GNMI, JSON RPC, etc. Lastly, containerlab also allows for a graphical outpur to validate the lab in a visual format using [graphviz](https://graphviz.org)

Containerlab supports the following containers:

* standard linux/alpine containers, typically used as test clients
* networking containers:
	* Nokia SR-Linux
	* Arista cEOS.

Containerlab is build in [golang](https://golang.org) for people interested in the source code.

## Installation

### Pre-requisites

* Have sudo rights on the system: containerlab is using some sudo commands to set some parameters in the linux system to support the various options the containers need
* Install [docker](https://www.docker.com): this is used manage the containers
* Install [golang](https://golang.org): this is used for the following step in support of installing [cfssl](https://cfssl.org)
* Install [cfssl](https://cfssl.org): To build a CA per lab, containerlab is leveraging cfssl, build by cloudflare,  to manage the certificates
	* go get -u github.com/cloudflare/cfssl/cmd/...
<<<<<<< Updated upstream
* load the container images in docker locally

### Cloning the repo

git clone https://github.com/srl-wim/container-lab
=======
* Ensure you have a license.key for the srl containers
>>>>>>> Stashed changes

### Using rpm installation

sudo rpm -i contaianerlab-1.0.0.x86_64.rpm

## Usage

### Build a lab configuration file

To help build the lab topologies a YAML file is used with the following parameters:

* Prefix: The prefix can be seen as a namespace for the lab to make them unique.
* Docker_info: we use docker to manage the various containers. In this section we specify the management bridge name and the IPV4 and IPV6 prefixes we use for connecting as an OOB management to the containers
	* bridge
	* ipv4_subnet
	* ipv6_subnet
* Duts: this section provides information with respect to the dut containers that are used in the lab. The dut configuration provides an inheritance to optimize the configuration in 3 levels: global_defaults, kind_defaults, dut_specifics.
	*  	global_defaults: This section specifies the global defaults and will be inherited if the parameters are not specified in the more specific sections. As an example if kind = srl is specified in the global_defaults section and the kind is not specified in the kind_defaults or dut_specifics sections, the container will be using kind = srl
		* Kind: the kind of container e.g. srl, ceos or alpine
		* Group: used in the graph output, to help visualize the output
	* kind_defaults: This section specifies the kind defaults
		* type: the type of container. e.g. to use 7220-dx, or 7220-ixr series
		* config: the config file that is used by default for this kind of container
		* image: the image that is used for this kind of container
		* license: the license file that is used for this kind of container
	* dut_specifics: This section specifies the dut specific details. If parameters are not set the can be inherited from higher sections.
		* 	Kind: the kind of container e.g. srl, ceos or alpine
		*  Group: used in the graph output, to help visualize the output
	* kind_defaults: This section specifies the kind defaults
		* <dutName>
			* type: the type of container. e.g. to use 7220-dx, or 7220-ixr series
			* config: the config file that is used for this specific dut
			* image: the image that is used for this specific dut
			* license: the license file that is used for this specific dut
* Links: Define the virtual wiring for the lab
	* endpoints: define the virtual wire specified as: 
	```
	["<dutName-A>:<intf-dutName-A>", "<dutName-B>:<intf-dutName-B>"]
	```

There are some examples in the labs sub directory

```
Prefix: test
Docker_info: 
  bridge: srlinux_bridge
  ipv4_subnet: "172.19.19.0/24"
  ipv6_subnet: "2001:172:19:19::/80"

Duts:
  global_defaults:
    kind: srl
    group: bb
  kind_defaults:
    srl:
      type: ixr6
      config: templates/srl/config.json
      image: srlinux:20.6.1-286
      license: templates/srl/license.key
    alpine:
      image: henderiw/client-alpine:1.0.0
  dut_specifics:
    wan1: 
    wan2: 

Links:
  - endpoints: ["wan1:e1-1", "wan2:e1-1"]
```

### Deploy the lab

To deploy a lab there are a few parameters that can be used:

* -a, --action string   action: deploy or destroy
* -d, --debug           set log level to debug
* -c, --gen-certs       generate a certificate per container (default true)
* -g, --graph           generate a graph of the topology
* -t, --topo string     YAML file with topology information (default "labs/wan-topo.yml")

Example:

```
./containerlab -t labs/wan-topo.yml -a deploy
```

### Destroy the lab

Example:

```
./containerlab -t labs/wan-topo.yml -a destroy
```

### Generating a graph for the lab

containerlab has the option to generate a topology graph using [graphviz](https://graphviz.org) that can help showing the topology in a graph.

```
./containerlab -g
```

this commands generates some files in the lab topology

```
dot -Tps graph/wan-topo.dot -o graph/wan-topo.ps
dot -Tpng -Gdpi=300 graph/wan-topo.dot > graph/wan-topo.png
```

## logging in into the containers

### SRL

```
* ssh admin@<mgmt-ip-address>
* docker exec -ti <container-name> sr_cli
* docker exec -ti <container-name> /bin/bash
```
### cEOS

```
* ssh admin@<mgmt-ip-address>
* docker exec -ti <container-name> Cli
```