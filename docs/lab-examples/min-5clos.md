|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | A 5-stage CLOS topology based on Nokia SR Linux                      |
| **Components**                | [Nokia SR Linux][srl]                                                |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 4 <br/>:fontawesome-solid-memory: 8 GB |
| **Topology file**             | [clos02.yml][topofile]                                               |
| **Name**                      | clos02                                                               |

## Description
This labs provides a lightweight folded 5-stage CLOS fabric with Super Spine level bridging two PODs.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:7,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

The topology is additionally equipped with the Linux containers connected to leaves to facilitate use cases which require access side emulation.

## Use cases
With this lightweight CLOS topology a user can exhibit the following scenarios:

* perform configuration tasks applied to the 5-stage CLOS fabric
* demonstrate fabric behavior leveraging the user-emulating linux containers attached to the leaves

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/tree/master/lab-examples/clos02/clos02.yml

[^1]: Resource requirements are provisional. Consult with SR Linux Software Installation guide for additional information.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>