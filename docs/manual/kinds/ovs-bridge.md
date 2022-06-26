---
search:
  boost: 4
---
# Openvswitch bridge
Similar to [linux bridge](bridge.md) capability, containerlab allows to connect nodes to an Openvswitch (Ovs) bridge. Ovs bridges offers even more connectivity options compared to classic Linux bridge, as well as it allows to create stretched L2 domain by means of tunneled interfaces (vxlan).

## Using ovs-bridge kind
Containerlab doesn't create bridges on users behalf, that means that in order to use a bridge in the [topology definition file](../topo-def-file.md), the Ovs bridge needs to be created first.

Once the bridge is created, it has to be referenced as a node inside the topology file:

```yaml
name: ovs

topology:
  nodes:
    myovs:
      kind: ovs-bridge
    ceos:
      kind: ceos
      image: ceos:latest
  links:
    - endpoints: ["myovs:ovsp1", "srl:eth1"]

```

In the example above, node `myovs` of kind `ovs-bridge` tells containerlab to identify it as a Ovs bridge and look for a bridge named `myovs`.

When connecting other nodes to a bridge, the bridge endpoint must be present in the `links` section.

!!!note
    When choosing names of the interfaces that need to be connected to the bridge make sure that these names are not clashing with existing interfaces.  
    In the example above we attach a single interfaces named `ovsp1` to the Ovs bridge with a name `myovs`. Before that, we ensured that no other interfaces named `ovsp1` existed.

As a result of such topology definition, you will see the Ovs bridge `br-clab` with three interfaces attached to it:

```
❯ ovs-vsctl show
918a3466-4368-4167-9162-f2cf80a0c106
    Bridge myovs
        Port myovs
            Interface myovs
                type: internal
        Port eth1
            Interface ovsp1
    ovs_version: "2.13.1"
```
