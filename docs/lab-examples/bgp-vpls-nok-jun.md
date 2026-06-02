|                               |                                                                                                     |
| ----------------------------- | --------------------------------------------------------------------------------------------------- |
| **Description**               | BGP VPLS between Nokia SR OS and Juniper vMX                                                        |
| **Components**                | Nokia SR OS, Juniper vMX                                                                            |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 7-10 GB                             |
| **Lab location**              | :material-github: [hellt/bgp-vpls-lab](https://github.com/hellt/bgp-vpls-lab)                       |
| **Topology file**             | [vpls.clab.yml][topofile]                                                                           |
| **Version information**[^2]   | `containerlab:0.10.1`, `vr-sros:20.10.R1`, `vr-vmx:20.4R1.12`, `docker-ce:19.03.13`, `vrnetlab`[^3] |

## Description

This lab demonstrates how containerlab can be used in a classical networking labs where the prime focus is not on the containerized NOS, but on a classic VM-based routers.

The topology created in this lab matches the network used in the [BGP VPLS Deep Dive](https://netdevops.me/2016/bgp-vpls-deep-dive-nokia-sr-os--juniper/) article:

![topo](https://img-fotki.yandex.ru/get/194989/21639405.11d/0_8b222_20c181b9_orig.png)

It allows readers to follow through the article with the author and create BGP VPLS service between the Nokia and Juniper routers using [configuration snippets](https://github.com/hellt/bgp-vpls-lab/tree/master/configs) provided within the lab repository.

As the article was done before Nokia introduced MD-CLI, the configuration snippets for SR OS were translated to MD-CLI.

## Quickstart

1. Ensure that your host supports virtualization and/or nested virtualization in case of a VM.
2. [Install](../install.md)[^4] containerlab.
3. Build if needed, vrnetlab container images for the routers used in the lab.
4. Clone [lab repository](https://github.com/hellt/bgp-vpls-lab).
5. Deploy the lab topology `clab dep -t vpls.clab.yml`

[topofile]: https://github.com/hellt/bgp-vpls-lab/blob/master/vpls.clab.yml
[^1]: Resource requirements are provisional. Consult with the installation guides for additional information. Memory deduplication techniques like [UKSM](https://netdevops.me/2021/how-to-patch-ubuntu-2004-focal-fossa-with-uksm/) might help with RAM consumption.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
[^3]: Router images are built with vrnetlab [aebe377](https://github.com/srl-labs/vrnetlab/tree/aebe377f07da9497b1af82c081ca7ff5b072c3f4). To reproduce the image, checkout to this commit and build the relevant images. Note, that you might need to use containerlab of the version that is stated in the description.
[^4]: If installing the latest containerlab, make sure to use the latest [srl-labs/vrnetlab](https://github.com/srl-labs/vrnetlab) project as well, as there might have been changes with the integration. If unsure, install the containerlab version that is specified in the lab description.
