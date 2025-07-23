---
search:
  boost: 4
---
<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

# Linux bridge

Containerlab can connect its nodes to a Linux bridge instead of interconnecting the nodes directly. This connectivity option is enabled with `bridge` kind and opens a variety of integrations that containerlab labs can have with workloads of other types.

For example, by connecting a lab node to a bridge we can:

1. allow a node to talk to any workload (VM, container, baremetal) which are connected to that bridge
2. let a node to reach networks which are available via that bridge
3. scale out containerlab labs by running separate labs in different hosts and get network reachability between them
4. wiring nodes' data interfaces via a broadcast domain (linux bridge) and use vlans to making dynamic connections

-{{diagram(url='srl-labs/containerlab/diagrams/containerlab.drawio', page='8', title='Using bridges')}}-

## Using bridge kind

Containerlab doesn't create bridges on users behalf, that means that in order to use a bridge in the [topology definition file](../topo-def-file.md), the bridge needs to be created and enabled first.

Once the bridge is created, it needs to be referenced as a node inside the topology file:

```yaml
# topology documentation: http://containerlab.dev/lab-examples/ext-bridge/
name: br01

topology:
  kinds:
    nokia_srlinux:
      type: ixr-d2l
      image: ghcr.io/nokia/srlinux
  nodes:
    srl1:
      kind: nokia_srlinux
    srl2:
      kind: nokia_srlinux
    srl3:
      kind: nokia_srlinux
    # note, that the bridge br-clab must be created manually
    br-clab:
      kind: bridge

  links:
    - endpoints: ["srl1:e1-1", "br-clab:eth1"]
    - endpoints: ["srl2:e1-1", "br-clab:eth2"]
    - endpoints: ["srl3:e1-1", "br-clab:eth3"]
```

In the example above, node `br-clab` of kind `bridge` tells containerlab to identify it as a linux bridge and look for a bridge named `br-clab`.

When connecting other nodes to a bridge, the bridge endpoint must be present in the `links` section.

/// admonition
    type: subtle-note
When choosing names of the interfaces that need to be connected to the bridge make sure that these names are not clashing with existing interfaces.  
In the example above we named interfaces `eth1`, `eth2`, `eth3` accordingly and ensured that none of these interfaces existed before in the root netns.
///

As a result of such topology definition, you will see bridge `br-clab` with three interfaces attached to it:

```
bridge name     bridge id               STP enabled     interfaces
br-clab         8000.6281eb7133d2       no              eth1
                                                        eth2
                                                        eth3
```

Containerlab automatically adds iptables rules for the referenced bridges (v4 and v6) to allow traffic ingressing/egressing to/from the bridges. Namely, for a given bridge named `br-clab` containerlab will attempt to create the allowing rule in the filter table, FORWARD chain like this:

```
iptables -I FORWARD -i br-clab -j ACCEPT
iptables -I FORWARD -o br-clab -j ACCEPT
```

This will ensure that traffic is forwarded when passing this particular bridge.

/// warning
Once you destroy the lab, the rules in the FORWARD chain will stay, if you wish to remove it, you will have to do it manually. For example the with the following script (for v4 family):

```
sudo iptables -vL FORWARD --line-numbers -n | \
grep "set by containerlab" | awk '{print $1}' \
| sort -r | xargs -I {} sudo iptables -D FORWARD {}
```

///

Check out ["External bridge"](../../lab-examples/ext-bridge.md) lab for a ready-made example on how to use bridges.

## Bridges in container namespace

It is possible to make Containerlab create bridges inside the container namespace and connect nodes to them. As opposed to the host-bound bridges, these bridges reside in a container namespace and therefore are isolated from the host.

A practical use case for this is to create backplane bridges that are used for internal connectivity between nodes in a lab and should not be part of the host namespace. To defined a namespaced bridge, you need to

1. use a namespace of another node using the `network-mode` field
2. use a special naming convention for the namespaced bridge, which includes the parent node name in the bridge after the `|` character. The bridge nodes name must be `<bridge-name>|<node-name>` whilst `<node-name>` must match the `network-mode: container:<node-name>`.

```yaml
name: "bridge-ns"
topology:
  nodes:
     br01|bp1:
       kind: bridge
       network-mode: container:bp1
     bp1:
       kind: linux
       image: alpine:latest
     c1:
       kind: linux
       image: alpine:latest
   links:
     - endpoints: ["c1:eth1", "br01|bp1:c1eth1"]
```

In the example above, the bridge `br01` is created inside the container namespace of the `bp1` node. The bridge will be named `br01` inside the `bp1` container and will have an interface `c1eth1` connected to it from the `c1` node.

The extra `|<parent node>` suffix is used to distinguish the bridges and make them unique for containerlab, but this suffix will be dropped when the bridge is created inside the container namespace, so the bridge will still be named `br01` inside the `bp1` and `bp2` containers.
