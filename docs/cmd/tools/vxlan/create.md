# vxlan create

## Description

The `create` sub-command under the `tools vxlan` command creates a VxLAN interface and sets `tc` rules to redirect traffic to/from a specified interface available in root namespace of a container host.

This combination of a VxLAN interface and `tc` rules make possible to transparently connect lab nodes running on different VMs/hosts.

VxLAN interface name will be a catenation of a prefix `vx-` and the interface name that is used to redirect traffic. If the existing interface is named `srl_e1-1`, then VxLAN interface created for this interface will be named `vx-srl_e1-1`.

## Usage

`containerlab tools vxlan create [local-flags]`

## Flags

### remote

VxLAN tunnels set up with this command are unidirectional in nature. To set the remote endpoint address the `--remote` flag should be used.

### port

Port number that the VxLAN tunnel will use is set with `--port | -p` flag. Defaults to `14789`[^1].

### id

VNI that the VxLAN tunnel will use is set with `--id | -i` flag. Defaults to `10`.

### link

As mentioned above, the tunnels are set up with a goal to transparently connect containers deployed on different hosts.

To indicate which interface will be "piped" to a VxLAN tunnel the `--link | -l` flag should be used.

### dev

With `--dev` flag users can set the linux device that should be used in setting up the tunnel.

Normally this flag can be omitted, since containerlab will take the device name which is used to reach the remote address as seen by the kernel routing table.

### mtu

With `--mtu | -m` flag it is possible to set VxLAN MTU. Max MTU is automatically set, so this flag is only needed when MTU lower than max is needed to be provisioned.

## Examples

```bash
# create vxlan tunnel and redirect traffic to/from existing interface srl_e1-1 to it
# this effectively means anything that appears on srl_e1-1 interface will be piped to vxlan interface
# and vice versa.

# srl_e1-1 interface exists in root namespace
❯ ip l show srl_e1-1
617: srl_e1-1@if618: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default 
    link/ether fa:4c:16:11:11:05 brd ff:ff:ff:ff:ff:ff link-netns clab-vx-srl1

# create a vxlan tunnel to a remote vtep 10.0.0.20 with VNI 10 and redirect traffic to srl_e1-1 interface
❯ clab tools vxlan create --remote 10.0.0.20 -l srl_e1-1 --id 10
INFO[0000] Adding VxLAN link vx-srl_e1-1 under ens3 to remote address 10.0.0.20 with VNI 10
INFO[0000] configuring ingress mirroring with tc in the direction of vx-srl_e1-1 -> srl_e1-1
INFO[0000] configuring ingress mirroring with tc in the direction of srl_e1-1 -> vx-srl_e1-1

# check the created interface
❯ ip l show vx-srl_e1-1
619: vx-srl_e1-1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    link/ether 7a:6e:ba:82:a4:6f brd ff:ff:ff:ff:ff:ff
```

[^1]: The reason we don't use default `4789` port number is because it is often blocked/filtered in (cloud) environments and we want to make sure that the VxLAN tunnels have higher chances to be established.
