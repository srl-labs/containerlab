|                               |                                                                                            |
| ----------------------------- | ------------------------------------------------------------------------------------------ |
| **Description**               | A Nokia SR Linux connected back-to-back with SONiC-VS                                      |
| **Components**                | [Nokia SR Linux][srl], [SONiC][sonic]                                                      |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB                       |
| **Topology file**             | [sonic01.clab.yml][topofile]                                                               |
| **Name**                      | sonic01                                                                                    |
| **Version information**[^2]   | `containerlab:0.9.0`, `srlinux:20.6.3-145`, `docker-sonic-vs:202012`, `docker-ce:19.03.13` |

## Description
A lab consists of an SR Linux node connected with Azure SONiC via a point-to-point ethernet link. Both nodes are also connected with their management interfaces to the `containerlab` docker network.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlsonic01.drawio&quot;}"></div>

## Use cases
This lab allows users to launch basic interoperability scenarios between Nokia SR Linux and SONiC operating systems.

### BGP
<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlsonic01.drawio&quot;}"></div>

This lab demonstrates a simple iBGP peering scenario between Nokia SR Linux and SONiC. Both nodes exchange NLRI with their loopback prefix making it reachable.

#### Configuration
Once the lab is deployed with containerlab, use the following configuration instructions to make interfaces configuration and enable BGP on both nodes.

=== "srl"
    Get into SR Linux CLI with `docker exec -it clab-sonic01-srl sr_cli` and start configuration
    ```bash
    # enter candidate datastore
    enter candidate

    # configure loopback and data interfaces
    set / interface ethernet-1/1 admin-state enable
    set / interface ethernet-1/1 subinterface 0 admin-state enable
    set / interface ethernet-1/1 subinterface 0 ipv4 address 192.168.1.1/24

    set / interface lo0 subinterface 0 admin-state enable
    set / interface lo0 subinterface 0 ipv4 address 10.10.10.1/32
    set / network-instance default interface ethernet-1/1.0
    set / network-instance default interface lo0.0

    # configure BGP
    set / network-instance default protocols bgp admin-state enable
    set / network-instance default protocols bgp router-id 10.10.10.1
    set / network-instance default protocols bgp autonomous-system 65001
    set / network-instance default protocols bgp group ibgp ipv4-unicast admin-state enable
    set / network-instance default protocols bgp group ibgp export-policy export-lo
    set / network-instance default protocols bgp neighbor 192.168.1.2 admin-state enable
    set / network-instance default protocols bgp neighbor 192.168.1.2 peer-group ibgp
    set / network-instance default protocols bgp neighbor 192.168.1.2 peer-as 65001

    # create export policy
    set / routing-policy policy export-lo statement 10 match protocol local
    set / routing-policy policy export-lo statement 10 action accept

    # commit config
    commit now
    ```
=== "sonic"
    Get into sonic container shell with `docker exec -it clab-sonic01-sonic bash` and configure the so-called _front-panel_ ports.  
    Since we defined only one data interface for our sonic/srl nodes, we need to confgure a single port and a loopback interface:
    ```bash
    config interface ip add Ethernet0 192.168.1.2/24
    config interface startup Ethernet0
    config loopback add Loopback0
    config interface ip add Loopback0 10.10.10.2/32
    config interface startup Loopback0
    ```
    Now when data interface has been configured, check to make sure in /etc/frr/daemons that "bgpd=yes".  Restart the frr service if required and verify that bgpd is running.
    ```bash
    root@sonic:/# service frr restart
    [ ok ] Stopped watchfrr.
    [ ok . Stopped staticd[....] Stopped zebra[....] Stopped bgpd
    .
    .
    [ ok ] Started watchfrr.
    root@sonic:/# service frr status
    [ ok ] Status of watchfrr: running.
    [ ok ] Status of zebra: running.
    [ ok ] Status of bgpd: running.
    [ ok ] Status of staticd: running.
    ```
    Then enter in the FRR shell to configure BGP by typing `vtysh` command inside the sonic container.
    ```bash
    # enter configuration mode
    configure

    # configure BGP
    router bgp 65001
      bgp router-id 10.10.10.2
      neighbor 192.168.1.1 remote-as 65001
      address-family ipv4 unicast
        network 10.10.10.2/32
      exit-address-family
    exit
    access-list all seq 5 permit any
    ```

#### Verification
Once BGP peering is established, the routes can be seen in GRT of both nodes:

=== "srl"
    ```bash
    A:srl# / show network-instance default route-table ipv4-unicast summary | grep bgp
    | 10.10.10.2/32                 | 0     | true       | bgp             | 0       | 170   | 192.168.1.2 (indirect)                   | None              |
    ```

=== "sonic"
    ```bash
    sonic# sh ip route
    Codes: K - kernel route, C - connected, S - static, R - RIP,
          O - OSPF, I - IS-IS, B - BGP, E - EIGRP, N - NHRP,
          T - Table, v - VNC, V - VNC-Direct, A - Babel, D - SHARP,
          F - PBR, f - OpenFabric,
          > - selected route, * - FIB route, q - queued route, r - rejected route

    K>* 0.0.0.0/0 [0/0] via 172.20.20.1, eth0, 00:20:55
    B>* 10.10.10.1/32 [200/0] via 192.168.1.1, Ethernet0, 00:01:51
    C>* 10.10.10.2/32 is directly connected, Loopback0, 00:00:53
    C>* 172.20.20.0/24 is directly connected, eth0, 00:20:55
    B   192.168.1.0/24 [200/0] via 192.168.1.0 inactive, 00:01:51
    C>* 192.168.1.0/24 is directly connected, Ethernet0, 00:03:50
    ```

Data plane confirms that routes have been programmed to FIB:
```
sonic# ping 10.10.10.1
PING 10.10.10.1 (10.10.10.1) 56(84) bytes of data.
64 bytes from 10.10.10.1: icmp_seq=1 ttl=64 time=2.28 ms
64 bytes from 10.10.10.1: icmp_seq=2 ttl=64 time=2.84 ms
```



[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[sonic]: https://sonic-net.github.io/SONiC/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/sonic01/sonic01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>