|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | Two Nokia SR Linux nodes                                             |
| **Components**                | [Nokia SR Linux][srl]                                                |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 2 GB |
| **Topology file**             | [srl02.yml][topofile]                                                |
| **Prefix**                    | srl02                                                                |

## Description
A lab consists of two SR Linux nodes connected with each other via a point-to-point link over `e1-1` interfaces. Both nodes are also connected with their management interfaces to the `containerlab` docker network.

<center><div class="mxgraph" style="max-width:100%;border:1px solid transparent;" data-mxgraph="{&quot;page&quot;:3,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/containerlab-diagrams/main/containerlab.drawio&quot;}"></div></center>

## Use cases
This lab, besides having the same objectives as [srl01](single-srl.md) lab, also enables the following scenarios:

* get to know protocols and services configuration
* verify basic control plane and data plane operations
* explore SR Linux state datastore for the paths which reflect control plane operation metrics or dataplane counters

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-wim/container-lab/tree/master/lab-examples/srl02/srl02.yaml

[^1]: Resource requirements are provisional. Consult with SR Linux Software Installation guide for additional information.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fsrl-wim%2Fcontainerlab-diagrams%2Fmain%2Fcontainerlab.drawio" async></script>