# SONiC

[SONiC](https://azure.github.io/SONiC/) is identified with `sonic-vs` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a `sonic-vs` node.

!!!info
    `vs` in the name of a kind refers to a SONiC platform type.
    
!!!note
    To build a `sonic-vs` docker image, you will need to consult the [SONiC build documentation](https://github.com/Azure/sonic-buildimage/blob/master/README.md#usage) and build the docker images with `PLATFORM_ARCH=vs`

sonic-vs nodes launched with containerlab comes without any additional configuration.

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

When containerlab launches sonic-vs node, it will assign IPv4/6 address to the `eth0` interface. Data interface `eth1` which is mapped to `Ethernet0` port needs to be configured with IP addressing manually. See Lab examples for exact configurations.

## Lab examples
The following labs feature sonic-vs node:

- [SR Linux and sonic-vs](../../lab-examples/srl-sonic.md)
