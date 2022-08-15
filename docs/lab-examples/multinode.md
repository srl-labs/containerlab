|                               |                                                                                                   |
| ----------------------------- | ------------------------------------------------------------------------------------------------- |
| **Description**               | A lab demonstrating multi-node (multi-vm) capabilities                                            |
| **Components**                | Nokia SR OS, Juniper vMX                                                                          |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 6 GB <br/><small>per node</small> |
| **Topology file**             | [vxlan-vmx.clab.yml][vmx-topofile], [vxlan-sros.clab.yml][sros-topofile]                          |
| **Name**                      | vxlan01                                                                                           |
| **Version information**[^2]   | `containerlab:0.11.0`, `vr-sros:20.2.R1`, `vr-vmx:20.4R1.12`, `docker-ce:20.10.2`                 |

## Description
This lab demonstrates how containerlab can deploy labs on different machines and stitch the interfaces of the running nodes via VxLAN tunnels.

With such approach users are allowed to spread the load between multiple VMs and still have the nodes connected via p2p links as if they were sitting on the same virtual machine.

For the sake of the demonstration the topology used in this lab consists of just two virtualized routers [packaged in a container format](../manual/vrnetlab.md) - Nokia SR OS and Juniper vMX. Although the routers are running on different VMs, they logically form a back-to-back connection over a pair of interfaces aggregated in a logical bundle.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:7,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/multinode.drawio&quot;}"></div>
<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

Upon succesful lab deployment and configuration, the routers will be able to exchange LACP frames, thus proving a transparent L2 connectivity and will be able to ping each other.

## Deployment

Since this lab is of a multi-node nature, a user needs to have two machines/VMs and perform lab deployment process on each of them. The [lab directory](https://github.com/srl-labs/containerlab/tree/main/lab-examples/vxlan01/) has topology files named `vxlan-sros.clab.yml` and `vxlan-vmx.clab.yml` which are meant to be deployed on VM1 and VM2 accordingly.

The following command will deploy a lab on a specified host:

=== "VM1 (SROS)"
    ```bash
    clab dep -t vxlan-sros.clab.yml
    ```
=== "VM2 (VMX)"
    ```bash
    clab dep -t vxlan-vmx.clab.yml
    ```

### host links
Both topology files leverage [host link](../manual/network.md#host-links) feature which allows a container to have its interface to be connected to a container host namespace. Once the topology is created you will have one side of the veth link visible in the root namespace by the names specified in topo file. For example, `vxlan-sros.clab.yml` file has the following `links` section:

```yaml
  links:
    # we expose two sros container interfaces
    # to host namespace by using host interfaces style
    # docs: https://containerlab.dev/manual/network/#host-links
    - endpoints: ["sros:eth1", "host:sr-eth1"]
    - endpoints: ["sros:eth2", "host:sr-eth2"]
```

This will effectively make two veth pairs. Let us consider the first veth pair where one end of a it will be placed inside the container' namespace and named `eth1`, the other end will stay in the container host root namespace and will be named `sros-eth1`.  

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:8,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/multinode.drawio&quot;}"></div>

Same picture will be on VM2 with vMX interfaces exposed to a container host.

??? "verify host link"
    === "VM1"
        ```bash
        ❯ ip l | grep sros-eth
        622: sr-eth1@if623: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default 
        624: sr-eth2@if625: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default 
        ```
    === "VM2"
        ```bash
        ❯ ip l | grep vmx-eth
        1982: vmx-eth1@if1983: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default
        1984: vmx-eth2@if1985: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default
        ```

### vxlan tunneling
At this moment there is no connectivity between the routers, as the datapath is not ready. What we need to add is the VxLAN tunnels that will stitch SR OS container with vMX.

We do this by provisioning VxLAN tunnels that will _stitch_ the interfaces of our routers.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:9,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/multinode.drawio&quot;}"></div>

Logically we make our interface appear to be connected in a point-to-point fashion. To make these tunnels we leverage containerlab' [`tools vxlan create`](../cmd/tools/vxlan/create.md) command, that will create the VxLAN tunnel and the necessary redirection rules to forward traffic back-and-forth to a relevant host interface.

All we need is to provide the VMs address and choose VNI numbers. And do this on both hosts.

=== "VM1"
    ```bash
    ❯ clab tools vxlan create --remote 10.0.0.20 --id 10 --link sr-eth1

    ❯ clab tools vxlan create --remote 10.0.0.20 --id 20 --link sr-eth2
    ```
=== "VM2"
    ```bash
    ❯ clab tools vxlan create --remote 10.0.0.18 --id 10 --link vmx-eth1

    ❯ clab tools vxlan create --remote 10.0.0.18 --id 20 --link vmx-eth2
    ```

The above set of commands will create the necessary VxLAN tunnels and the datapath is ready.

At this moment, the connectivity diagrams becomes complete and can be depicted as follows:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:10,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/multinode.drawio&quot;}"></div>

## Configuration
Once the datapath is in place, we proceed with the configuration of a simple LACP use case, where both SR OS and vMX routers have their pair of interfaces aggregated into a LAG and form an LACP neighborship.

=== "SR OS"
    ```
    configure lag "lag-aggr" admin-state enable
    configure lag "lag-aggr" mode hybrid
    configure lag "lag-aggr" lacp mode active
    configure lag "lag-aggr" port 1/1/c1/1
    configure lag "lag-aggr" port 1/1/c2/1

    configure port 1/1/c1 admin-state enable
    configure port 1/1/c1 connector breakout c1-100g
    configure port 1/1/c1/1 admin-state enable
    configure port 1/1/c1/1 ethernet
    configure port 1/1/c1/1 ethernet mode hybrid

    configure port 1/1/c2 admin-state enable
    configure port 1/1/c2 connector breakout c1-100g
    configure port 1/1/c2/1 admin-state enable
    configure port 1/1/c2/1 ethernet mode hybrid

    configure router "Base" interface "toVMX" port lag-aggr:0
    configure router "Base" interface "toVMX" ipv4 primary address 192.168.1.1 prefix-length 24
    ```
=== "vMX"
    ```
    set interfaces ge-0/0/0 gigether-options 802.3ad ae0
    set interfaces ge-0/0/1 gigether-options 802.3ad ae0
    set interfaces ae0 aggregated-ether-options minimum-links 1
    set interfaces ae0 aggregated-ether-options link-speed 1g
    set interfaces ae0 aggregated-ether-options lacp active
    set interfaces ae0 unit 0 family inet address 192.168.1.2/24
    ```

## Verification

To verify that LACP protocol works the following commands can be issued on both routers to display information about the aggregated interface and LACP status:

=== "SR OS"
    ```
    # verifying operational status of LAG interface
    A:admin@sros# show lag "lag-aggr"

    ===============================================================================
    Lag Data
    ===============================================================================
    Lag-id         Adm     Opr     Weighted Threshold Up-Count MC Act/Stdby
        name
    -------------------------------------------------------------------------------
    65             up      up      No       0         2        N/A
        lag-aggr
    ===============================================================================

    # show LACP statistics. Both incoming and trasmitted counters will increase
    A:admin@sros# show lag "lag-aggr" lacp-statistics

    ===============================================================================
    LAG LACP Statistics
    ===============================================================================
    LAG-id    Port-id        Tx         Rx         Rx Error   Rx Illegal
                            (Pdus)     (Pdus)     (Pdus)     (Pdus)
    -------------------------------------------------------------------------------
    65        1/1/c1/1       78642      77394      0          0
    65        1/1/c2/1       78644      77396      0          0
    -------------------------------------------------------------------------------
    Totals                   157286     154790     0          0
    ===============================================================================
    ```
=== "vMX"
    ```
    admin@vmx> show interfaces ae0 brief
    Physical interface: ae0, Enabled, Physical link is Up
    Link-level type: Ethernet, MTU: 1514, Speed: 2Gbps, Loopback: Disabled, Source filtering: Disabled, Flow control: Disabled
    Device flags   : Present Running
    Interface flags: SNMP-Traps Internal: 0x4000

    Logical interface ae0.0
        Flags: Up SNMP-Traps 0x4004000 Encapsulation: ENET2
        inet  192.168.1.2/24
        multiservice


    admin@vmx> show lacp interfaces
    Aggregated interface: ae0
        LACP state:       Role   Exp   Def  Dist  Col  Syn  Aggr  Timeout  Activity
        ge-0/0/0       Actor    No    No   Yes  Yes  Yes   Yes     Fast    Active
        ge-0/0/0     Partner    No    No   Yes  Yes  Yes   Yes     Fast    Active
        ge-0/0/1       Actor    No    No   Yes  Yes  Yes   Yes     Fast    Active
        ge-0/0/1     Partner    No    No   Yes  Yes  Yes   Yes     Fast    Active
        LACP protocol:        Receive State  Transmit State          Mux State
        ge-0/0/0                  Current   Fast periodic Collecting distributing
        ge-0/0/1                  Current   Fast periodic Collecting distributing

    admin@vmx> show lacp statistics interfaces ae0
    Aggregated interface: ae0
        LACP Statistics:       LACP Rx     LACP Tx   Unknown Rx   Illegal Rx
        ge-0/0/0               78104       77469            0            0
        ge-0/0/1               78106       77471            0            0
    ```

After the control plane verfification let's verify that the dataplane is working by pinging the IP address of the remote interface (issued from SR OS node in the example):

```
A:admin@sros# ping 192.168.1.2
PING 192.168.1.2 56 data bytes
64 bytes from 192.168.1.2: icmp_seq=1 ttl=64 time=13.5ms.
64 bytes from 192.168.1.2: icmp_seq=2 ttl=64 time=2.61ms.
ping aborted by user

---- 192.168.1.2 PING Statistics ----
2 packets transmitted, 2 packets received, 0.00% packet loss
round-trip min = 2.61ms, avg = 8.04ms, max = 13.5ms, stddev = 0.000ms
```

Great! Additionally users can [capture the traffic](../manual/wireshark.md) from any of the interfaces involved in the datapath. To see the VxLAN encapsulation the VM's outgoing interfaces should be used.

[vmx-topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/vxlan01/vxlan-vmx.clab.yml
[sros-topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/vxlan01/vxlan-sros.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.