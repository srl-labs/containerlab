|                               |                                                                                  |
| ----------------------------- | -------------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Arista cEOS                         |
| **Components**                | [Nokia SR Linux][srl], [Arista cEOS][ceos]                                       |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 2 GB             |
| **Topology file**             | [srlceos01.yml][topofile]                                                        |
| **Name**                      | srlceos01                                                                        |
| **Version information**[^2]   | `containerlab:0.9.0`, `srlinux:20.6.3-145`, `ceos:4.25.0F`, `docker-ce:19.03.13` |

## Description
A lab consists of an SR Linux node connected with Arista cEOS via a point-to-point ethernet link. Both nodes are also connected with their management interfaces to the `containerlab` docker network.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/container-lab/diagrams/srlceos01.drawio&quot;}"></div>

## Use cases
This lab allows users to launch basic interoperability scenarios between Nokia SR Linux and Arista cEOS operating systems.

### BGP
<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/container-lab/diagrams/srlceos01.drawio&quot;}"></div>

This lab demonstrates a simple iBGP peering scenario between Nokia SR Linux and Arista cEOS.

#### Configuration
Once the lab is deployed with containerlab, use the following configuration instructions to make interfaces configuration and enable BGP on both nodes.

=== "srl"
    Get into SR Linux CLI with `docker exec -it clab-srlceos01-srl sr_cli` and start configuration
    ```bash
    # enter candidate datastore
    enter candidate
    
    # configure loopback and data interfaces
    set / interface ethernet-1/1 admin-state enable
    set / interface ethernet-1/1 subinterface 0 admin-state enable
    set / interface ethernet-1/1 subinterface 0 ipv4 address 192.168.1.1/24

    set / interface lo0 subinterface 0 admin-state enable
    set / interface lo0 subinterface 0 ipv4 address 10.10.10.1/32

    # configure BGP
    set / network-instance default router-id 10.10.10.1
    set / network-instance default interface ethernet-1/1.0
    set / network-instance default interface lo0.0
    
    set / network-instance default protocols ospf instance main admin-state enable
    set / network-instance default protocols ospf instance main version ospf-v2
    set / network-instance default protocols ospf instance main area 0.0.0.0 interface ethernet-1/1.0 interface-type point-to-point
    set / network-instance default protocols ospf instance main area 0.0.0.0 interface ethernet-1/1.0

    # commit config
    commit now
    ```
=== "ceos"
    cRPD configuration needs to be done both from the container process, as well as within the CLI.  
    First attach to the container process `bash` shell and configure interfaces: `docker exec -it clab-srlceos01-crpd bash`
    ```bash
    # configure linux interfaces
    ip addr add 192.168.1.2/24 dev eth1
    ip addr add 10.10.10.2/32 dev lo
    ```
    Then launch the CLI and continue configuration `docker exec -it clab-srlceos01-crpd cli`:
    ```bash
    # enter configuration mode
    configure
    set routing-options router-id 10.10.10.2

    set protocols ospf area 0.0.0.0 interface eth1 interface-type p2p
    set protocols ospf area 0.0.0.0 interface lo.0 interface-type nbma
    
    # commit configuration
    commit
    ```

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[ceos]: https://www.arista.com/en/products/software-controlled-container-networking
[topofile]: https://github.com/srl-wim/container-lab/tree/master/lab-examples/srlceos01/srlceos01.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/embed2.js"></script>