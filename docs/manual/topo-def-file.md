Containerlab builds labs based on the topology information that users pass to it. This topology information is expressed as a code contained in the _topology definition file_ which structure is the prime focus of this document.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:4,&quot;zoom&quot;:1,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

## Topology definition components

The topology definition file is a configuration file expressed in YAML and has a name pattern of `*.clab.yml`[^1]. In this document, we take a pre-packaged [Nokia SR Linux and Arista cEOS](../lab-examples/srl-ceos.md) lab and explain the topology definition structure using its definition file [srlceos01.clab.yml](https://github.com/srl-labs/containerlab/tree/main/lab-examples/srlceos01/srlceos01.clab.yml) which is pasted below:

```yaml
name: srlceos01

topology:
  nodes:
    srl:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux
    ceos:
      kind: ceos
      image: ceos:4.25.0F

  links:
    - endpoints: ["srl:e1-1", "ceos:eth1"]
```

///note
Containerlab provides a [JSON schema file](https://github.com/srl-labs/containerlab/blob/main/schemas/clab.schema.json) for the topology file. The schema is used to live-validate user's input if a code editor supports this feature.

<!-- Additionally, the [auto-generated schema documentation](https://json-schema.app/view/%23?url=https%3A%2F%2Fraw.githubusercontent.com%2Fsrl-labs%2Fcontainerlab%2Fmain%2Fschemas%2Fclab.schema.json) can be explored to understand the full scope of the configuration options containerlab provides. -->
///

This topology results in the two nodes being started up and interconnected with each other using a single point-to-point interface:
<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlceos01.drawio&quot;}"></div>

Let's touch on the key components of the topology definition file used in this example.

### Name

The topology must have a name associated with it. The name is used to distinct one topology from another, to allow multiple topologies to be deployed on the same host without clashes.

```yaml
name: srlceos01
```

Its user's responsibility to give labs unique names if they plan to run multiple labs.

The name is a free-formed string, though it is better not to use dashes (`-`) as they are used to separate lab names from node names.

When containerlab starts the containers, their names will be generated using the following pattern: `clab-{{lab-name}}-{{node-name}}`. The lab name here is used to make the container's names unique between two different labs, even if the nodes are named the same.

### Prefix

It is possible to change the prefix that containerlab adds to node names. The `prefix` parameter is in charge of that. It follows the below-mentioned logic:

1. When `prefix` is not present in the topology file, the default prefix logic applies. Containers will be named as `clab-<lab-name>-<node-name>`.
1. When `prefix` is set to some value, for example, `myprefix`, this string is used instead of `clab`, and the resulting container name will be: `myprefix-<lab-name>-<node-name>`.
1. When `prefix` is set to a magic value `__lab-name` the resulting container name will not have the `clab` prefix, but will keep the lab name: `<lab-name>-<node-name>`.
1. When set to an empty string, the node names will not be prefixed at all. If your node is named `mynode`, you will get the `mynode` container in your system.

!!!warning
    In the case of an empty prefix, you have to keep in mind that nodes need to be named uniquely across all labs.

Examples:
=== "custom prefix"
    ```yaml
    name: mylab
    prefix: myprefix
    nodes:
      n1:
      # <some config>
    ```

    With a prefix set to `myprefix` the container name for node `n1` will be `myprefix-mylab-n1`.
=== "empty prefix"
    ```yaml
    name: mylab
    prefix: ""
    nodes:
      n1:
      # <some config>
    ```

    When a prefix is set to an empty string, the container name will match the node name - `n1`.

!!!note
    Even when you change the prefix, the lab directory is still uniformly named using the `clab-<lab-name>` pattern.

### Topology

The topology object inside the topology definition is the core element of the file. Under the `topology` element you will find all the main building blocks of a topology such as `nodes`, `kinds`, `defaults` and `links`.

#### Nodes

As with every other topology the nodes are in the center of things. With nodes we define which lab elements we want to run, in what configuration and flavor.

Let's zoom into the two nodes we have defined in our topology:

```yaml
topology:
  nodes:
    srl:                    # this is a name of the 1st node
      kind: nokia_srlinux
      type: ixrd2
      image: ghcr.io/nokia/srlinux
    ceos:                   # this is a name of the 2nd node
      kind: ceos
      image: ceos:4.25.0F
```

We defined individual nodes under the `topology.nodes` container. The name of the node is the key under which it is defined. Following the example, our two nodes are named `srl` and `ceos` respectively.

Each node can have multiple configuration properties which make containerlab quite a flexible tool. The `srl` node in our example is defined with the a few node-specific properties:

```yaml
srl:
  kind: nokia_srlinux
  type: ixrd2
  image: ghcr.io/nokia/srlinux
```

Refer to the [node configuration](nodes.md) document to meet all other options a node can have.

#### Links

Although it is absolutely fine to define a node without any links (like in [this lab](../lab-examples/single-srl.md)), we usually interconnect the nodes to make topologies. One of containerlab purposes is to make the interconnection of the nodes simple.

Links are defined under the `topology.links` section of the topology file. Containerlab understands two formats of link definition - brief and extended.  
A brief form of a link definition compresses link parameters in a single string and provide a quick way to define a link at the cost of link features available.  
A more expressive extended form exposes all link features, but requires more typing if done manually. The extended format is perfect for machine-generated link topologies.

##### Brief format

The brief version looks as follows.

```yaml
# nodes configuration omitted for clarity
topology:
  nodes:
    srl:
    ceos:

  links:
    - endpoints: ["srl:e1-1", "ceos:eth1"]
    - endpoints: ["srl:e1-2", "ceos:eth2"]
```

As you see, the `topology.links` element is a list of individual links. The link itself is expressed as pair of `endpoints`. This might sound complicated, lets use a graphical explanation:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:11,&quot;zoom&quot;:2,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

As demonstrated on a diagram above, the links between the containers are the point-to-point links which are defined by a pair of interfaces. The link defined as:

```yaml
endpoints: ["srl:e1-1", "ceos:eth1"]
```

will result in a creation of a p2p link between the node named `srl` and its `e1-1` interface and the node named `ceos` and its `eth1` interface. The p2p link is realized with a veth pair.

##### Extended format

The extended link format allows a user to set every supported link parameter in a structured way. The available link parameters depend on the Link type and provided below.

###### veth

The veth link is the most common link type used in containerlab. It creates a virtual ethernet link between two endpoints where each endpoint refers to a node in the topology.

```yaml
links:
  - type: veth
    endpoints:
      - node: <NodeA-Name>                  # mandatory
        interface: <NodeA-Interface-Name>   # mandatory
        mac: <NodeA-Interface-Mac>          # optional
      - node: <NodeB-Name>                  # mandatory
        interface: <NodeB-Interface-Name>   # mandatory
        mac: <NodeB-Interface-Mac>          # optional
    mtu: <link-mtu>                         # optional
    vars: <link-variables>                  # optional (used in templating)
    labels: <link-labels>                   # optional (used in templating)
```

###### mgmt-net

The mgmt-net link type represents a veth pair that is connected to a container node on one side and to the management network (usually a bridge) instantiated by the container runtime on the other.

```yaml
  links:
  - type: mgmt-net
    endpoint:
      node: <NodeA-Name>                  # mandatory
      interface: <NodeA-Interface-Name>   # mandatory
      mac: <NodeA-Interface-Mac>          # optional
    host-interface: <interface-name         # mandatory
    mtu: <link-mtu>                         # optional
    vars: <link-variables>                  # optional (used in templating)
    labels: <link-labels>                   # optional (used in templating)
```

The `host-interface` is the desired interface name that will be attached to the management network in the host namespace.

###### macvlan

The macvlan link type creates a MACVlan interface with the `host-interface` as its parent interface. The MACVlan interface is then moved to a node's network namespace and renamed to the `endpoint.interface` name.

```yaml
  links:
  - type: macvlan
    endpoint:
      node: <NodeA-Name>                  # mandatory
      interface: <NodeA-Interface-Name>   # mandatory
      mac: <NodeA-Interface-Mac>          # optional
    host-interface: <interface-name>        # mandatory
    mode: <macvlan-mode>                    # optional ("bridge" by default)
    vars: <link-variables>                  # optional (used in templating)
    labels: <link-labels>                   # optional (used in templating)
```

The `host-interface` is the name of the existing interface present in the host namespace.

[Modes](https://man7.org/linux/man-pages/man8/ip-link.8.html) are `private`, `vepa`, `bridge`, `passthru` and `source`. The default is `bridge`.

###### host

The host link type creates a veth pair between a container and the host network namespace.  
In comparison to the veth type, no bridge or other namespace is required to be referenced in the link definition for a "remote" end of the veth pair.

```yaml
  links:
  - type: host
    endpoint:
      node: <NodeA-Name>                  # mandatory
      interface: <NodeA-Interface-Name>   # mandatory
      mac: <NodeA-Interface-Mac>          # optional
    host-interface: <interface-name>        # mandatory
    mtu: <link-mtu>                         # optional
    vars: <link-variables>                  # optional (used in templating)
    labels: <link-labels>                   # optional (used in templating)
```

The `host-interface` parameter defines the name of the veth interface in the host's network namespace.

###### vxlan

The vxlan type results in a vxlan tunnel interface that is created in the host namespace and subsequently pushed into the nodes network namespace.

```yaml
  links:
    - type: vxlan                       
      endpoint:                              # mandatory
        node: <Node-Name>                    # mandatory
        interface: <Node-Interface-Name>     # mandatory
        mac: <Node-Interface-Mac>            # optional
      remote: <Remote-VTEP-IP>               # mandatory
      vni: <VNI>                             # mandatory
      udp-port: <VTEP-UDP-Port>              # mandatory
      mtu: <link-mtu>                        # optional
      vars: <link-variables>                 # optional (used in templating)
      labels: <link-labels>                  # optional (used in templating)
```

###### vxlan-stitched

The vxlan-stitched type results in a veth pair linking the host namespace and the nodes namespace and a vxlan tunnel that also terminates in the host namespace.
In addition to these interfaces, tc rules are being provisioned to stitch the vxlan tunnel and the host based veth interface together.

```yaml
  links:
    - type: vxlan-stitch
      endpoint:                              # mandatory
        node: <Node-Name>                    # mandatory
        interface: <Node-Interface-Name>     # mandatory
        mac: <Node-Interface-Mac>            # optional
      remote: <Remote-VTEP-IP>               # mandatory
      vni: <VNI>                             # mandatory
      udp-port: <VTEP-UDP-Port>              # mandatory
      mtu: <link-mtu>                        # optional
      vars: <link-variables>                 # optional (used in templating)
      labels: <link-labels>                  # optional (used in templating)
```

#### Kinds

Kinds define the behavior and the nature of a node, it says if the node is a specific containerized Network OS, virtualized router or something else. We go into details of kinds in its own [document section](kinds/index.md), so here we will discuss what happens when `kinds` section appears in the topology definition:

```yaml
topology:
  kinds:
    nokia_srlinux:
      type: ixrd2
      image: ghcr.io/nokia/srlinux
  nodes:
    srl1:
      kind: nokia_srlinux
    srl2:
      kind: nokia_srlinux
    srl3:
      kind: nokia_srlinux
```

In the example above the `topology.kinds` element has `srl` kind referenced. With this, we set some values for the properties of the `srl` kind. A configuration like that says that nodes of `srl` kind will also inherit the properties (type, image) defined on the _kind level_.

Essentially, what `kinds` section allows us to do is to shorten the lab definition in cases when we have a number of nodes of a same kind. All the nodes (`srl1`, `srl2`, `srl3`) will have the same values for their `type` and `image` properties.

Consider how the topology would have looked like without setting the `kinds` object:

```yaml
topology:
  nodes:
    srl1:
      kind: nokia_srlinux
      type: ixrd2
      image: ghcr.io/nokia/srlinux
    srl2:
      kind: nokia_srlinux
      type: ixrd2
      image: ghcr.io/nokia/srlinux
    srl3:
      kind: nokia_srlinux
      type: ixrd2
      image: ghcr.io/nokia/srlinux
```

A lot of unnecessary repetition is eliminated when we set `srl` kind properties on kind level.

#### Defaults

`kinds` set the values for the properties of a specific kind, whereas with the `defaults` container it is possible to set values globally.

For example, to set the environment variable for all the nodes of a topology:

```yaml
topology:
  defaults:
    env:
      MYENV: VALUE
  nodes:
    srl1:
    srl2:
    srl3:
```

Now every node in this topology will have environment variable `MYENV` set to `VALUE`.

### Settings

Global containerlab settings are defined in `settings` container. The following settings are supported:

#### Certificate authority

Global certificate authority settings section allows users to tune certificate management in containerlab. Refer to the [Certificate management](cert.md) doc for more details.

## Environment variables

Topology definition file may contain environment variables anywhere in the file. The syntax is the same as in the bash shell:

```yaml
name: linux

topology:
  nodes:
    l1:
      kind: linux
      image: alpine:${ALPINE_VERSION:=3}
```

In the example above, the `ALPINE_VERSION` environment variable is used to set the version of the alpine image. If the variable is not set, the value of `3` will be used. The following syntax is used to expand the environment variable:

| **Expression**     | **Meaning**                                                          |
| ------------------ | -------------------------------------------------------------------- |
| `${var}`           | Value of var (same as `$var`)                                        |
| `${var-$DEFAULT}`  | If var not set, evaluate expression as $DEFAULT                      |
| `${var:-$DEFAULT}` | If var not set or is empty, evaluate expression as $DEFAULT          |
| `${var=$DEFAULT}`  | If var not set, evaluate expression as $DEFAULT                      |
| `${var:=$DEFAULT}` | If var not set or is empty, evaluate expression as $DEFAULT          |
| `${var+$OTHER}`    | If var set, evaluate expression as $OTHER, otherwise as empty string |
| `${var:+$OTHER}`   | If var set, evaluate expression as $OTHER, otherwise as empty string |
| `$$var`            | Escape expressions. Result will be `$var`.                           |

## Generated topologies

To further simplify parametrization of the topology files, containerlab allows users to template the topology files using Go Template engine.

Using templating approach it is possible to create a lab template and instantiate different labs from it, by simply changing the variables in the variables file.

Standard Go templating language has been extended with the functions provided in [docs.gomplate.ca](https://docs.gomplate.ca/) project, which opens the doors to a very flexible topology generation workflows.

To help you get started, we created the following lab examples which demonstrate how topology templating can be used:

- [Leaf-Spine topology with parametrized number of leaves/spines](lab-examples/../../lab-examples/templated01.md)
- [5-stage Clos topology with parametrized number of pods and super-spines](lab-examples/../../lab-examples/templated02.md)

[^1]: if the filename has `.clab.yml` or `-clab.yml` suffix, the YAML file will have autocompletion and linting support in VSCode editor.
