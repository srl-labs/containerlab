|                               |                                                                                      |
| ----------------------------- | ------------------------------------------------------------------------------------ |
| **Description**               | A Nokia SR Linux connected back-to-back with Juniper vMX                             |
| **Components**                | [Nokia SR Linux][srl], [Juniper vMX][vmx]                                            |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 8 GB                 |
| **Topology file**             | [vr02.clab.yml][topofile]                                                            |
| **Name**                      | vr02                                                                                 |
| **Version information**[^2]   | `containerlab:0.9.0`, `srlinux:20.6.3-145`, `vr-vmx:20.2R1.10`, `docker-ce:19.03.13` |

## Description

A lab consists of an SR Linux node connected with Juniper vMX via a point-to-point ethernet link. Both nodes are also connected with their management interfaces to the `clab` docker network.

Juniper vMX VM is launched as a container, using [vrnetlab integration](../manual/vrnetlab.md).

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/vr02.drawio&quot;}"></div>

## Use cases

This lab allows users to launch basic interoperability scenarios between Nokia SR Linux and Juniper vMX network operating systems.

The lab directory [contains](https://github.com/srl-labs/containerlab/tree/main/lab-examples/vr02) files with essential configurations which can be used to jumpstart the interop demonstration.

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[vmx]: https://www.juniper.net/documentation/product/us/en/vmx/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/vr02/vr02.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
