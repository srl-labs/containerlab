|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Arista cEOS             |
| **Components**                | [Nokia SR Linux][srl], [Arista cEOS][ceos]                           |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 2 GB |
| **Topology file**             | [srlceos01.yml][topofile]                                            |
| **Name**                      | srlceos01                                                            |

## Description
A lab consists of an SR Linux node connected with Arista cEOS via a point-to-point ethernet link. Both nodes are also connected with their management interfaces to the `containerlab` docker network.

<center><div class="mxgraph" style="max-width:100%;border:1px solid transparent;" data-mxgraph="{&quot;page&quot;:6,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/containerlab-diagrams/main/containerlab.drawio&quot;}"></div></center>

## Use cases
This lab allows users to launch basic interoperability scenarios between Nokia SR Linux and Arista cEOS operating systems.

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[ceos]: https://www.arista.com/en/products/software-controlled-container-networking
[topofile]: https://github.com/srl-wim/container-lab/tree/master/lab-examples/srlceos01/srlceos01.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fsrl-wim%2Fcontainerlab-diagrams%2Fmain%2Fcontainerlab.drawio" async></script>