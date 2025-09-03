---
search:
  boost: 4
---
# Generic VM

Generic VM is identified with `generic_vm` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and offers containerlab users to launch arbitrary VMs that are packaged in a container using vrnetlab.

A typical use case for this kind is to launch a regular Linux VM such as Ubuntu, AlmaLinux, Redhat, etc. The term generic here means that containerlab does not provide any specific configuration for the VM, it just launches the VM and it is up to a user to confiugre it further.

## Generic VM images

To build a docker container for a generic VM you will need to download a `qcow2` VM image for your distribution.

For Ubuntu images use [srl-labs/vrnetlab/ubuntu](https://github.com/srl-labs/vrnetlab/tree/master/ubuntu) repository and the associated build instructions.

## Managing Linux VM nodes

/// note
Boot time depends on a linux distrubutive in use as well as the hardware resources.  
You can monitor the progress with `docker logs -f <container-name>`.

Ubuntu 22.04 takes about 1 minute to complete its start up.
///

A Linux VM node launched with containerlab can be managed via the following interfaces:

/// tab | bash
to connect to a `bash` shell of a running linux container:

```bash
docker exec -it <container-name/id> bash
```

///
/// tab | CLI via SSH
Connect to the VM Guest via SSH. Default password see below the [Credentials](#credentials).

```bash
ssh clab@<container-name>
```

///
/// tab | Telnet
serial port (console) is exposed over TCP port 5000:

```bash
# from container host
telnet <container-name> 5000
```

You can also connect to the container and use `telnet localhost 5000` if telnet is not available on your container host.
///

## Credentials

Default credentials for the Generic VM nodes are `clab:clab@123`.

## Interfaces mapping

* `eth0` - management interface (maps to `enp1s0` in the case of Ubuntu) connected to the containerlab management network. Should not be provisioned in the topology file as it is handled by containerlab.
* `eth1+` - second and subsequent interfaces. Only `ethX` interface names are allowed (where X > 0). These interfaces must be provided in the topology file's links section.

When containerlab launches a Linux VM node, it will assign IPv4/6 address to the `enp1s0` (or whatever the name is assigned to the management interface by the OS) interface which connects to the containerlab management network.

Data interfaces need to be configured with IP addressing manually using CLI.

## Features and options

### Node configuration

Linux vm nodes come up with a basic configuration where only the management interface and a default user are provisioned.

#### DNS

##### Ubuntu

The Ubuntu VM node comes with `9.9.9.9` configured as the DNS resolver. Change it with `resolvectl` if required.

## Lab examples

The following simple lab consists of two vr_linux hosts connected via one cEOS host:

* [generic_vm](../../lab-examples/generic_vm01.md)
