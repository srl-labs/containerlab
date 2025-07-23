# SR-SIM

|                               |                                                                                  |
| ----------------------------- | -------------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Nokia SR OS                         |
| **Components**                | [Nokia SR Linux][srl], [Nokia SR OS][sros]                                       |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 5 GB             |
| **Topology file**             | [sros-srl.clab.yml][topofile]                                                    |
| **Name**                      | sr01                                                                             |
| **Version information**[^2]   | `containerlab:0.69.0`, `srlinux:25.7`, `nokia_srsim:25.7.R1`, `docker-ce:28.1.1` |

## Description

A lab consists of an SR Linux node connected with Nokia SR OS via a point-to-point ethernet link between the `ethernet-1/1` and `1/1/c1/1` respective interfaces.

> Nokia SR OS uses the [SR-SIM](../manual/kinds/sros.md) containerized router.

-{{diagram(url='srl-labs/containerlab/diagrams/sr-sim.drawio', page=0, title='')}}-

Both nodes are also provisioned with the basic startup configuration that configures the interface between the nodes with IPv4 and IPv6 addresses. As a result, upon the lab deployment, the nodes are able to ping each other over both IPv4 and IPv6 protocols.

```srl
[/]
A:admin@sros# show router interface "to-srl" 

===============================================================================
Interface Table (Router: Base)
===============================================================================
Interface-Name                   Adm       Opr(v4/v6)  Mode    Port/SapId
   IP-Address                                                  PfxState
-------------------------------------------------------------------------------
to-srl                           Up        Up/Up       Network 1/1/c1/1:0
   10.0.0.2/24                                                 n/a
   2001:10::2/96                                               PREFERRED
   fe80::1e53:1ff:fe00:0/64                                    PREFERRED
-------------------------------------------------------------------------------
Interfaces : 1
===============================================================================


[/]
A:admin@sros# ping 10.0.0.1 count 2 router-instance "Base" 
PING 10.0.0.1 56 data bytes
64 bytes from 10.0.0.1: icmp_seq=1 ttl=64 time=39.8ms.
64 bytes from 10.0.0.1: icmp_seq=2 ttl=64 time=1.48ms.

---- 10.0.0.1 PING Statistics ----
2 packets transmitted, 2 packets received, 0.00% packet loss
round-trip min = 1.48ms, avg = 20.6ms, max = 39.8ms, stddev = 19.2ms
```

## Use cases

This lab allows users to launch basic interoperability scenarios between Nokia SR Linux and Nokia SR OS network operating systems.

The SR-SIM lab directory [contains](https://github.com/srl-labs/containerlab/tree/main/lab-examples/sr-sim) more lab examples with different topologies and configurations that can be used to deploy more complex scenarios.

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[sros]: https://www.nokia.com/networks/products/service-router-operating-system/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/sr-sim/sros-srl.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
