---
search:
  boost: 4
kind_code_name: cisco_xrd_vrouter
kind_display_name: Cisco XRd vRouter
---
# Cisco XRd vRouter

[Cisco IOS XRd](https://www.cisco.com/site/us/en/products/networking/sdwan-routers/ios-xrd/index.html) vRouter is the high-performance dataplane (DPDK) form factor of the containerized IOS XR. It is identified with the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md) and is built using the [vrnetlab](../vrnetlab.md) project.

Unlike the XRd Control Plane (the [`cisco_xrd`](xrd.md) kind), XRd vRouter's dataplane interfaces are PCI-only and cannot bind a veth directly. The vrnetlab integration therefore runs the XRd vRouter container inside a small KVM guest (a micro-VM) that presents emulated PCI NICs, so it can be wired with ordinary containerlab links like any other VM-based node.

To build the containerlab-compatible container image from the XRd vRouter container release refer to the [srl-labs/vrnetlab/cisco/xrd-vrouter](https://github.com/srl-labs/vrnetlab/tree/master/cisco/xrd-vrouter) documentation.

XRd vRouter nodes launched with containerlab come up pre-provisioned with SSH, NETCONF and gNMI services enabled.

/// admonition | Resource requirements
    type: warning
XRd vRouter's DPDK dataplane busy-polls a CPU core and uses hugepages. Per node it needs at least 4 cores, ~5 GiB RAM, and 3 GiB of 1 GiB hugepages; containerlab allocates 4 vCPU / 10 GiB by default and floors vCPU at 4 and RAM at 8 GiB (at 8 GiB an idle node booted and forwarded with ~800 MB to spare). Drop RAM toward the 8 GiB floor only to pack more nodes onto a host. The build and run host must have nested virtualization (`/dev/kvm`) and hugepages available.

Tune the allocated resources with the `VCPU` and `RAM` environment variables:

```yaml
    xrd:
      kind: cisco_xrd_vrouter
      image: vrnetlab/cisco_xrd-vrouter:25.4.2
      env:
        VCPU: 4
        RAM: 10240   # 10 GiB (the default); lower toward the 8192 floor to pack more nodes
```

///

## Managing Cisco XRd vRouter nodes

A Cisco XRd vRouter node launched with containerlab can be managed via the following interfaces:

/// tab | bash
to connect to a `bash` shell of the node's container (the VM host running the XRd vRouter — not the XR CLI):

```bash
docker exec -it <container-name/id> bash
```
///

/// tab | CLI via SSH
to connect to the XR CLI:

```bash
ssh clab@<container-name/id>
```
///

/// tab | NETCONF
NETCONF server is running over port 830:

```bash
ssh clab@<container-name> -p 830 -s netconf
```
///

/// tab | gNMI
using the [gnmic](https://gnmic.openconfig.net/) gNMI client as an example:

```bash
gnmic -a <container-name/node-mgmt-address>:9339 --insecure \
-u clab -p clab@123 \
capabilities
```
///

/// note
Default user credentials: `clab:clab@123`
///

/// admonition | Reaching the XR CLI
    type: note
XRd vRouter runs inside the node's micro-VM, so — unlike the control-plane [`cisco_xrd`](xrd.md) kind — there is no `docker exec … xr_cli` shortcut. Reach the XR CLI over SSH (the **CLI via SSH** tab above). First boot takes a few minutes — XRd typically reaches `healthy` (shown by `docker ps`) in ~3–4 min; follow the boot log with `docker logs -f <node>`.
///

## Interface naming

Data interfaces are emulated **vmxnet3** NICs by default, which XRd presents as `TenGigE0/0/0/X` (10 GbE) where `X` starts at `0`:

* `eth1` → `TenGigE0/0/0/0`
* `eth2` → `TenGigE0/0/0/1`
* and so on.

Set `XRD_NIC_TYPE: igb` in the node's `env` to use emulated `igb` NICs instead, in which case the data interfaces appear as `GigabitEthernet0/0/0/X` (1 GbE). vmxnet3 forwards bulk traffic roughly 1.5× faster than igb; match your startup-config to whichever naming applies.

The management interface `MgmtEth0/RP0/CPU0/0` carries the containerlab node management IP and is not part of the dataplane.

```yaml
name: xrd
topology:
  nodes:
    xrd1:
      kind: cisco_xrd_vrouter
      image: vrnetlab/cisco_xrd-vrouter:25.4.2
    xrd2:
      kind: cisco_xrd_vrouter
      image: vrnetlab/cisco_xrd-vrouter:25.4.2
  links:
    - endpoints: ["xrd1:eth1", "xrd2:eth1"]
```

/// admonition | Dataplane throughput
    type: note
The emulated NICs are suitable for feature, protocol and dataplane-behaviour labs, not line-rate performance testing.
///

/// admonition | Bulk TCP between a Linux host and the node stalls
    type: note
If `ping` and UDP work but a sustained TCP transfer between a `linux` host and the node stalls, it's a path-MTU black hole in the emulated datapath (full-size frames are dropped under load), not an XRd issue. Lower the MTU on the Linux endpoints (`ip link set eth1 mtu 1400`) or clamp TCP MSS.
///

## Startup configuration

XRd vRouter nodes boot with a generated base configuration (the `clab` user, the management interface, and the SSH/NETCONF/gRPC servers). Supply a `startup-config` to layer your own configuration on top:

```yaml
topology:
  nodes:
    xrd:
      kind: cisco_xrd_vrouter
      image: vrnetlab/cisco_xrd-vrouter:25.4.2
      startup-config: xrd.cfg
```

The file is applied at first boot on top of the generated base, so it can contain partial configuration snippets (for example, just the data-interface addressing). Use the interface names that match the selected NIC type — `TenGigE0/0/0/X` for the default vmxnet3, or `GigabitEthernet0/0/0/X` when `XRD_NIC_TYPE: igb` is set.

/// admonition | Persisting configuration
    type: note
The micro-VM disk is ephemeral, so config committed at runtime is reset to the `startup-config` on the next boot (there is no `xr-storage`-style automatic persistence like the control-plane [`cisco_xrd`](xrd.md) kind). To snapshot your work, run [`containerlab save`](../../cmd/save.md): it writes each node's running config to `clab-<lab>/<node>/config/startup-config.cfg` in the lab directory, which is re-applied on the next deploy.
///
