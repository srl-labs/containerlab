<p align=center><a href="https://containerlab.srlinux.dev"><img src=https://gitlab.com/rdodin/pics/-/wikis/uploads/9f2e581a8d207a21ff024a312679a239/containerlab_export_white_ink_3.svg?sanitize=true/></a></p>

[![github release](https://img.shields.io/github/release/srl-labs/containerlab.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-labs/containerlab/releases/)
[![Github all releases](https://img.shields.io/github/downloads/srl-labs/containerlab/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-labs/containerlab/releases/)
[![Go Report](https://img.shields.io/badge/go%20report-A%2B-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://goreportcard.com/report/github.com/srl-labs/containerlab)
[![Doc](https://img.shields.io/badge/Docs-containerlab.srlinux.dev-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://containerlab.srlinux.dev)
[![build](https://img.shields.io/github/workflow/status/srl-labs/containerlab/Test/master?style=flat-square&labelColor=bec8d2)](https://github.com/srl-labs/containerlab/releases/)

---

With the growing number of containerized Network Operating Systems grows the demand to easily run them in the user-defined, versatile lab topologies.

Unfortunately, container orchestration tools like docker-compose are not a good fit for that purpose, as they do not allow a user to easily create connections between the containers which define a topology.

Containerlab provides a CLI for orchestrating and managing container-based networking labs. It starts the containers, builds a virtual wiring between them to create lab topologies of users choice and manages labs lifecycle.

![pic](https://gitlab.com/rdodin/pics/-/wikis/uploads/01fcdc212ee1c7de70ef5d2a8d109044/image.png)

Containerlab focuses on the containerized Network Operating Systems which are typically used to test network features and designs, such as:

* [Nokia SR-Linux](https://www.nokia.com/networks/products/service-router-linux-NOS/)
* [Arista cEOS](https://www.arista.com/en/products/software-controlled-container-networking)
* [Azure SONiC](https://azure.github.io/SONiC/)
* [Juniper cRPD](https://www.juniper.net/documentation/en_US/crpd/topics/concept/understanding-crpd.html)

In addition to native containerized NOSes, containerlab can launch traditional virtual-machine based routers using [vrnetlab integration](https://containerlab.srlinux.dev/manual/vrnetlab/):

* [Nokia virtual SR OS (vSim/VSR)](https://containerlab.srlinux.dev/manual/kinds/vr-sros/)
* [Juniper vMX](https://containerlab.srlinux.dev/manual/kinds/vr-vmx/)
* [Cisco IOS XRv9k](https://containerlab.srlinux.dev/manual/kinds/vr-xrv9k/)
* [Arista vEOS](https://containerlab.srlinux.dev/manual/kinds/vr-veos)

And, of course, containerlab is perfectly capable of wiring up arbitrary linux containers which can host your network applications, virtual functions or simply be a test client. With all that, containerlab provides a single IaaC interface to manage labs which can span contain all the needed variants of nodes:

<p align="center">
<img src="https://gitlab.com/rdodin/pics/-/wikis/uploads/bb8d9163f265dc827428097e6726d949/image.png" width="80%">
</p>

This short clip briefly demonstrates containerlab features and explains its purpose:

[![vid](https://gitlab.com/rdodin/pics/-/wikis/uploads/6fb7780687e329a830b8da7d115dfa65/image.png)](https://youtu.be/xdi7rwdJgkg)

## Features
* **IaaC approach**  
    Declarative way of defining the labs by means of the topology definition [`clab` files](https://containerlab.srlinux.dev/manual/topo-def-file/).
* **Network Operating Systems centric**  
    Focus on containerized Network Operating Systems. The sophisticated startup requirements of various NOS containers are abstracted with [kinds](https://containerlab.srlinux.dev/manual/kinds/kinds/) which allows the user to focus on the use cases, rather than infrastructure hurdles.
* **VM based nodes friendly**  
    With the [vrnetlab integration](https://containerlab.srlinux.dev/manual/vrnetlab) it is possible to get the best of two worlds - running virtualized and containerized nodes alike with the same IaaC approach and workflows.
* **Multi-vendor and open**  
    Although being kick-started by Nokia engineers, containerlab doesn't take sides and supports NOSes from other vendors and opensource projects.
* **Lab orchestration**  
    Starting the containers and interconnecting them alone is already good, but containerlab packages even more features like managing lab lifecycle: [deploy](https://containerlab.srlinux.dev/cmd/deploy), [destroy](https://containerlab.srlinux.dev/cmd/destroy), [save](https://containerlab.srlinux.dev/cmd/save), [inspect](https://containerlab.srlinux.dev/cmd/inspect), [graph](https://containerlab.srlinux.dev/cmd/graph) operations.
* **Scaled labs generator**  
    With [`generate`](https://containerlab.srlinux.dev/cmd/generate) capabilities of containerlab it possible to define/launch CLOS-based topologies of arbitrary scale. Just say how many tiers you need and how big each tier is, the rest will be done in a split second.
* **Simplicity and convenience**  
    Starting from frictionless [installation](https://containerlab.srlinux.dev/install/) and [upgrade](https://containerlab.srlinux.dev/install#upgrade) capabilities and ranging to the behind-the-scenes [link wiring machinery](https://containerlab.srlinux.dev/manual/network), containerlab does its best for you to enjoy the tool.
* **Fast**  
    Blazing fast way to create container based labs on any Linux system with Docker.
* **Automated TLS certificates provisioning**  
    The nodes which require TLS certs will get them automatically on boot.
* **Documentation is a first-class citizen**  
    We do not let our users guess by making a complete, concise and clean [documentation](https://containerlab.srlinux.dev).
* **Lab catalog**  
   The "most-wanted" lab topologies are [documented and included](https://containerlab.srlinux.dev/lab-examples/lab-examples/) with containerlab installation. Based on this cherry-picked selection you can start crafting the labs answering your needs.

## Use cases
* **Labs and demos**  
    Containerlab was meant to be a tool for provisioning networking labs built with containers. It is free, open and ubiquitous. No software apart from Docker is required!  
    As with any lab environment it allows the users to validate features, topologies, perform interop testing, datapath testing, etc.  
    It is also a perfect companion for your next demo. Deploy the lab fast, with all its configuration stored as a code -> destroy when done. Easily and [securely share lab access](https://containerlab.srlinux.dev/manual/published-ports) if needed.
* **Testing and CI**  
    Because of the containerlab's single-binary packaging and code-based lab definition files, it was never that easy to spin up a test bed for CI. Gitlab CI, Github Actions and virtually any CI system will be able to spin up containerlab topologies in a single simple command.
* **Telemetry validation**
    Coupling modern telemetry stacks with containerlab labs make a perfect fit for Telemetry use cases validation. Spin up a lab with containerized network functions with a telemetry on the side, and run comprehensive telemetry use cases.

Containerlab documentation is provided at https://containerlab.srlinux.dev.
