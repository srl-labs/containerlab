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

With the global `--name` flag a user sets the name of the lab that will be generated.

#### nodes

The user configures the CLOS fabric topology by using the `--nodes` flag. The flag value is a comma separated list of CLOS tiers where each tier is defined by the number of nodes, its kind and type. Multiple `--node` flags can be specified.

-{{diagram(url='srl-labs/containerlab/diagrams/containerlab.drawio', page=12, title='')}}-

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

For example, the following flag value will define a 2-tier CLOS fabric with tier1 (leafs) consists of 4x SR Linux containers of IXR-D3 type and the 2x Arista cEOS spines:

```
4:srl:ixrd3,2:ceos
```

Note, that the default kind is `srl`, so you can omit the kind for SR Linux node. The same nodes value can be expressed like that: `4:ixrd3,2:ceos`

#### kind

With `--kind` flag it is possible to set the default kind that will be set for the nodes which do not have a kind specified in the `--nodes` flag.

For example the following value will generate a 3-tier CLOS fabric of cEOS nodes:

```bash
# cEOS fabric
containerlab gen --name 3tier --kind ceos --nodes 4,2,1

# since SR Linux kind is assumed by default
# SRL fabric command is even shorter
containerlab gen --name 3tier --nodes 4,2,1
```

#### image

Use `--image` flag to specify the container image that should be used by a given kind.

The value of this flag follows the `kind=image` pattern. For example, to set the container image `ceos:4.32.0F` for the `ceos` kind the flag will be: `--image ceos=ceos:4.32.0F`.

To set images for multiple kinds repeat the flag: `--image srl=ghcr.io/nokia/srlinux:latest --image ceos=ceos:4.32.0F` or use the comma separated form: `--image srl=ghcr.io/nokia/srlinux:latest,ceos=ceos:latest`

If the kind information is not provided in the `image` flag, the kind value will be taken from the `--kind` flag.

#### license

With `--license` flag it is possible to set the license path that should be used by a given kind.

The value of this flag follows the `kind=path` pattern. For example, to set the license path for the `srl` kind: `--license srl=/tmp/license.key`.

To set license for multiple kinds repeat the flag: `--license <kind1>=/path1 --image <kind2>=/path2` or use the comma separated form: `--license <kind1>=/path1,<kind2>=/path2`

#### deploy

When `--deploy` flag is present, the lab deployment process starts using the generated topology definition file.

The generated definition file is first saved by the path set with `--file` or, if file path is not set, by the default path of `<lab-name>.clab.yml`. Then the equivalent of the `deploy -t <file> --reconfigure` command is executed.

#### max-workers

With `--max-workers` flag it is possible to limit the amout of concurrent workers that create containers or wire virtual links. By default the number of workers equals the number of nodes/links to create.

If during the deployment of a large scaled lab you see errors about max number of opened files reached, limit the max workers with this flag.

#### file

With `--file` flag it's possible to save the generated topology definition in a file by a given path.

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

With `--ipv4-subnet` and `ipv6-subnet` it's possible to change the address ranges of the management network. Nodes will receive IP addresses from these ranges if they are configured with DHCP.

#### owner

With `--owner` flag you can specify a custom owner for the lab. This value will be applied as the owner label for all nodes in the lab.

This flag is designed for multi-user environments where you need to track ownership of lab resources. Only users who are members of the `clab_admins` group can set a custom owner. If a non-admin user attempts to set an owner, the flag will be ignored with a warning, and the current user will be used as the owner instead.

Example:

```bash
containerlab generate --name 3tier --nodes 8,4,2 --owner bob --deploy
```

### Examples

#### Generate topology for a 3-tier CLOS network

Generate and deploy a lab topology for 3-tier CLOS network with 8 leafs, 4 spines and 2 superspines. All using Nokia SR Linux nodes with license and image provided.

/// note
The `srl` kind in the image and license flags can be omitted, as it is implied by default
///

```bash
containerlab generate --name 3tier --image srl=ghcr.io/nokia/srlinux:latest \
                      --nodes 8,4,2 --deploy
```
