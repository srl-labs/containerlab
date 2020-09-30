<p align=center><img src=https://gitlab.com/rdodin/pics/-/wikis/uploads/18b84497134ee39510d9daa6bc6712ad/containerlab_export.svg?sanitize=true/></p>

[![github release](https://img.shields.io/github/release/srl-wim/container-lab.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)
[![Github all releases](https://img.shields.io/github/downloads/srl-wim/container-lab/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)
[![build](https://img.shields.io/github/workflow/status/srl-wim/container-lab/Test/master?style=flat-square&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)

---

Containerlab provides a framework for setting up networking labs with containers. It builds a virtual wiring between the containers to create virtual topologies of users choice.

![clab-example-topos](https://gitlab.com/rdodin/pics/-/wikis/uploads/5a10778e2a10fa4ca581c7164b71175f/image.png)

Conainerlab focuses on _networking_ containers which are typically used to test network features and designs, such as:

* [Nokia SR-Linux](https://www.nokia.com/networks/products/service-router-linux-NOS/)
* [Arista cEOS](https://www.arista.com/en/products/software-controlled-container-networking)

Although, containerlab can wire up any typical linux container which can be used as the test clients.

### Features
* Labs can be wired to an external bridge to connect to an external environment or to build hierarchical labs.
* TLS certificates can be generated automatically for every node of a lab.
* The lab topology can be graphically visualized with [graphviz](https://graphviz.org) tool (WIP).
