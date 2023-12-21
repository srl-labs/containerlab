|                               |                                                                                          |
| ----------------------------- | ---------------------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Juniper vJunosEvolved                                       |
| **Components**                | [Nokia SR Linux][srl], [Juniper vJunosEvolved][vjunos]          |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 6 <br/>:fontawesome-solid-memory: 12 GB                     |
| **Topology file**             | [srlvjunos02.clab.yml][topofile]                                                            |
| **Name**                      | srlvjunos02                                                                                 |
| **Version information**[^2]   | `containerlab:0.49.0`, `srlinux:23.7.1`, `vJunosEvolved-23.2R1-S1.8`, `docker-ce:24.0.7,` |

## Description

A lab consists of an SR Linux node connected with Juniper vJunosEvolved via three point-to-point ethernet links. Both nodes are also connected with their management interfaces to the `clab` docker network.

## Use cases

The nodes are provisioned with a basic interface configuration for three interfaces they are connected with. Pings between the nodes should work out of the box using all three interfaces.

[srl]: ../manual/kinds/srl.md
[vjunos]: ../manual/kinds/vr-vjunosevolved.md
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/srlvjunos02/srlvjunos02.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
