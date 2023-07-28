---
search:
  boost: 4
---
# Kinds

Containerlab launches, wires up and manages container-based labs. The steps required to launch a vanilla `debian` or `centos` container image aren't at all different. On the other hand, Nokia SR Linux launching procedure is nothing like the one for Arista cEOS.

Things like required syscalls, mounted directories, entrypoint and commands to execute are all different for the containerized NOS'es. To let containerlab understand which launching sequence to use, the notion of a `kind` was introduced. Essentially `kinds` abstract away the need to understand certain setup peculiarities of different NOS'es.

Given the following [topology definition file](../topo-def-file.md), containerlab is able to know how to launch `node1` as an SR Linux container and `node2` as a cEOS one because they are associated with the kinds:

```yaml
name: srlceos01

topology:
  nodes:
    node1:
      kind: srl              # node1 is of srl kind
      type: ixrd2
      image: ghcr.io/nokia/srlinux
    node2:
      kind: ceos             # node2 is of ceos kind
      image: ceos:4.25F

  links:
    - endpoints: ["node1:e1-1", "node2:eth1"]
```

Containerlab supports a fixed number of platforms. Most platforms are identified with both a short and a long `kind` name; these names can be used interchangeably.

Within each predefined kind, we store the necessary information that is used to successfully launch the container. The following kinds are supported by containerlab:

| Name                      | Short/Long kind name                                | Status    | Packaging |
| ------------------------- | --------------------------------------------------- | --------- | :-------: |
| **Nokia SR Linux**        | [`srl/nokia_srlinux`](srl.md)                       | supported | container |
| **Nokia SR OS**           | [`vr-sros/nokia_sros`](vr-sros.md)                  | supported |    VM     |
| **Arista cEOS**           | [`ceos/arista_ceos`](ceos.md)                       | supported | container |
| **Arista vEOS**           | [`vr-veos/vr-arista_veos`](vr-veos.md)              | supported |    VM     |
| **Juniper cRPD**          | [`crpd/juniper_crpd`](crpd.md)                      | supported | container |
| **Juniper vMX**           | [`vr-vmx/vr-juniper_vmx`](vr-vmx.md)                | supported |    VM     |
| **Juniper vQFX**          | [`vr-vqfx/vr-juniper_vqfx`](vr-vqfx.md)             | supported |    VM     |
| **Juniper vSRX**          | [`vr-vsrx/vr-juniper_vsrx`](vr-vsrx.md)             | supported |    VM     |
| **Cisco XRv9k**           | [`vr-xrv9k/vr-cisco_xrv9k`](vr-xrv9k.md)            | supported |    VM     |
| **Cisco XRv**             | [`vr-xrv/vr-cisco_xrv`](vr-xrv.md)                  | supported |    VM     |
| **Cisco Nexus 9000v**     | [`vr-n9kv/vr-cisco_n9kv`](vr-n9kv.md)               | supported |    VM     |
| **Cumulus VX**            | [`cvx/cumulus_cvx`](cvx.md)                         | supported | container |
| **SONiC**                 | [`sonic`](sonic-vs.md)                              | supported | container |
| **Dell FTOS**             | [`vr-ftosv/vr-dell_ftos`](vr-ftosv.md)              | supported |    VM     |
| **Mikrotik Router OS**    | [`vr-ros/vr-mikrotik_ros`](vr-ros.md)               | supported |    VM     |
| **Palo Alto PAN OS**      | [`vr-panos/vr-paloalto_panos`](vr-pan.md)           | supported |    VM     |
| **IPInfusion OcNOS**      | [`ipinfusion_ocnos`](ipinfusion-ocnos.md)           | supported |    VM     |
| **Keysight ixia-c-one**   | [`keysight_ixia-c-one`](keysight_ixia-c-one.md)     | supported | container |
| **Checkpoint Cloudguard** | [`checkpoint_cloudguard`](checkpoint_cloudguard.md) | supported |    VM     |
| **Linux container**       | [`linux`](linux.md)                                 | supported | container |
| **Linux bridge**          | [`bridge`](bridge.md)                               | supported |    N/A    |
| **OvS bridge**            | [`ovs-bridge`](ovs-bridge.md)                       | supported |    N/A    |
| **mysocketio node**       | [`mysocketio`](../published-ports.md)               | supported |    N/A    |
| **RARE/freeRtr node**     | [`rare`](rare-freertr.md)                           | supported | container |
| **Aruba ArubaOS-CX**      | [`vr-aoscx/vr-aruba_aoscx`](vr-aoscx.md)            | supported |    VM     |

Refer to a specific kind documentation article for kind-specific details.
