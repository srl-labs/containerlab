---
search:
  boost: 4
---
# SONiC

[SONiC](https://sonic-net.github.io/SONiC/) is identified with `sonic-vs` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a `sonic-vs` node.

!!!note
    To build a `sonic-vs` docker image:

    1. Leverage [automated scripts](https://github.com/antongisli/sonic-builder) provided by @antongisli
    2. or consult with the [SONiC build documentation](https://github.com/Azure/sonic-buildimage/blob/master/README.md#usage) and create the docker images with `PLATFORM=vs` yourself.


sonic-vs nodes launched with containerlab come without any additional configuration.

## Getting Sonic images
Apparently, it is still tricky to find sonic container images, as their prime distribution format is a VM. [Several discussions](https://github.com/srl-labs/containerlab/pull/545#issuecomment-998205602) happened around creating a container build pipeline and a [few projects](https://github.com/antongisli/sonic-builder) were created.

One of the latest known working images can be found [here](https://hub.docker.com/r/netreplica/docker-sonic-vs).

## Managing sonic-vs nodes
SONiC node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running sonic-vs container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the sonic-vs CLI (vtysh)
    ```bash
    docker exec -it <container-name/id> vtysh
    ```


## Interfaces mapping
sonic-vs container uses the following mapping for its linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data (front-panel port) interface

When containerlab launches sonic-vs node, it will assign IPv4/6 address to the `eth0` interface. Data interface `eth1` mapped to `Ethernet0` port and needs to be configured with IP addressing manually. See Lab examples for exact configurations.

## Lab examples
The following labs feature sonic-vs node:

- [SR Linux and sonic-vs](../../lab-examples/srl-sonic.md)
