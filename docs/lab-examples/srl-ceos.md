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

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[ceos]: https://www.arista.com/en/products/software-controlled-container-networking
[topofile]: https://github.com/srl-wim/container-lab/tree/master/lab-examples/srlceos01/srlceos01.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/embed2.js"></script>