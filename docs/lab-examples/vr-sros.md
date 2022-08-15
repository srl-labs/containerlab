|                               |                                                                                  |
| ----------------------------- | -------------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Nokia SR OS                         |
| **Components**                | [Nokia SR Linux][srl], [Nokia SR OS][sros]                                       |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 5 GB             |
| **Topology file**             | [vr01.clab.yml][topofile]                                                        |
| **Name**                      | vr01                                                                             |
| **Version information**[^2]   | `containerlab:0.27.1`, `srlinux:22.3.2`, `vr-sros:22.5.R1`, `docker-ce:19.03.13` |

## Description
A lab consists of an SR Linux node connected with Nokia SR OS via a point-to-point ethernet link. Both nodes are also connected with their management interfaces to the `clab` docker network.

Nokia SR OS VM is launched as a container, using [vrnetlab integration](../manual/vrnetlab.md).

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/vr01.drawio&quot;}"></div>

## Use cases
This lab allows users to launch basic interoperability scenarios between Nokia SR Linux and Nokia SR OS network operating systems.

The lab directory [contains](https://github.com/srl-labs/containerlab/tree/main/lab-examples/vr01) files with essential configurations which can be used to jumpstart the interop demonstration.

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[sros]: https://www.nokia.com/networks/products/service-router-operating-system/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/vr01/vr01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>