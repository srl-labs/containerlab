|                               |                                                                                   |
| ----------------------------- | --------------------------------------------------------------------------------- |
| **Description**               | An F5 BIG-IP VE connected to a Linux peer                                         |
| **Components**                | [F5 BIG-IP VE][bigip], [Multitool Alpine Linux][multitool]                        |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 4 <br/>:fontawesome-solid-memory: 8 GB              |
| **Topology file**             | [f5bigipve01.clab.yml][topofile]                                                  |
| **Name**                      | f5bigipve01                                                                       |
| **Version information**[^2]   | `vrnetlab/f5_bigip-ve:17.5.1.3-0.0.19`, `wbitt/network-multitool:alpine-extra`    |

## Description

This lab demonstrates how to deploy an [F5 BIG-IP VE](../manual/kinds/f5_bigipve.md) node using containerlab and connect a dataplane interface (`1.1`) to a simple Linux peer.

```
peer1:eth1 <----> bigip1:1.1
```

BIG-IP management networking uses vrnetlab management *passthrough*: the BIG-IP management IP is the same as the containerlab management IP for the node.

/// admonition
    type: warning
BIG-IP VE is proprietary software and requires a valid license to fully function. This example does not include any proprietary artifacts and does not provide guidance for bypassing licensing requirements.
///

## Prerequisites

- A Linux host with hardware virtualization support (KVM) available as `/dev/kvm`.
- A locally built vrnetlab BIG-IP VE container image (the topology references `vrnetlab/f5_bigip-ve:17.5.1.3-0.0.19`).
  - See the upstream build steps in the vrnetlab `f5_bigip` image documentation and the containerlab kind documentation: [F5 BIG-IP VE](../manual/kinds/f5_bigipve.md).
- A BIG-IP VE license (demo/evaluation licenses are available via an F5 registered account, subject to eligibility/terms).

## Deployment

Deploy the lab using the topology file in this directory:

```bash
sudo containerlab deploy -t f5bigipve01.clab.yml
```

BIG-IP VE takes a while to boot (often 10+ minutes). Watch the logs until you see `Startup complete`:

```bash
docker logs -f clab-f5bigipve01-bigip1
```

Inspect the running lab to get the BIG-IP management address:

```bash
containerlab inspect -t f5bigipve01.clab.yml
```

Example output (addresses and IDs will differ):

```
+---+-----------------------+--------------+------------------------------+------------+---------+----------------+----------------------+
| # |         Name          | Container ID |            Image             |    Kind    |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-----------------------+--------------+------------------------------+------------+---------+----------------+----------------------+
| 1 | clab-f5bigipve01-bigip1 | ...        | vrnetlab/f5_bigip-ve:...     | f5_bigip_ve | running | 172.20.20.4/24 | 3fff:172:20:20::4/64 |
| 2 | clab-f5bigipve01-peer1  | ...        | wbitt/network-multitool:...  | linux      | running | 172.20.20.5/24 | 3fff:172:20:20::5/64 |
+---+-----------------------+--------------+------------------------------+------------+---------+----------------+----------------------+
```

## Access BIG-IP (SSH/HTTPS)

Use the IPv4 management address reported by `containerlab inspect` (e.g., `172.20.20.4` without the `/24`).

- SSH (CLI):

```bash
ssh root@<bigip-mgmt-ip>
```

- HTTPS (Web UI/API):

```bash
https://<bigip-mgmt-ip>
```

This lab also publishes BIG-IP HTTPS `443` to host port `8443` (`ports: - 8443:443`), so you can also access the GUI as `https://localhost:8443`.

Default credentials and override options are documented in [F5 BIG-IP VE](../manual/kinds/f5_bigipve.md).

## Verify dataplane interface wiring

This topology connects BIG-IP `1.1` (mapped to `eth1` in the container) to `peer1:eth1`.

On the host, confirm the dataplane link exists on the BIG-IP container:

```bash
docker exec -it clab-f5bigipve01-bigip1 ip -br link show dev eth1
```

Confirm the Linux peer also has `eth1` present:

```bash
docker exec -it clab-f5bigipve01-peer1 ip -br link show dev eth1
```

Optionally, after logging into BIG-IP over SSH, confirm the VM sees `1.1`:

```
tmsh show net interface 1.1
```

## Cleanup

Destroy the lab:

```bash
sudo containerlab destroy -t f5bigipve01.clab.yml
```

[bigip]: ../manual/kinds/f5_bigipve.md
[multitool]: https://github.com/wbitt/Network-MultiTool
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/f5bigipve01/f5bigipve01.clab.yml

[^1]: Resource requirements are provisional. Consult with the BIG-IP VE and vrnetlab documentation for additional information.
[^2]: Version information is provided as an example reference; adjust image tags to match the artifacts you built.
