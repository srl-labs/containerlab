<p align=center><a href="https://containerlab.srlinux.dev"><img src=https://gitlab.com/rdodin/pics/-/wikis/uploads/18b84497134ee39510d9daa6bc6712ad/containerlab_export.svg?sanitize=true/></a></p>

[![github release](https://img.shields.io/github/release/srl-wim/container-lab.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)
[![Github all releases](https://img.shields.io/github/downloads/srl-wim/container-lab/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)
[![Go Report](https://img.shields.io/badge/go%20report-A%2B-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://goreportcard.com/report/github.com/srl-wim/container-lab)
[![Doc](https://img.shields.io/badge/Docs-containerlab.srlinux.dev-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://containerlab.srlinux.dev)
[![build](https://img.shields.io/github/workflow/status/srl-wim/container-lab/Test/master?style=flat-square&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)

---

## Description

Containerlab provides a framework for setting up networking labs with containers. It starts the containers and builds a virtual wiring between them to create lab topologies of users choice.

![pic](https://gitlab.com/rdodin/pics/-/wikis/uploads/8244ceb188abd3831e3715c42d4fa38f/image.png)
Containerlab focuses on containerized Network Operating Systems which are typically used to test network features and designs, such as:

* [Nokia SR-Linux](https://www.nokia.com/networks/products/service-router-linux-NOS/)
* [Arista cEOS](https://www.arista.com/en/products/software-controlled-container-networking)
* [SONiC](https://azure.github.io/SONiC/)
* [Juniper cRPD](https://www.juniper.net/documentation/en_US/crpd/topics/concept/understanding-crpd.html)

But, of course, containerlab is perfectly capable of wiring up arbitrary containers which can host your network applications, virtual router or simply be a test client.

<p align="center">
<img src="https://gitlab.com/rdodin/pics/-/wikis/uploads/e9222468fe580bc57a9ff2da03cca1cb/image.png" width="40%">
</p>

## Features
* **IaaC approach**  
    Declarative way of defining the labs by means of the [topology definition files](https://containerlab.srlinux.dev/manual/topo-def-file/).
* **Network Operating Systems centric**  
    Focus on containerized Network Operating Systems. The sophisticated startup requirements of various NOS containers are abstracted with [kinds](https://containerlab.srlinux.dev/manual/kinds/) which allows the user to focus on the use cases, rather than infrastructure.
* **Simplicity and convenience are keys**  
    One-click [installation](https://containerlab.srlinux.dev/install/) and upgrade capabilities.
* **Fast**  
    Blazing fast way to create container based labs on any Debian or RHEL system.
* **Automated TLS certificates provisioning**  
    The nodes which require TLS certs will get them automatically on start.
* **Documentation is a first-class citizen**  
    We do not let our users guess by making a complete, concise and clean [documentation](https://containerlab.srlinux.dev).
* **Lab catalog**  
   The "most-wanted" lab topologies are [documented and included](https://containerlab.srlinux.dev/lab-examples/lab-examples/) with containerlab installation. Based on this cherry-picked selection you can start crafting the labs answering your needs.


Containerlab documentation is provided at https://containerlab.srlinux.dev.
