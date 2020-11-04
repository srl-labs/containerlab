|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | A 5-stage CLOS topology based on Nokia SR Linux                      |
| **Components**                | [Nokia SR Linux][srl]                                                |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 4 <br/>:fontawesome-solid-memory: 8 GB |
| **Topology file**             | [clos02.yml][topofile]                                               |
| **Prefix**                    | clos02                                                               |

## Description
This labs provides a lightweight folded 5-stage CLOS fabric with Super Spine level bridging two PODs.

<center><div class="mxgraph" style="max-width:100%;border:1px solid transparent;" data-mxgraph="{&quot;page&quot;:8,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/containerlab-diagrams/main/containerlab.drawio&quot;}"></div></center>

The topology is additionally equipped with the Linux containers connected to leaves to facilitate use cases which require access side emulation.

## Use cases
With this lightweight CLOS topology a user can exhibit the following scenarios:

* perform configuration tasks applied to the 5-stage CLOS fabric
* demonstrate fabric behavior leveraging the user-emulating linux containers attached to the leaves

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-wim/container-lab/tree/master/lab-examples/clos02/clos02.yaml

[^1]: Resource requirements are provisional. Consult with SR Linux Software Installation guide for additional information.

<script type="text/javascript" src="https://viewer.diagrams.net/embed2.js?&fetch=https%3A%2F%2Fraw.githubusercontent.com%2Fsrl-wim%2Fcontainerlab-diagrams%2Fmain%2Fcontainerlab.drawio" async></script>