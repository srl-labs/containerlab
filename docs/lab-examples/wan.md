|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | WAN emulating topology                                               |
| **Components**                | [Nokia SR Linux][srl]                                                |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 3 GB |
| **Topology file**             | [srl03.yml][topofile]                                                |
| **Name**                      | srl03                                                                |

## Description
Nokia SR Linux while focusing on the data center deployments in the first releases, will also be suitable for WAN deployments. In this lab users presented with a small WAN topology of four interconnected SR Linux nodes with multiple p2p interfaces between them.

<center><div class="mxgraph" style="max-width:100%;border:1px solid transparent;" data-mxgraph="{&quot;page&quot;:9,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/container-lab/diagrams/containerlab.drawio&quot;}"></div></center>

## Use cases
The WAN-centric scenarios can be tested with this lab:

* Link aggregation
* WAN protocols and features

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-wim/container-lab/tree/master/lab-examples/srl03/srl03.yml

[^1]: Resource requirements are provisional. Consult with SR Linux Software Installation guide for additional information.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>