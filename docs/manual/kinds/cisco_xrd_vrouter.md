---
search:
  boost: 4
kind_code_name: cisco_xrd_vrouter
kind_display_name: Cisco XRd vRouter
---
# Cisco XRd vRouter

[Cisco XRd](https://www.cisco.com/c/en/us/products/collateral/routers/ios-xrd/solution-overview-c22-2927494.html) vRouter is the high-performance dataplane (DPDK) form factor of the containerized IOS XR. It is identified with the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md) and is built using the [vrnetlab](../vrnetlab.md) project.

Unlike the XRd Control Plane (the [`cisco_xrd`](xrd.md) kind), XRd vRouter's dataplane interfaces are PCI-only and cannot bind a veth directly. The vrnetlab integration therefore runs the XRd vRouter container inside a small KVM guest that presents emulated PCI NICs, so it can be wired with ordinary containerlab links like any other VM-based node.

To build the containerlab-compatible container image from the XRd vRouter container release refer to the [srl-labs/vrnetlab/cisco/xrd-vrouter](https://github.com/srl-labs/vrnetlab/tree/master/cisco/xrd-vrouter) documentation.

XRd vRouter nodes launched with containerlab come up pre-provisioned with SSH, NETCONF and gNMI services enabled.

/// admonition | Resource requirements
    type: warning
XRd vRouter dedicates a CPU core to the dataplane and uses hugepages. It needs 2 vCPU and ~5 GiB RAM plus 3 GiB of 1 GiB hugepages per node; containerlab defaults to 4 vCPU / 10 GiB. 8 GiB is the supported minimum — in our testing an idle node booted and forwarded with ~800 MB RAM to spare — so raise `RAM` for feature-heavy labs. The build and run host must have nested virtualization (`/dev/kvm`) and hugepages available.

Tune the allocated resources with the `VCPU` and `RAM` environment variables:

```yaml
    xrd:
      kind: cisco_xrd_vrouter
      image: vrnetlab/cisco_xrd-vrouter:25.4.2
      env:
        VCPU: 4
        RAM: 8192
```

///

## Managing Cisco XRd vRouter nodes

A Cisco XRd vRouter node launched with containerlab can be managed via the following interfaces:

/// tab | bash
to connect to a `bash` shell of a running node:

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

## Interface naming

Data interfaces use the `Gi0/0/0/X` naming where `X` starts at `0`:

* `eth1` → `GigabitEthernet0/0/0/0`
* `eth2` → `GigabitEthernet0/0/0/1`
* and so on.

The management interface `MgmtEth0/RP0/CPU0/0` carries the containerlab node management IP and is not part of the data plane.

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
