# Nokia SR Linux and Arrcus ArcOS

|                               |                                                                             |
| ----------------------------- | --------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Arrcus ArcOS                   |
| **Components**                | [Nokia SR Linux][srl], [Arrcus ArcOS][arcos]                                |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 4 GB        |
| **Topology file**             | [srlarcos01.clab.yml][topofile]                                             |
| **Name**                      | srlarcos01                                                                  |
| **Version information**[^2]   | `containerlab:0.56.0`, `srlinux:25.3.2`, `arcos:4.3.1B`, `docker-ce:27.1.1` |

## Description

A lab consists of an SR Linux node connected with Arrcus ArcOS via a point-to-point ethernet link. Both nodes are also connected with their management interfaces to the `containerlab` docker network.

## Use cases

This lab allows users to launch basic interoperability scenarios between Nokia SR Linux and Arrcus ArcOS operating systems.

## Configuration

Both SR Linux and ArcOS nodes come with a startup config files referenced for them. These user-defined startup files introduce the following change on top of the default config that these nodes boot with:

* On SR Linux, interface `ethernet-1/1` is configured with `192.168.0.0/31` address and this interface attached to the default network instance.
* On ArcOS, interface `swp1` is configured with `192.168.0.1/31` address.

## Verification

When the deployment of the lab finishes, users can validate that the datapath works between the nodes by pinging the directly connected interfaces from either node.

Here is an example from SR Linux side:

```bash
--{ running }--[  ]--
A:admin@srl# ping network-instance default 192.168.0.1 -c 2
Using network instance default
PING 192.168.0.1 (192.168.0.1) 56(84) bytes of data.
64 bytes from 192.168.0.1: icmp_seq=1 ttl=64 time=1.99 ms
64 bytes from 192.168.0.1: icmp_seq=2 ttl=64 time=1.38 ms

--- 192.168.0.1 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1001ms
rtt min/avg/max/mdev = 1.379/1.685/1.992/0.306 ms
```

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[arcos]: ../manual/kinds/arcos.md
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.

[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
