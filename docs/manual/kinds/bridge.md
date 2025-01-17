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
      type: ixrd2l
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
