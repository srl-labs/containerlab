### Build a lab configuration file

To help build the lab topologies a YAML file is used with the following parameters:

* `Prefix`: The prefix can be seen as a namespace for the lab to make them unique.
* `Docker_info`: we use docker to manage the various containers. In this section we specify the management bridge name and the IPV4 and IPV6 prefixes we use for connecting as an OOB management to the containers
	* `bridge`
	* `ipv4_subnet`
	* `ipv6_subnet`
* `Duts`: this section provides information with respect to the dut containers that are used in the lab. The dut configuration provides an inheritance to optimize the configuration in 3 levels: global_defaults, kind_defaults, dut_specifics.
	*  	`global_defaults`: This section specifies the global defaults and will be inherited if the parameters are not specified in the more specific sections. As an example if kind = srl is specified in the global_defaults section and the kind is not specified in the kind_defaults or dut_specifics sections, the container will be using kind = srl
		* `kind`: the kind of container e.g. srl, ceos, alpine, linux or bridge
			* *bridge* is a special kind and is used to connect to an external bridge
		* `group`: used in the graph output, to help visualize the output
	* `kind_defaults`: This section specifies the kind defaults
		* `type`: the type of container. e.g. to use 7220-dx, or 7220-ixr series
		* `config`: the config file that is used by default for this kind of container
		* `image`: the image that is used for this kind of container
		* `license`: the license file that is used for this kind of container
	* `dut_specifics`: This section specifies the dut specific details. If parameters are not set the can be inherited from higher sections.
		* 	`kind`: the kind of container e.g. srl, ceos, alpine, linux or bridge
		*  `group`: used in the graph output, to help visualize the output
	* `kind_defaults`: This section specifies the kind defaults
		* `<dutName>`
			* `type`: the type of container. e.g. to use 7220-dx, or 7220-ixr series
			* `config`: the config file that is used for this specific dut
			* `image`: the image that is used for this specific dut
			* `license`: the license file that is used for this specific dut
* `Links`: Define the virtual wiring for the lab
	* `endpoints`: define the virtual wire specified as: 
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

```
[henderiw@srlinux-2 clab]$ containerlab -h
deploy container based lab environments with a user-defined interconnections

Usage:
  containerlab [command]

Available Commands:
  deploy      deploy a lab
  destroy     destroy a lab
  exec        execute a command on one or multiple containers
  graph       generate a topology graph
  help        Help about any command
  save        save containers configuration
  version     show containerlab version

Flags:
  -d, --debug   enable debug mode
  -h, --help    help for containerlab

Use "containerlab [command] --help" for more information about a command.
```

Example:

```
./containerlab deploy -t lab-examples/wan-topo.yml 
```

### Destroy the lab

Example:

```
./containerlab destroy -t lab-examples/wan-topo.yml
```

### Generating a graph for the lab

containerlab has the option to generate a topology graph based on [graphviz](https://graphviz.org), which can be utilized to generate a picture of the topology.

```
./containerlab graph -t lab-examples/wan-topo.yml
```

If graphviz is installed on your system (see pre-requisites), this will generate the graphviz .dot file as well as a final png file which can be viewed in any image viewer.
If the graphviz executabe `dot` is not found on your system, just the `<topology_name>.dot` file is created which can then be transformed into an image via e.g. this website https://dreampuf.github.io/GraphvizOnline.


### Logging in into the containers

#### SRL

```
* ssh admin@<mgmt-ip-address>
* docker exec -ti <container-name> sr_cli
* docker exec -ti <container-name> /bin/bash
```
#### cEOS

```
* ssh admin@<mgmt-ip-address>
* docker exec -ti <container-name> Cli
```