# MikroTik RouterOS/Cloud-hosted router

[MikroTik RouterOS](https://mikrotik.com/download) cloud hosted router is identified with `vr-ros` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

## Managing vr-ros nodes

MikroTik RouterOS node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-ros container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the vEOS CLI
    ```bash
    ssh admin@<container-name/id>
    ```

!!!info
    Default user credentials: `admin:admin`

## Interfaces mapping
vr-ros container can have up to 30 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to the `ether2` interface of the RouterOS
* `eth2+` - second and subsequent data interface

When containerlab launches vr-ros node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.

