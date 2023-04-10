# RARE/freeRtr

|                               |                                                                       |
| ----------------------------- | --------------------------------------------------------------------- |
| **Description**               | A 2-node network of RARE/freeRtr routers                              |
| **Components**                | [RARE/freeRtr](http://docs.freertr.org)                               |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 1 GB  |
| **Lab location**              | [rare-freertr/freeRtr-containerlab][repo]                             |
| **Version information**[^2]   | `containerlab:0.39.0`, `freertr-containerlab:latest`, `docker:23.0.1` |

## Description

[RARE](http://rare.freertr.org) stands for Router for Academia, Research & Education. It is an open source routing platform, used to create a network operating system (NOS) on commodity hardware. This lab example comprises two RARE/freeRtr routers connected via their respective `eth1` port.

Containerlab's support for RARE is detailed on the [Rare's kind page](../manual/kinds/rare-freertr.md).

## Lab instructions

The lab is documented in details in the [rare-freertr/freeRtr-containerlab](https://github.com/rare-freertr/freeRtr-containerlab/blob/main/README.md) repo.

There you'll find information such as:

* How to build `RARE/freeRtr containerlab` image
* How to launch `RARE/freeRtr Hello world` lab
* Lab configuration
* Lab verification

[repo]: https://github.com/rare-freertr/freeRtr-containerlab

[^1]: Resource requirements are provisional. Consult with docs for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
