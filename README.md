<a href="https://containerlab.srlinux.dev"><p align=center><img src=https://gitlab.com/rdodin/pics/-/wikis/uploads/18b84497134ee39510d9daa6bc6712ad/containerlab_export.svg?sanitize=true/></p></a>

[![github release](https://img.shields.io/github/release/srl-wim/container-lab.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)
[![Github all releases](https://img.shields.io/github/downloads/srl-wim/container-lab/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)
[![Go Report](https://img.shields.io/badge/go%20report-A%2B-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://goreportcard.com/report/github.com/srl-wim/container-lab)
[![Doc](https://img.shields.io/badge/Docs-containerlab.srlinux.dev-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://containerlab.srlinux.dev)
[![build](https://img.shields.io/github/workflow/status/srl-wim/container-lab/Test/master?style=flat-square&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)

---

## Description

Containerlab provides a framework for setting up and destroying labs for networking containers. It builds a virtual wiring using veth pairs between the containers to provide virtual topologies.

The labs could also be wired to an external bridge to connect to external environment or to build hierarchical labs.

A CA can be provided per lab and when enabled, containerlab generates certificates per device that can be used for various use cases, like GNMI, JSON RPC, etc. 

Lastly, containerlab also allows for a graphical output to validate the lab in a visual format using [graphviz](https://graphviz.org)

Containerlab supports the following containers:

* standard linux/alpine containers, typically used as test clients
* networking containers:
	* Nokia SR-Linux
	* Arista cEOS.

Containerlab documentation is provided at containerlab.srlinux.dev.
