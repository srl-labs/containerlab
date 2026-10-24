---
search:
  boost: 4
kind_code_name: unmanaged-switch
kind_display_name: Unmanaged switch
---

# -{{ kind_display_name }}-

-{{ kind_display_name }}- is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).

The `-{{ kind_code_name }}-` kind connects lab nodes to a shared layer 2 segment. The switch is created and removed together with the lab, so there is nothing to set up on the host.

## Using unmanaged-switch kind

```yaml
name: switch01

topology:
  nodes:
    sw1:
      kind: -{{ kind_code_name }}-
    client1:
      kind: linux
      image: alpine:3
    client2:
      kind: linux
      image: alpine:3

  links:
    - endpoints: ["client1:eth1", "sw1:eth1"]
    - endpoints: ["client2:eth1", "sw1:eth2"]
```

Any valid interface name except `eth0` can be used for the switch ports.

## How it works

A switch node is a container that holds a network namespace with a Linux bridge named `br0`. All switch ports are attached to the bridge, so the switch learns MAC addresses and floods broadcast, multicast and unknown unicast frames. Spanning tree and multicast snooping are disabled, and VLAN-tagged frames are forwarded unchanged.

The switch has no management interface, but it runs an Alpine container that ships `brctl` for inspecting the switch:

```bash
clab exec -t switch01.clab.yml --label clab-node-name=sw1 --cmd "brctl showmacs br0"
```
