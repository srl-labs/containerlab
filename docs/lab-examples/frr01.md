|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | A 3-node ring of FRR routers with OSPF IGP                           |
| **Components**                | [FRR](https://docs.frrouting.org/en/stable-7.5/overview.html)        |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB |
| **Topology file**             | [frr01.clab.yml][topofile]                                           |
| **Name**                      | frr01                                                                |
| **Version information**[^2]   | `containerlab:0.13.0`, `frrouting/frr:v7.5.1`, `docker-ce:19.03.13`  |

## Description

This lab example consists of three FRR routers connected in a ring topology. Each router has one PC connected to it.

This is also an example of how to pre-configure lab nodes of `linux` kind in containerlab.

To start this lab, run the [`run.sh`][run] script, which will run the containerlab deploy commands, and then configure the PC interfaces.

The lab configuration is documented in detail at: https://www.brianlinkletter.com/2021/05/use-containerlab-to-emulate-open-source-routers/

[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/frr01/frr01.clab.yml
[run]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/frr01/run.sh

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
