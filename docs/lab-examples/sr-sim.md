# SR-SIM
|                               |                                                                                  |
| ----------------------------- | -------------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Nokia SR OS                         |
| **Components**                | [Nokia SR Linux][srl], [Nokia SR OS][sros]                                       |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 5 GB             |
| **Topology file**             | [sros-srl.clab.yml][topofile]                                                        |
| **Name**                      | sr01                                                                             |
| **Version information**[^2]   | `containerlab:0.69`, `srlinux:25.7.1`, `nokia_srsim:25.10.R1`, `docker-ce:28.1.1` |

## Description
A lab consists of an SR Linux node connected with Nokia SR OS via a point-to-point ethernet link. Both nodes are also connected with their management interfaces to the `clab` docker network.

Nokia SR OS is launched as a container, using the SR-SIM image.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/sros-srl.drawio&quot;}"></div>

## Use cases
This lab allows users to launch basic interoperability scenarios between Nokia SR Linux and Nokia SR OS network operating systems.

The lab directory [contains](https://github.com/srl-labs/containerlab/tree/main/lab-examples/sr-sim) files with essential configurations which can be used to jumpstart the interop demonstration.

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[sros]: https://www.nokia.com/networks/products/service-router-operating-system/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/sr-sim/sros-srl.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>