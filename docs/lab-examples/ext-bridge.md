|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | Connecting nodes via linux bridges                                   |
| **Components**                | [Nokia SR Linux][srl]                                                |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB |
| **Topology file**             | [br01.clab.yml][topofile]                                            |
| **Name**                      | br01                                                                 |

## Description
This lab consists of three Nokia SR Linux nodes connected to a linux bridge.

<center><div class="mxgraph" style="max-width:100%;border:1px solid transparent;" data-mxgraph="{&quot;page&quot;:8,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div></center>

!!!note
    `containerlab` **will not** create/remove the bridge interface on your behalf.

    bridge element must be part of the lab nodes. Consult with the [topology file][topofile] to see how to reference a bridge.

## Use cases
By introducing a link of `bridge` type to the containerlab topology, we are opening ourselves to some additional scenarios:

* interconnect nodes via a broadcast domain
* connect multiple fabrics together
* connect containerlab nodes to the applications/nodes running outside of the lab host


[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/br01/br01.clab.yml

[^1]: Resource requirements are provisional. Consult with SR Linux Software Installation guide for additional information.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>