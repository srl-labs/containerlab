---
search:
  boost: 4
---
# SONiC (container)

[SONiC](https://sonic-net.github.io/SONiC/) Network OS is distributed in two formats suitable for testing with containerlab

1. Containerized SONiC (topic of this document)
2. Virtual Machine SONiC

This document covers the containerized SONiC that is identified with `sonic-vs` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a `sonic-vs` node.

## Getting Sonic images

Getting SONiC images is possible via two resources:

1. [Sonic.software](https://sonic.software/) -- an unofficial repo with SONiC images (may be down sometimes, uses Azure pipeline as a source)
2. [Azure pipeline](https://sonic-build.azurewebsites.net/ui/sonic/pipelines) -- an official source of SONiC images, but finding the right one there is a pita.

When https://sonic.software is down, you can follow the following procedure to find the SONiC image in the Azure pipeline artifacts maze:

1. Go to the piplines list: https://sonic-build.azurewebsites.net/ui/sonic/pipelines
2. Scroll all the way to the bottom where `vs` platform is listed
3. Pick a branch name that you want to use (e.g. `202405`) and click on the "Build History".
4. On the build history page choose the latest build that has succeeded (check the Result column) and click on the "Artifacts" link
5. In the new window, you will see a list with a single artifact, click on it
6. One more long scroll down until you see `target/docker-sonic-vs.gz` name (or Ctrl+F for it), click on it to start the download or copy the download link.
7. Here you go, you managed to download a SONiC image from a mysteriosly named branch for a build that probably means nothing to you. This Sonic experience for ya...

/// details | How to download SONiC image from Azure pipeline (video)
<video width="100%" controls>
  <source src="https://gitlab.com/rdodin/pics/-/wikis/uploads/054c60a0c8d685f826297c115470221b/sonic-dl.mp4" type="video/mp4">
</video>
///

## Managing sonic-vs nodes

SONiC node launched with containerlab can be managed via the following interfaces:

/// tab | bash
to connect to a `bash` shell of a running sonic-vs container:

```bash
docker exec -it <container-name/id> bash
```

///
/// tab | CLI
to connect to the sonic-vs CLI (vtysh)

```bash
docker exec -it <container-name/id> vtysh
```

///

## Interfaces mapping

sonic-vs container uses the following mapping for its linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data (front-panel port) interface

When containerlab launches sonic-vs node, it will assign IPv4/6 address to the `eth0` interface. Data interface `eth1` mapped to `Ethernet0` port and needs to be configured with IP addressing manually. See Lab examples for exact configurations.

## Lab examples

The following labs feature sonic-vs node:

* [SR Linux and sonic-vs](../../lab-examples/srl-sonic.md)
