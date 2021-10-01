# vEth create
### Description

The `create` sub-command under the `tools veth` command creates a vEth interface between the following combination of nodes:

1. container <-> container
2. container <-> linux bridge
3. container <-> ovs bridge
4. container <-> host

To specify the both endpoints of the veth interface pair the following two notations are used:

1. two elements notation: `<node-name>:<interface-name>`
    this notation is used for `container <-> container` or `container <-> host` attachments.
2. three elements notation: `<kind>:<node-name>:<interface-name>`
    this notation is used for `container <-> bridge` and `container <-> ovs-bridge` attachments

Check out [examples](#examples) to see how these notations are used.

### Usage

`containerlab tools veth create [local-flags]`

### Flags

#### a-endpoint
vEth interface endpoint A is set with `--a-endpoint | -a` flag.

#### b-endpoint
vEth interface endpoint B is set with `--b-endpoint | -b` flag.

#### mtu
vEth interface MTU is set to `9500` by default, and can be changed with `--mtu | -m` flag.

### Examples

```bash
# create veth interface between containers clab-demo-node1 and clab-demo-node2
# both ends of veth pair will be named `eth1`
containerlab tools veth create -a clab-demo-node1:eth1 -b clab-demo-node2:eth1

# create veth interface between container clab-demo-node1 and linux bridge br-1
containerlab tools veth create -a clab-demo-node1:eth1 -b bridge:br-1:br-eth1

# create veth interface between container clab-demo-node1 and OVS bridge ovsbr-1
containerlab tools veth create -a clab-demo-node1:eth1 -b ovs-bridge:ovsbr-1:br-eth1

# create veth interface between container clab-demo-node1 and host
# note that a special node-name `host` is reserved to indicate that attachment is destined for container host system
containerlab tools veth create -a clab-demo-node1:eth1 -b host:veth-eth1
```