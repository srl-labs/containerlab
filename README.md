<p align=center><a href="https://containerlab.dev"><img src=docs/images/containerlab_export_white_ink.svg?sanitize=true/></a></p>

[![github release](https://img.shields.io/github/release/srl-labs/containerlab.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-labs/containerlab/releases/)
[![Github all releases](https://img.shields.io/github/downloads/srl-labs/containerlab/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-labs/containerlab/releases/)
[![Doc](https://img.shields.io/badge/Docs-containerlab.dev-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://containerlab.dev)
[![Twitter](https://img.shields.io/badge/follow-%40go_containerlab-1DA1F2?logo=twitter&style=flat-square&color=00c9ff&labelColor=bec8d2)](https://twitter.com/go_containerlab)
[![Discord](https://img.shields.io/discord/860500297297821756?style=flat-square&label=discord&logo=discord&color=00c9ff&labelColor=bec8d2)](https://discord.gg/vAyddtaEV9)
[![Go Report](https://img.shields.io/badge/go%20report-A%2B-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://goreportcard.com/report/github.com/srl-labs/containerlab)

---

With the growing number of containerized Network Operating Systems grows the demand to easily run them in the user-defined, versatile lab topologies.

Unfortunately, container orchestration tools like docker-compose are not a good fit for that purpose, as they do not allow a user to easily create connections between the containers which define a topology.

Containerlab provides a CLI for orchestrating and managing container-based networking labs. It starts the containers, builds a virtual wiring between them to create lab topologies of users choice and manages labs lifecycle.

![pic](https://gitlab.com/rdodin/pics/-/wikis/uploads/01fcdc212ee1c7de70ef5d2a8d109044/image.png)

Containerlab focuses on the containerized Network Operating Systems which are typically used to test network features and designs, such as:

* [Nokia SR-Linux](https://containerlab.dev/manual/kinds/srl/)
* [Arista cEOS](https://containerlab.dev/manual/kinds/ceos/)
* [Cisco XRd](https://containerlab.dev/manual/kinds/xrd/)
* [Azure SONiC](https://containerlab.dev/manual/kinds/sonic-vs/)
* [Juniper cRPD](https://containerlab.dev/manual/kinds/crpd/)
* [Cumulus VX](https://containerlab.dev/manual/kinds/cvx/)
* [Keysight IXIA-C](https://containerlab.dev/manual/kinds/keysight_ixia-c-one/)
* [RARE/FreeRtr](https://containerlab.dev/manual/kinds/rare-freertr/)
* [Ostinato](https://containerlab.dev/manual/kinds/ostinato/)

In addition to native containerized NOSes, containerlab can launch traditional virtual machine based routers using [vrnetlab or boxen integration](https://containerlab.dev/manual/vrnetlab/):

* [Nokia virtual SR OS (vSim/VSR)](https://containerlab.dev/manual/kinds/vr-sros/)
* [Juniper vMX](https://containerlab.dev/manual/kinds/vr-vmx/)
* [Juniper vQFX](https://containerlab.dev/manual/kinds/vr-vqfx/)
* [Juniper vSRX](https://containerlab.dev/manual/kinds/vr-vsrx/)
* [Juniper vJunos-router](https://containerlab.dev/manual/kinds/vr-vjunosrouter/)
* [Juniper vJunos-switch](https://containerlab.dev/manual/kinds/vr-vjunosswitch/)
* [Juniper vJunosEvolved](https://containerlab.dev/manual/kinds/vr-vjunosevolved/)
* [Cisco IOS XRv9k](https://containerlab.dev/manual/kinds/vr-xrv9k/)
* [Cisco Nexus 9000v](https://containerlab.dev/manual/kinds/vr-n9kv)
* [Cisco c8000v](https://containerlab.dev/manual/kinds/vr-c8000v/)
* [Cisco CSR 1000v](https://containerlab.dev/manual/kinds/vr-csr)
* [Dell FTOS10v](https://containerlab.dev/manual/kinds/vr-ftosv)
* [Arista vEOS](https://containerlab.dev/manual/kinds/vr-veos)
* [Palo Alto PAN](https://containerlab.dev/manual/kinds/vr-pan)
* [IPInfusion OcNOS](https://containerlab.dev/manual/kinds/ipinfusion-ocnos)
* [Check Point Cloudguard](https://containerlab.dev/manual/kinds/checkpoint_cloudguard/)
* [Fortinet Fortigate](https://containerlab.dev/manual/kinds/fortinet_fortigate/)
* [Aruba AOS-CX](https://containerlab.dev/manual/kinds/vr-aoscx)
* [OpenBSD](https://containerlab.dev/manual/kinds/openbsd)
* [FreeBSD](https://containerlab.dev/manual/kinds/freebsd)

And, of course, containerlab is perfectly capable of wiring up arbitrary linux containers which can host your network applications, virtual functions or simply be a test client. With all that, containerlab provides a single IaaC interface to manage labs which can span contain all the needed variants of nodes:

<p align="center">
<img src="https://gitlab.com/rdodin/pics/-/wikis/uploads/bb8d9163f265dc827428097e6726d949/image.png" width="80%">
</p>

This short clip briefly demonstrates containerlab features and explains its purpose:

[![vid](https://gitlab.com/rdodin/pics/-/wikis/uploads/35d954fd81d9594ffa5b6110cbc950f5/clab-clip-stillshot.png)](https://youtu.be/xdi7rwdJgkg)

## Features

* **IaaC approach**  
    Declarative way of defining the labs by means of the topology definition [`clab` files](https://containerlab.dev/manual/topo-def-file/).
* **Network Operating Systems centric**  
    Focus on containerized Network Operating Systems. The sophisticated startup requirements of various NOS containers are abstracted with [kinds](https://containerlab.dev/manual/kinds/) which allows the user to focus on the use cases, rather than infrastructure hurdles.
* **VM based nodes friendly**  
    With the [vrnetlab integration](https://containerlab.dev/manual/vrnetlab) it is possible to get the best of two worlds - running virtualized and containerized nodes alike with the same IaaC approach and workflows.
* **Multi-vendor and open**  
    Although being kick-started by Nokia engineers, containerlab doesn't take sides and supports NOSes from other vendors and opensource projects.
* **Lab orchestration**  
    Starting the containers and interconnecting them alone is already good, but containerlab packages even more features like managing lab lifecycle: [deploy](https://containerlab.dev/cmd/deploy), [destroy](https://containerlab.dev/cmd/destroy), [save](https://containerlab.dev/cmd/save), [inspect](https://containerlab.dev/cmd/inspect), [graph](https://containerlab.dev/cmd/graph) operations.
* **Scaled labs generator**  
    With [`generate`](https://containerlab.dev/cmd/generate) capabilities of containerlab it possible to define/launch CLOS-based topologies of arbitrary scale. Just say how many tiers you need and how big each tier is, the rest will be done in a split second.
* **Simplicity and convenience**  
    Starting from frictionless [installation](https://containerlab.dev/install/) and [upgrade](https://containerlab.dev/install#upgrade) capabilities and ranging to the behind-the-scenes [link wiring machinery](https://containerlab.dev/manual/network), containerlab does its best for you to enjoy the tool.
* **Fast**  
    Blazing fast way to create container based labs on any Linux system with Docker.
* **Automated TLS certificates provisioning**  
    The nodes which require TLS certs will get them automatically on boot.
* **Documentation is a first-class citizen**  
    We do not let our users guess by making a complete, concise and clean [documentation](https://containerlab.dev).
* **Lab catalog**  
   The "most-wanted" lab topologies are [documented and included](https://containerlab.dev/lab-examples/lab-examples/) with containerlab installation. Based on this cherry-picked selection you can start crafting the labs answering your needs.

## Use cases

* **Labs and demos**  
    Containerlab was meant to be a tool for provisioning networking labs built with containers. It is free, open and ubiquitous. No software apart from Docker is required!  
    As with any lab environment it allows the users to validate features, topologies, perform interop testing, datapath testing, etc.  
    It is also a perfect companion for your next demo. Deploy the lab fast, with all its configuration stored as a code -> destroy when done. Easily and [securely share lab access](https://containerlab.dev/manual/published-ports) if needed.
* **Testing and CI**  
    Because of the containerlab's single-binary packaging and code-based lab definition files, it was never that easy to spin up a test bed for CI. Gitlab CI, Github Actions and virtually any CI system will be able to spin up containerlab topologies in a single simple command.
* **Telemetry validation**  
    Coupling modern telemetry stacks with containerlab labs make a perfect fit for Telemetry use cases validation. Spin up a lab with containerized network functions with a telemetry on the side, and run comprehensive telemetry use cases.

Containerlab documentation is provided at <https://containerlab.dev>.
