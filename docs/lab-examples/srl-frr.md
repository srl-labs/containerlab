|                               |                                                                                          |
| ----------------------------- | ---------------------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back FRR router                                       |
| **Components**                | [Nokia SR Linux][srl], [FRR](https://docs.frrouting.org/en/stable-7.5/overview.html)     |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB                     |
| **Topology file**             | [srlfrr01.clab.yml][topofile]                                                            |
| **Name**                      | srlfrr01                                                                                 |
| **Version information**[^2]   | `containerlab:0.9.0`, `srlinux:20.6.3-145`, `frrouting/frr:v7.5.0`, `docker-ce:19.03.13` |

## Description

A lab consists of an SR Linux node connected with FRR router via a point-to-point ethernet link. Both nodes are also connected with their management interfaces to the `clab` docker network.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:2,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlsonic01.drawio&quot;}"></div>

## Use cases

This lab allows users to launch basic control plane interoperability scenarios between Nokia SR Linux and FRR network operating systems.

The lab directory [contains](https://github.com/srl-labs/containerlab/tree/main/lab-examples/srlfrr01) files with essential configurations which can be used to jumpstart the interop demonstration. There you will find the config files to demonstrate a classic iBGP peering use case:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:3,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlsonic01.drawio&quot;}"></div>

- `daemons`: frr daemons config that is bind mounted to the frr container to trigger the start of the relevant FRR services
- `frr.cfg`: vtysh config lines to configure a basic iBGP peering
- `srl.cfg`: sr_cli config lines to configure a basic iBGP peering

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/srlfrr01/srlfrr01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
