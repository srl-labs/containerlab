---
search:
  boost: 4
---
# Linux VM (vr_linux)

Generic linux VM is identified with `vr_linux` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

## Getting Linux image

To build a linux docker container you will need to download a `qcow2` VM image for your distribution.

For ubuntu 22.04 LTS (jammy) there exists a download script in the [vrnetlab](../vrnetlab.md) project.


## Managing Linux VM nodes

!!!note
    Containers with a Linux vm inside will take ~1-2 min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

A Linux VM node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running linux container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the Linux shell (password `sysadmin`)
    ```bash
    ssh sysadmin@<container-name>
    ```
=== "Telnet"
    serial port (console) is exposed over TCP port 5000:
    ```bash
    # from container host
    telnet <container-name> 5000
    ```
    You can also connect to the container and use `telnet localhost 5000` if telnet is not available on your container host.

!!!info
    Default user credentials: `admin:admin`

## Interfaces mapping

* `enp1s0` - management interface (vtnet0) connected to the containerlab management network
* `enp1s2+` - second and subsequent data interfaces (enp1s2, enp1s3, etc.)

When containerlab launches a Linux VM node, it will assign IPv4/6 address to the `enp1s0` interface. These addresses are used to reach the management plane of the router.

Data interfaces `enp1s2+` need to be configured with IP addressing manually using CLI.

## Features and options

### Node configuration

Linux vm nodes come up with a basic configuration where only the management interface and a default user are provisioned.

## Lab examples

The following simple lab consists of two vr_linux hosts connected via one cEOS host:

* [vr_linux](../../lab-examples/vr-linux01.md)
