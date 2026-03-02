---
search:
  boost: 4
kind_code_name: kinds_overview
kind_display_name: Kinds
---
# -{{ kind_display_name }}-

This overview of -{{ kind_display_name }}- pages (id `-{{ kind_code_name }}-`) explains how containerlab interprets the notion of a "kind".
Containerlab launches, wires up and manages container-based labs. The steps required to launch a vanilla `debian` or `centos` container image aren't at all different. On the other hand, Nokia SR Linux launching procedure is nothing like the one for Arista cEOS.

Things like required syscalls, mounted directories, entrypoint and commands to execute are all different for the containerized NOS'es. To let containerlab understand which launching sequence to use, the notion of a `kind` was introduced. Essentially `kinds` abstract away the need to understand certain setup peculiarities of different NOS'es.

Given the following [topology definition file](../topo-def-file.md), containerlab is able to know how to launch `node1` as an SR Linux container and `node2` as a cEOS one because they are associated with the kinds:

```yaml
name: srlceos01

topology:
  nodes:
    node1:
      # node1 is of nokia_srlinux kind
      kind: nokia_srlinux
      type: ixr-d2l
      image: ghcr.io/nokia/srlinux
    node2:
      # node2 is of arista_ceos kind
      kind: arista_ceos
      image: ceos:4.32.0F

  links:
    - endpoints: ["node1:e1-1", "node2:eth1"]
```

Containerlab supports a fixed number of platforms. Each platform is identified with a `kind` name that is used in topology definitions.

Within each predefined kind, we store the necessary information that is used to successfully launch the container. The following kinds are supported by containerlab:

| Name                       | Kind name                                           | Status    | Packaging |
| -------------------------- | --------------------------------------------------- | --------- | :-------: |
| **Nokia SR Linux**         | [`nokia_srlinux`](srl.md)                           | supported | container |
| **Nokia SR OS (VM)**       | [`nokia_sros`](vr-sros.md)                          | supported |    VM     |
| **Nokia SR OS (SRsim)**    | [`nokia_srsim`](sros.md)                            | supported | container |
| **Arista cEOS**            | [`arista_ceos`](ceos.md)                            | supported | container |
| **Arista vEOS**            | [`arista_veos`](vr-veos.md)                         | supported |    VM     |
| **Juniper cRPD**           | [`juniper_crpd`](crpd.md)                           | supported | container |
| **Juniper vMX**            | [`juniper_vmx`](vr-vmx.md)                          | supported |    VM     |
| **Juniper vQFX**           | [`juniper_vqfx`](vr-vqfx.md)                        | supported |    VM     |
| **Juniper vSRX**           | [`juniper_vsrx`](vr-vsrx.md)                        | supported |    VM     |
| **Juniper vJunos-router**  | [`juniper_vjunosrouter`](vr-vjunosrouter.md)        | supported |    VM     |
| **Juniper vJunos-switch**  | [`juniper_vjunosswitch`](vr-vjunosswitch.md)        | supported |    VM     |
| **Juniper vJunosEvolved**  | [`juniper_vjunosevolved`](vr-vjunosevolved.md)      | supported |    VM     |
| **Juniper cJunosEvolved**  | [`juniper_cjunosevolved`](cjunosevolved.md)         | supported |    VM     |
| **Cisco XRd**              | [`cisco_xrd`](xrd.md)                               | supported | container |
| **Cisco XRv**              | [`cisco_xrv`](vr-xrv.md)                            | supported |    VM     |
| **Cisco XRv9k**            | [`cisco_xrv9k`](vr-xrv9k.md)                        | supported |    VM     |
| **Cisco CSR1000v**         | [`cisco_csr1000v`](vr-csr.md)                       | supported |    VM     |
| **Cisco Nexus 9000v**      | [`cisco_n9kv`](vr-n9kv.md)                          | supported |    VM     |
| **Cisco 8000**             | [`cisco_c8000`](c8000.md)                           | supported |    VM+    |
| **Cisco 8000v**            | [`cisco_c8000v`](vr-c8000v.md)                      | supported |    VM     |
| **Cisco Catalyst 9000v**   | [`cisco_cat9kv`](vr-cat9kv.md)                      | supported |    VM     |
| **Cisco IOL**              | [`cisco_iol`](cisco_iol.md)                         | supported | container |
| **Cisco FTDv**             | [`cisco_ftdv`](vr-ftdv.md)                          | supported |    VM     |
| **Cumulus VX**             | [`cumulus_cvx`](cvx.md)                             | supported | container |
| **Aruba ArubaOS-CX**       | [`aruba_aoscx`](vr-aoscx.md)                        | supported |    VM     |
| **SONiC (VS)**             | [`sonic-vs`](sonic-vs.md)                           | supported | container |
| **SONiC VM**               | [`sonic-vm`](sonic-vm.md)                           | supported |    VM     |
| **Dell FTOS10v**           | [`dell_ftosv`](vr-ftosv.md)                         | supported |    VM     |
| **Dell SONiC**             | [`dell_sonic`](dell_sonic.md)                       | supported |    VM     |
| **Mikrotik RouterOS**      | [`mikrotik_ros`](vr-ros.md)                         | supported |    VM     |
| **Huawei VRP**             | [`huawei_vrp`](huawei_vrp.md)                       | supported |    VM     |
| **IPInfusion OcNOS**       | [`ipinfusion_ocnos`](ipinfusion-ocnos.md)           | supported |    VM     |
| **Palo Alto PAN-OS**       | [`paloalto_panos`](vr-pan.md)                       | supported |    VM     |
| **Fortinet Fortigate**     | [`fortinet_fortigate`](fortinet_fortigate.md)       | supported |    VM     |
| **Check Point Cloudguard** | [`checkpoint_cloudguard`](checkpoint_cloudguard.md) | supported |    VM     |
| **6WIND VSR**              | [`6wind_vsr`](6wind_vsr.md)                         | supported | container |
| **Keysight ixia-c-one**    | [`keysight_ixia-c-one`](keysight_ixia-c-one.md)     | supported | container |
| **Arrcus ArcOS**           | [`arrcus_arcos`](arrcus_arcos.md)                   | supported | container |
| **Code Laboratory BNG**   | [`codelaboratory_bng`](codelaboratory_bng.md)       | supported | container |
| **FD.io VPP**              | [`fdio_vpp`](fdio_vpp.md)                           | supported | container |
| **RARE/freeRtr**           | [`rare`](rare-freertr.md)                           | supported | container |
| **VyOS Networks VyOS**     | [`vyosnetworks_vyos`](vyosnetworks_vyos.md)         | supported |    VM     |
| **Generic VM**             | [`generic_vm`](generic_vm.md)                       | supported |    VM     |
| **Linux container**        | [`linux`](linux.md)                                 | supported | container |
| **Ostinato**               | [`linux`](ostinato.md)                              | supported | container |
| **FreeBSD**                | [`freebsd`](freebsd.md)                             | supported |    VM     |
| **OpenWRT**                | [`openwrt`](openwrt.md)                             | supported |    VM     |
| **OpenBSD**                | [`openbsd`](openbsd.md)                             | supported |    VM     |
| **Kubernetes in Docker**   | [`k8s-kind`](k8s-kind.md)                           | supported | service   |
| **Linux bridge**           | [`bridge`](bridge.md)                               | supported |    N/A    |
| **Openvswitch bridge**     | [`ovs-bridge`](ovs-bridge.md)                       | supported |    N/A    |
| **External container**     | [`ext-container`](ext-container.md)                 | supported | container |
| **Host**                   | [`host`](host.md)                                   | supported |    N/A    |

Refer to a specific kind documentation article for kind-specific details.

