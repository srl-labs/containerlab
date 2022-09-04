|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | Two Nokia SR Linux nodes                                             |
| **Components**                | [Nokia SR Linux][srl]                                                |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB |
| **Topology file**             | [srl02.clab.yml][topofile]                                           |
| **Name**                      | srl02                                                                |
| **Validated versions**[^2]    | `containerlab v0.26.2`,`srlinux:21.11.3`                             |

## Description
A lab consists of two SR Linux nodes connected via a point-to-point link over `e1-1` interfaces. Both nodes are also connected with their management interfaces to the `clab` docker network.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:7,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srl02.drawio&quot;}"></div>

## Configuration
The nodes of this lab have been provided with a startup configuration using [`startup-config`](../manual/kinds/srl.md#user-defined-startup-config) directive. The startup configuration adds loopback and interfaces addressing as per the diagram above.

Once the lab is started, the nodes will be able to ping each other via configured interfaces:

```
--{ running }--[  ]--
A:srl1# ping network-instance default 192.168.0.1
Using network instance default
PING 192.168.0.1 (192.168.0.1) 56(84) bytes of data.
64 bytes from 192.168.0.1: icmp_seq=1 ttl=64 time=55.2 ms
64 bytes from 192.168.0.1: icmp_seq=2 ttl=64 time=6.61 ms
64 bytes from 192.168.0.1: icmp_seq=3 ttl=64 time=8.92 ms
64 bytes from 192.168.0.1: icmp_seq=4 ttl=64 time=14.2 ms
^C
--- 192.168.0.1 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3005ms
rtt min/avg/max/mdev = 6.610/21.232/55.173/19.790 ms
```

## Use cases
This lab, besides having the same objectives as [srl01](single-srl.md) lab, also enables the following scenarios:

* get to know protocols and services configuration
* verify basic control plane and data plane operations
* explore SR Linux state datastore for the paths which reflect control plane operation metrics or dataplane counters

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/srl02/srl02.clab.yml

[^1]: Resource requirements are provisional. Consult with SR Linux Software Installation guide for additional information.
[^2]: versions of respective container images or software that was used to create the lab.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>