|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | Cumulus Linux Test Drive                                             |
| **Components**                | [Cumulus Linux][cvx]                                                 |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB |
| **Topology file**             | [lab-start.clab.yml][topofile] <br/>[lab-final.clab.yml][finalfile]  |
| **Name**                      | cvx03                                                                |
| **Version information**[^2]   | `cvx:4.3.0` `docker-ce:19.03.13`                                     |

## Description
The lab is a 5-node topology with 2 servers attached to a pair of leaf switches and a single spine switch. 

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/cvx.drawio&quot;}"></div>

## Use cases
This is a "Cumulus Test Drive" topology designed to provide an overview of NVIDIA Cumulus Linux. It can be used together with a series of [self-paced hands-on labs](https://resource.nvidia.com/en-us-linux-lab-guide/linux-lab-guide):

1. Interface Configuration (Lab2) -- learn how to configure L2 (access, trunk, LAG) and L3 (SVI and VRR) interfaces.
2. BGP Unnumbered (Lab3) -- learn how to configure BGP unnumbered between leaf and spine switches and advertise locally connected interfaces.

!!!note
    Everything from Lab1 is already pre-configured when the topology is created with [lab-start.clab.yml][topofile].

Additionally, the lab directory contains a [lab-final.clab.yml][finalfile] which will load final configurations as they appear at the end of Lab3.

[cvx]: https://www.nvidia.com/en-gb/networking/ethernet-switching/cumulus-vx/
[topofile]: https://github.com/srl-labs/containerlab/tree/master/lab-examples/cvx03/lab-start.clab.yml
[finalfile]: https://github.com/srl-labs/containerlab/tree/master/lab-examples/cvx03/lab-final.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>