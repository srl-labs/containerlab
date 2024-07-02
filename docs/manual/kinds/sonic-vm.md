---
search:
  boost: 4
---
# SONiC (VM)

[SONiC](https://sonic-net.github.io/SONiC/) Network OS is distributed in two formats suitable for testing with containerlab

1. Containerized SONiC
2. Virtual Machine SONiC (topic of this document)

This document covers the containerized SONiC that is identified with `sonic-vs` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a `sonic-vs` node.

## Getting Sonic images

Getting SONiC images is possible via two resources:

1. [Sonic.software](https://sonic.software/) -- an unofficial repo with SONiC images
2. [Azure pipeline](https://dev.azure.com/mssonic/build/_build) -- an official source of SONiC images, but finding the right one there is a pita.

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
