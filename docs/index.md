<p align=center><object type="image/svg+xml" data=https://cdn.jsdelivr.net/gh/srl-labs/containerlab@master/docs/images/containerlab_export_white_ink_js.svg ></object></p>

[![github release](https://img.shields.io/github/release/srl-labs/containerlab.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-labs/containerlab/releases/)
[![Github all releases](https://img.shields.io/github/downloads/srl-labs/containerlab/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-labs/containerlab/releases/)

---

With the growing number of containerized Network Operating Systems grows the demand to easily run them in the user-defined, versatile lab topologies.

Unfortunately, container orchestration tools like docker-compose are not a good fit for that purpose, as they do not allow a user to easily create connections between the containers which define a topology.

Containerlab provides a CLI for orchestrating and managing container-based networking labs. It starts the containers, builds a virtual wiring between them to create lab topologies of users choice and manages labs lifecycle.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:2,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/index.md&quot;}"></div>

Containerlab focuses on the containerized Network Operating Systems which are typically used to test network features and designs, such as:

* [Nokia SR-Linux](https://www.nokia.com/networks/products/service-router-linux-NOS/)
* [Arista cEOS](https://www.arista.com/en/products/software-controlled-container-networking)
* [Azure SONiC](https://azure.github.io/SONiC/)
* [Juniper cRPD](https://www.juniper.net/documentation/en_US/crpd/topics/concept/understanding-crpd.html)
* [Cumulus VX](https://docs.nvidia.com/networking-ethernet-software/cumulus-vx/)

In addition to native containerized NOSes, containerlab can launch traditional virtual-machine based routers using [vrnetlab integration](manual/vrnetlab.md):

* [Nokia virtual SR OS (vSim/VSR)](manual/kinds/vr-sros.md)
* [Juniper vMX](manual/kinds/vr-vmx.md)
* [Cisco IOS XRv9k](manual/kinds/vr-xrv9k.md)
* [Arista vEOS](manual/kinds/vr-veos.md)

And, of course, containerlab is perfectly capable of wiring up arbitrary linux containers which can host your network applications, virtual functions or simply be a test client. With all that, containerlab provides a single IaaC interface to manage labs which can span all the needed variants of nodes:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/index.md&quot;}"></div>

This short clip briefly demonstrates containerlab features and explains its purpose:

<iframe type="text/html"
    width="100%"
    height="465"
    src="https://www.youtube.com/embed/xdi7rwdJgkg"
    frameborder="0">
</iframe>

## Features
* **IaaC approach**  
    Declarative way of defining the labs by means of the topology definition [`clab` files](manual/topo-def-file.md).
* **Network Operating Systems centric**  
    Focus on containerized Network Operating Systems. The sophisticated startup requirements of various NOS containers are abstracted with [kinds](manual/kinds/kinds.md) which allows the user to focus on the use cases, rather than infrastructure hurdles.
* **VM based nodes friendly**  
    With the [vrnetlab integration](manual/vrnetlab.md) it is possible to get the best of two worlds - running virtualized and containerized nodes alike with the same IaaC approach and workflows.
* **Multi-vendor and open**  
    Although being kick-started by Nokia engineers, containerlab doesn't take sides and supports NOSes from other vendors and opensource projects.
* **Lab orchestration**  
    Starting the containers and interconnecting them alone is already good, but containerlab packages even more features like managing lab lifecycle: [deploy](cmd/deploy.md), [destroy](cmd/destroy.md), [save](cmd/save.md), [inspect](cmd/inspect.md), [graph](cmd/graph.md) operations.
* **Scaled labs generator**  
    With [`generate`](cmd/generate.md) capabilities of containerlab it possible to define/launch CLOS-based topologies of arbitrary scale. Just say how many tiers you need and how big each tier is, the rest will be done in a split second.
* **Simplicity and convenience**  
    Starting from frictionless [installation](install.md) and [upgrade](install.md#upgrade) capabilities and ranging to the behind-the-scenes [link wiring machinery](manual/network.md), containerlab does its best for you to enjoy the tool.
* **Fast**  
    Blazing fast way to create container based labs on any Linux system with Docker.
* **Automated TLS certificates provisioning**  
    The nodes which require TLS certs will get them automatically on boot.
* **Documentation is a first-class citizen**  
    We do not let our users guess by making a complete, concise and clean [documentation](https://containerlab.srlinux.dev).
* **Lab catalog**  
   The "most-wanted" lab topologies are [documented and included](lab-examples/lab-examples.md) with containerlab installation. Based on this cherry-picked selection you can start crafting the labs answering your needs.

## Use cases
* **Labs and demos**  
    Containerlab was meant to be a tool for provisioning networking labs built with containers. It is free, open and ubiquitous. No software apart from Docker is required!  
    As with any lab environment it allows the users to validate features, topologies, perform interop testing, datapath testing, etc.  
    It is also a perfect companion for your next demo. Deploy the lab fast, with all its configuration stored as a code -> destroy when done. Easily and [securely share lab access](manual/published-ports.md) if needed.
* **Testing and CI**  
    Because of the containerlab's single-binary packaging and code-based lab definition files, it was never that easy to spin up a test bed for CI. Gitlab CI, Github Actions and virtually any CI system will be able to spin up containerlab topologies in a single simple command.
* **Telemetry validation**
    Coupling modern telemetry stacks with containerlab labs make a perfect fit for Telemetry use cases validation. Spin up a lab with containerized network functions with a telemetry on the side, and run comprehensive telemetry use cases.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>
