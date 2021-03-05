|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | Two Nokia SR Linux nodes                                             |
| **Components**                | [Nokia SR Linux][srl]                                                |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB |
| **Topology file**             | [srl02.yml][topofile]                                                |
| **Name**                      | srl02                                                                |
| **Validated versions**[^2]    | `containerlab v0.8.2`,`srlinux:20.6.2-332`                           |

## Description
A lab consists of two SR Linux nodes connected with each other via a point-to-point link over `e1-1` interfaces. Both nodes are also connected with their management interfaces to the `clab` docker network.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:7,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/containerlab-diagrams/main/srl02.drawio&quot;}"></div>

## Configuration
The nodes of this lab have been provided with a startup configuration by means of `config` directive in the topo definition file. The startup configuration adds loopback and interfaces addressing as per the diagram above.

Once the lab is started, the nodes will be able to ping each other via configured interfaces:

```
A:srl1# ping 192.168.0.1 network-instance default
Using network instance default
PING 192.168.0.1 (192.168.0.1) 56(84) bytes of data.
64 bytes from 192.168.0.1: icmp_seq=1 ttl=64 time=5.17 ms
```

## Use cases
This lab, besides having the same objectives as [srl01](single-srl.md) lab, also enables the following scenarios:

* get to know protocols and services configuration
* verify basic control plane and data plane operations
* explore SR Linux state datastore for the paths which reflect control plane operation metrics or dataplane counters

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-wim/container-lab/tree/master/lab-examples/srl02/srl02.yml

[^1]: Resource requirements are provisional. Consult with SR Linux Software Installation guide for additional information.
[^2]: versions of respective container images or software that was used to create the lab.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>