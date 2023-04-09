# RARE/freeRtr hello world !

|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | A 2-node network of RARE/freeRtr routers                            |
| **Components**                | [RARE/freeRtr](http://docs.freertr.org)             |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 1 GB  |
| **Topology file**             | [rtr000.clab.yml][topofile]                                           |
| **Name**                      | rtr000                                                               |
| **Version information**[^2]   | `containerlab:0.38.0`, `freertr-containerlab:latest`, `docker:23.0.1`  |

## Description

This lab example comprises two RARE/freeRtr routers connected via their respective `eth1` port.

Configuration details such as the bind mounts and access details are outlined on the [Rare's kind page](../manual/kinds/rare-freertr.md).

[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/rtr/000/rtr000.clab.yml

## Full lab instructions

The lab is documented in details [here](https://github.com/rare-freertr/freeRtr-containerlab/blob/main/README.md)

You'll find information such as:

* How to build `RARE/freeRtr containerlab` image
* How to lauch `RARE/freeRtr Hello world` lab
* Lab configuration
* Lab Verification

[^1]: Resource requirements are provisional. Consult with docs for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
