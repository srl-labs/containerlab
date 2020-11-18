# generate command

### Description

The `generate` command generates the topology definition file based on the user input provided via CLI flags.

With this command it is possible to generate definition file for a CLOS fabric by just providing the number of nodes on each tier. The generated topology can be saved in a file or immediately scheduled for deployment.

It is assumed, that the interconnection between the tiers is done in a full-mesh fashion. Such as tier1 nodes are fully meshed with tier2, tier2 is meshed with tier3 and so on.

### Usage

`containerlab [global-flags] generate [local-flags]`

**aliases:** `gen`

### Flags

#### name

With the global `--name | -n` flag a user sets the name of the lab that will be generated.

#### nodes
The user configures the CLOS fabric topology by using the `--nodes` flag. The flag value is a comma separated list of CLOS tiers where each tier is defined by the number of nodes, its kind and type:
```
 num_of_nodes:kind:type,num_of_nodes:kind:type,etc
^                     ^^                     ^
|________tier1________||________tier2________|
```

For example, the following flag value will define a 2-tier CLOS fabric with tier1 (leafs) consists of 4x SR Linux containers of IXR6 type and the 2x Arista cEOS spines:
```
4:srl:ixr6,2:ceos
```

Note, that the default kind is `srl`, so you can omit the kind for SR Linux node. The same nodes value can be expressed like that: `4:ixr6,2:ceos`

#### kind

With `--kind` flag it is possible to set the default kind that will be set for the nodes which do not have a kind specified in the `--nodes` flag.

For example the following value will generate a 3-tier CLOS fabric of cEOS nodes:

```bash
# cEOS fabric
containerlab gen -n 3tier --kind ceos --nodes 4,2,1

# since SR Linux kind is assumed by default
# SRL fabric command is even shorter
containerlab gen -n 3tier --nodes 4,2,1
```

#### image
Use `--image` flag to specify the container image that should be used by a given kind.

The value of this flag follows the `kind=image` pattern. For example, to set the container image for the `ceos` kind: `--image ceos=4.21.F`.

To set images for multiple kinds repeat the flag: `--image srl=srlinux:latest --image ceos=4.21.F`

If the kind information is not provided in the `image` flag, the kind value will be taken from the `--kind` flag.

#### license
With `--license` flag it is possible to set the license path that should be used by a given kind.

The value of this flag follows the `kind=path` pattern. For example, to set the license path for the `srl` kind: `--license srl=/tmp/license.key`.

#### deploy
When `--deploy` flag is present, the lab deployment process starts using the generated topology definition file.

The generated definition file is first saved by the path set with `--flag` or, if file path is not set, by the default path of `<lab-name>.yml`. Then the equivalent of the `deploy -t <file> --regenerate` command is executed.

#### file
With `--file` flag its possible to save the generated topology definition in a file by a given path.

#### node-prefix
With `--node-prefix` flag a user sets the name prefix of every node in a lab.

Nodes will be named by the following template: `<node-prefix>-<tier>-<node-number>`. So a node named `node1-3` means this is the third node in a first tier of a topology.

Default prefix: `node`.

#### group-prefix
With `--group-prefix` it is possible to change the Group value of a node. Group information is used in the topology graph rendering.

#### network
With `--network` flag a user sets the name of the management network that will be created by container orchestration system such as docker.

Default: `clab`.

#### ipv4-subnet | ipv6-subnet
With `--ipv4-subnet` and `ipv6-subnet` its possible to change the address ranges of the management network. Nodes will receive IP addresses from these ranges if they are configured with DHCP.

### Examples

```bash
# generate and deploy a lab topology for 3-tier CLOS network
# with 8 leafs, 4 spines and 2 superspines
# all using Nokia SR Linux nodes with license and image provided
containerlab generate -n 3tier --image srl=srlinux:latest \
                      --license srl=license.key \
                      --nodes 8,4,2 --deploy
```