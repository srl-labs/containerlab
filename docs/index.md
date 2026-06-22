---
template: home.html
hide:
  - navigation
  - toc
  - path
  - tags
tags:
  - Getting started
---

<section class="clab-hero" markdown>
<div class="clab-hero__content" markdown>
<p align=center><a href="https://containerlab.dev"><img src=images/containerlab_export_white_ink.svg?sanitize=true/></a></p>

<p class="clab-badges" markdown>
[![github release](https://img.shields.io/github/release/srl-labs/containerlab.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-labs/containerlab/releases/)
[![Github all releases](https://img.shields.io/github/downloads/srl-labs/containerlab/total.svg?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://github.com/srl-labs/containerlab/releases/)
[![DeepWiki](https://img.shields.io/badge/deepwiki-1DA1F2?logo=wikipedia&style=flat-square&color=00c9ff&labelColor=bec8d2&logoColor=black)](https://deepwiki.com/srl-labs/containerlab)
[![Bluesky](https://img.shields.io/badge/follow-containerlab-1DA1F2?logo=bluesky&style=flat-square&color=00c9ff&labelColor=bec8d2)](https://bsky.app/profile/containerlab.dev)
[![Discord](https://img.shields.io/discord/860500297297821756?style=flat-square&label=discord&logo=discord&color=00c9ff&labelColor=bec8d2)](https://discord.gg/vAyddtaEV9)
</p>

With the growing number of containerized Network Operating Systems grows the demand to run them in user-defined lab topologies.

Containerlab provides a CLI for orchestrating and managing container-based networking labs. It starts the containers, builds a virtual wiring between them to create lab topologies of users choice and manages labs lifecycle.
</div>
</section>

<section class="clab-section" markdown>
## Start here

<div class="grid cards clab-card-grid" markdown>

-   :material-rocket-launch-outline:{ .lg .middle } __Quickstart__

    ---

    Install containerlab, fetch a sample topology and deploy the lab.

    [:octicons-arrow-right-24: Quickstart](quickstart.md)

-   :material-file-code-outline:{ .lg .middle } __Topology definition__

    ---

    The topology file describes nodes, links and lab settings.

    [:octicons-arrow-right-24: Topology definition](manual/topo-def-file.md)

-   :material-cube-outline:{ .lg .middle } __Kinds__

    ---

    Kinds define how different NOSes, VMs, containers and services are started.

    [:octicons-arrow-right-24: Kinds](manual/kinds/index.md)

-   :material-flask-outline:{ .lg .middle } __Lab examples__

    ---

    Small lab topologies shipped with containerlab and documented in the catalog.

    [:octicons-arrow-right-24: Lab examples](lab-examples/lab-examples.md)

</div>
</section>

<section class="clab-section" markdown>
## Topology wiring

Container orchestration tools like docker-compose are not a good fit for this purpose, as they do not allow a user to easily create connections between the containers which define a topology.

<div class="clab-diagram" markdown>
<div class="mxgraph" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:2,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/index.md&quot;}"></div>
</div>
</section>

<section class="clab-section" markdown>
## Common workflows

<div class="grid cards clab-card-grid" markdown>

-   :material-console-line:{ .lg .middle } __Command reference__

    ---

    Commands for lab lifecycle, inspection, execution, graphing and tools.

    [:octicons-arrow-right-24: Commands](cmd/deploy.md)

-   :material-monitor-dashboard:{ .lg .middle } __GUI__

    ---

    VS Code extension, desktop application and web UI.

    [:octicons-arrow-right-24: GUI](manual/gui/index.md)

-   :material-lan:{ .lg .middle } __Networking__

    ---

    Management network, node links, bridges, host endpoints and link impairments.

    [:octicons-arrow-right-24: Network](manual/network.md)

-   :material-kubernetes:{ .lg .middle } __Clabernetes__

    ---

    Running containerlab-style topologies on Kubernetes.

    [:octicons-arrow-right-24: Clabernetes](manual/clabernetes/index.md)

</div>
</section>

<section class="clab-section" markdown>
## Supported platforms

Containerlab focuses on the containerized Network Operating Systems which are typically used to test network features and designs. It can also launch traditional virtual machine based routers using [vrnetlab or boxen integration](manual/vrnetlab.md), and it can wire arbitrary linux containers into a lab.

<div class="clab-diagram" markdown>
<div class="mxgraph" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/index.md&quot;}"></div>
</div>

<div class="grid cards clab-card-grid" markdown>

-   :simple-nokia:{ .lg .middle } __Nokia__

    ---

    SR Linux, SR OS SR-SIM and SR OS vSIM.

    [:octicons-arrow-right-24: Nokia SR Linux](manual/kinds/srl.md)

-   :simple-cisco:{ .lg .middle } __Cisco__

    ---

    XRd, XRv, XRv9k, CSR1000v, Nexus 9000v, Catalyst 9000v, IOL, ASAv and FTDv.

    [:octicons-arrow-right-24: Cisco XRd](manual/kinds/xrd.md)

-   :simple-junipernetworks:{ .lg .middle } __Juniper__

    ---

    cRPD, cSRX, vMX, vQFX, vSRX, vJunos-router, vJunos-switch and vJunosEvolved.

    [:octicons-arrow-right-24: Juniper cRPD](manual/kinds/crpd.md)

-   :material-plus-network:{ .lg .middle } __Other kinds__

    ---

    Arista, SONiC, Cumulus VX, VyOS, FRR, VPP, FreeBSD, OpenBSD, OpenWRT and more.

    [:octicons-arrow-right-24: All kinds](manual/kinds/index.md)

</div>
</section>

<section class="clab-section" markdown>
## Overview video

This short clip demonstrates containerlab features and explains its purpose:

<div class="iframe-container">
  <iframe type="text/html" src="https://www.youtube.com/embed/xdi7rwdJgkg" title="Containerlab overview" frameborder="0" allowfullscreen></iframe>
</div>
</section>

<section class="clab-section" markdown>
## Use cases

* **Labs and demos**  
    Containerlab can be used to provision networking labs built with containers. No software apart from Docker is required on the lab host.
* **Testing and CI**  
    Because of the containerlab's single-binary packaging and code-based lab definition files, CI systems can spin up containerlab topologies in a single command.
* **Telemetry validation**  
    Containerlab labs can be used together with telemetry stacks to validate collection, transport and visualization workflows.

## Join us

Have questions, ideas, bug reports or just want to chat? Come join [our discord server](https://discord.gg/vAyddtaEV9).
</section>
