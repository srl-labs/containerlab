<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>
# Linux bridge
Containerlab can connect its nodes to a Linux bridge instead of interconnecting the nodes directly. This connectivity option is enabled with `bridge` kind and opens a variety of integrations that containerlab labs can have with workloads of other types.

For example, by connecting a lab node to a bridge we can:

1. allow a node to talk to any workloads (VMs, containers, baremetals) which are connected to that bridge
2. let a node to reach networks which are available via that bridge
3. scale out containerlab labs by running separate labs in different hosts and get network reachibility between them
4. wiring nodes' data interfaces via a broadcast domain (linux bridge) and use vlans to making dynamic connections


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:8,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

## Using bridge kind
Containerlab doesn't create bridges on users behalf, that means that in order to use a bridge in the [topology definition file](../topo-def-file.md), the bridge needs to be created first.

Once the bridge is created, it needs to be referenced as a node inside the topology file:

```yaml
# topology documentation: http://containerlab.srlinux.dev/lab-examples/ext-bridge/
name: br01

topology:
  kinds:
    srl:
      type: ixrd2
      image: srlinux:21.3.1-410
  nodes:
    srl1:
      kind: srl
    srl2:
      kind: srl
    srl3:
      kind: srl
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

!!!note
    When choosing names of the interfaces that need to be connected to the bridge make sure that these names are not clashing with existing interfaces.  
    In the example above we named interfaces `eth1`, `eth2`, `eth3` accordingly and ensured that none of these interfaces existed before in the root netns.  

As a result of such topology definition, you will see bridge `br-clab` with three interfaces attached to it:

```
bridge name     bridge id               STP enabled     interfaces
br-clab         8000.6281eb7133d2       no              eth1
                                                        eth2
                                                        eth3
```

Chec out ["External bridge"](../../lab-examples/ext-bridge.md) lab for a ready-made example on how to use bridges.