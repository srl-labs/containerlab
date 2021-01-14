<p align=center><img src=https://gitlab.com/rdodin/pics/-/wikis/uploads/18b84497134ee39510d9daa6bc6712ad/containerlab_export.svg?sanitize=true/></p>

[![github release](https://img.shields.io/github/release/srl-wim/container-lab.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)
[![Github all releases](https://img.shields.io/github/downloads/srl-wim/container-lab/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-wim/container-lab/releases/)

---

With the growing number of containerized Network Operating Systems grows the demand to easily run them in the user-defined, versatile lab topologies.
A distinctive requirement of a container-based lab is the need for point-to-point interfaces interconnecting the elements.

Unfortunately, container orchestration tools like docker/podman/etc are not a good fit for that purpose, as they do not allow a user to easily create such p2p connections between the containers.

Containerlab provides a framework for setting up networking labs with containers. It starts the containers and builds a virtual wiring between them to create lab topologies of users choice.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:2,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/container-lab/diagrams/containerlab.drawio&quot;}"></div>

Containerlab focuses on containerized Network Operating Systems which are typically used to test network features and designs, such as:

* [Nokia SR-Linux](https://www.nokia.com/networks/products/service-router-linux-NOS/)
* [Arista cEOS](https://www.arista.com/en/products/software-controlled-container-networking)
* [SONiC](https://azure.github.io/SONiC/)
* [Juniper cRPD](https://www.juniper.net/documentation/en_US/crpd/topics/concept/understanding-crpd.html)

But, of course, containerlab is perfectly capable of wiring up arbitrary containers which can host your network applications, virtual router or simply be a test client.
<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-wim/container-lab/diagrams/containerlab.drawio&quot;}"></div>

## Features
* **IaaC approach**  
    Declarative way of defining the labs by means of the [topology definition files](manual/topo-def-file.md).
* **Network Operating Systems centric**  
    Focus on containerized Network Operating Systems. The sophisticated startup requirements of various NOS containers are abstracted with [kinds](manual/kinds/kinds.md) which allows the user to focus on the use cases, rather than infrastructure.
* **Simplicity and convenience are keys**  
    One-click [installation](install.md) and upgrade capabilities.
* **Fast**  
    Blazing fast way to create container based labs on any Debian or RHEL system.
* **Automated TLS certificates provisioning**  
    The nodes which require TLS certs will get them automatically on start.
* **Documentation is a first-class citizen**  
    We do not let our users guess by making a complete, concise and clean [documentation](https://containerlab.srlinux.dev).
* **Lab catalog**  
   The "most-wanted" lab topologies are [documented and included](lab-examples/lab-examples.md) with containerlab installation. Based on this cherry-picked selection you can start crafting the labs answering your needs.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>
