|                               |                                                                                          |
| ----------------------------- | ---------------------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Juniper vJunos-switch                                       |
| **Components**                | [Nokia SR Linux][srl], [Juniper vJunos-switch][vjunos]          |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 4 <br/>:fontawesome-solid-memory: 8 GB                     |
| **Topology file**             | [srlvjunos01.clab.yml][topofile]                                                            |
| **Name**                      | srlvjunos01                                                                                 |
| **Version information**[^2]   | `containerlab:0.45.0`, `srlinux:23.7.1`, `vjunos-switch:23.2R1.14`, `docker-ce:23.0.3` |

## Description

A lab consists of an SR Linux node connected with Juniper vJunos-switch via three point-to-point ethernet links. Both nodes are also connected with their management interfaces to the `clab` docker network.

## Use cases

The nodes are provisioned with a basic interface configuration for three interfaces they are connected with. Pings between the nodes should work out of the box using all three interfaces.

[srl]: ../manual/kinds/srl.md
[vjunos]: ../manual/kinds/vr-vjunosswitch.md
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/srlvjunos01/srlvjunos01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
