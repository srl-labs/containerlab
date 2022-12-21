|                               |                                                                                  |
| ----------------------------- | -------------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Cisco XRd                         |
| **Components**                | [Nokia SR Linux][srl], [Cisco XRd][xrd]                                               |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 4 GB             |
| **Topology file**             | [srlxrd01.clab.yml][topofile]                                                   |
| **Name**                      | srlxrd01                                                                        |
| **Version information**[^2]   | `containerlab:0.34.0`, `srlinux:22.11.1`, `xrd:7.8.1`, `docker-ce:19.03.13` |

## Description

A lab consists of an SR Linux node connected with Cisco XRd via a point-to-point ethernet link. Both nodes are also connected with their management interfaces to the `containerlab` docker network.

## Use cases

This lab allows users to launch basic interoperability scenarios between Nokia SR Linux and Cisco XRd operating systems.

## Configuration

Both SR Linux and XRd nodes come with a startup config files referenced for them. These user-defined startup files introduce the following change on top of the default config that these nodes boot with:

* On SR Linux, interface `ethernet-1/1` is configured with `192.168.0.0/31` address and this interface attached to the default network instance.
* On XRd, interface `Gi 0/0/0/0` is configured with `192.168.0.1/31` address.

## Verification

When the deployment of the lab finishes, users can validate that the datapath works between the nodes by pinging the directly connected interfaces from either node.

Here is an example from SR Linux side:

```bash
--{ running }--[  ]--
A:srl# ping network-instance default 192.168.0.1 
Using network instance default
PING 192.168.0.1 (192.168.0.1) 56(84) bytes of data.
64 bytes from 192.168.0.1: icmp_seq=1 ttl=255 time=12.0 ms
64 bytes from 192.168.0.1: icmp_seq=2 ttl=255 time=6.83 ms
^C
--- 192.168.0.1 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1001ms
rtt min/avg/max/mdev = 6.830/9.428/12.027/2.600 ms
```

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[xrd]: ../manual/kinds/xrd.md
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/srlxrd01/srlxrd01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
