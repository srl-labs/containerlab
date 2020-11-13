Containerlab builds labs based on the topology information that users pass to it. This topology information is expressed as a code contained in the _topology definition file_ which structure is the prime focus of this document.


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:4,&quot;zoom&quot;:1,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/containerlab-diagrams/main/containerlab.drawio&quot;}"></div>

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fsrl-wim%2Fcontainerlab-diagrams%2Fmain%2Fcontainerlab.drawio" async></script>

## Topology definition components
The topology definition file is a configuration file expressed in YAML. In this document we will take a pre-packaged [Nokia SR Linux and Arista cEOS](../lab-examples/srl-ceos.md) lab and explain the topology definition structure using its definition file [srlceos01.yml](https://github.com/srl-wim/container-lab/tree/master/lab-examples/srlceos01/srlceos01.yml) which is pasted below:

```yaml
name: srlceos01

topology:
  nodes:
    srl:
      kind: srl
      type: ixrd2
      image: srlinux
      license: license.key
    ceos:
      kind: ceos
      image: ceos

  links:
    - endpoints: ["srl:e1-1", "ceos:eth1"]
```

### Name
The topology must have a name associated with it. The name is used to distinct one topology from another, to allow multiple topologies to be deployed on the host without their names clashed.

```yaml
name: srlceos01
```

Its user's responsibility to give labs unique names if they plan to run multiple labs.

The name is a free-formed string, though its recommended not to use dashes (`-`) as they are used to separate lab names from node names.

When containerlab starts the containers, their names will be generated using the following pattern: `clab-{{lab_name}}-{{node_name}}`. The lab name here is used to make the container's names unique between two different labs even if the nodes are named the same.

### Topology
The topology object inside the topology definition is the core element of the file. Under the `topology` element you will find all the core objects such as `nodes`, `links`, `kinds` and `defaults`.

#### Nodes
As with every other topology the nodes are in the center of things. With nodes we tell which lab elements we want to run, in what configuration and flavor.

Let's zoom into the two nodes we have defined in our topology:

```yaml
topology:
  nodes:
    srl:                    # this is a name of the 1st node
      kind: srl
      type: ixrd2
      image: srlinux
      license: license.key
    ceos:                   # this is a name of the 2nd node
      kind: ceos
      image: ceos
```

We defined individual `nodes` under the `topology.nodes` container. The name of the node is the key under which it is defined. Following the example, our two nodes will be named `srl` and `ceos`.

Each node can be defined with a set of properties. Such as the `srl` node is defined with the following properties:

```yaml
srl:
  kind: srl
  type: ixrd2
  image: srlinux
  license: license.key
```

The `kind` property selects which kind this node is of. Kinds are essentially a way of telling containerlab how to treat the nodes properties considering the specific flavor of the node. We dedicated a [separate section](kinds.md) to discuss kinds in details.

With `type` the user can sets a type of the node. Types work in combination with the kinds, such as the type value of `ixrd2` sets the chassis type for SR Linux node, thus this value only makes sense to nodes of kind `srl`.

The common `image` attribute sets the container image name that will be used to start the node. The image name should be provided in a regular format of `repository(:tag)`.

Some containerized NOSes require a license to operate. With `license` property a user sets a path to a license file that a node will use. The license file will then be mounted to the container by the path that is defined by the `kind/type` of the node.

#### Links
Although its totally fine to define the node without any links (like in [this lab](../lab-examples/single-srl.md)) most of the time we interconnect the nodes with links. One of containerlab purposes is to make the interconnection of nodes simple.

We define the links under the `topology.links` container in the following manner:

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

As you see, the `topology.links` container consists of links. The link itself is expressed as list of two `endpoints`. This might sound complicated, lets use a graphical explanation:

<center><div class="mxgraph" style="max-width:100%;border:1px solid transparent;" data-mxgraph="{&quot;page&quot;:11,&quot;zoom&quot;:1,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/containerlab-diagrams/main/containerlab.drawio&quot;}"></div></center>

As demonstrated on a diagram above, the links between the containers are the point-to-point links which are defined by a pair of interfaces. The link defined as:

```yaml
endpoints: ["srl:e1-1", "ceos:eth1"]
```

translates to an intent of creation a p2p link between the node named `srl` and its `e1-1` interface and the node named `ceos` and its `eth1` interface. The p2p link is realized with a veth pair.

#### Kinds
Kinds define the flavor of the node, it says if the node is a specific containerized Network OS or something else. We go into details of kinds in its own [document section](kinds.md), but for the sake of the topology container, we must discuss what happens when `kinds` section appears in the topology definition:


```yaml
topology:
  kinds:
    srl:
      type: ixrd2
      image: srlinux
      license: license.key
  nodes:
    srl1:
      kind: srl
    srl2:
      kind: srl
    srl3:
      kind: srl
```

In the example above the `topology.kinds` container has the `srl` kind referenced. With this, we set some values for the properties of the `srl` kind. With a configuration like that, we say that nodes that have `srl` kind associated will also inherit its properties (type, image, license).

Essentially, what `kinds` section allows us to do is to shorten the lab definition in cases when we have a number of nodes of a same kind. All the nodes (`srl1`, `srl2`, `srl3`) will have the same values for `type`, `image` and `license`.

Consider how the topology would have looked like without setting the `kinds` object:

```yaml
topology:
  nodes:
    srl1:
      kind: srl
      type: ixrd2
      image: srlinux
      license: license.key
    srl2:
      kind: srl
      type: ixrd2
      image: srlinux
      license: license.key
    srl3:
      kind: srl
      type: ixrd2
      image: srlinux
      license: license.key
```

A lot of unnecessary repetition which is eliminated when setting `kinds`.

#### Defaults
Since `kinds` set the values for the properties of a specific kind, we also introduced the `defaults` container, that can set values globally.

With `defaults` you can, for example, set the default kind for all the nodes like that:

```yaml
topology:
  defaults:
    kind: srl
  kinds:
    srl:
      type: ixrd2
      image: srlinux
      license: license.key
  nodes:
    srl1:
    srl2:
    srl3:
```

Now every node without a kind specified under it, will inherit the global default value of `srl`.