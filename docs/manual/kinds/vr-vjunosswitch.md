---
search:
  boost: 4
---
# Juniper vJunos-switch

[Juniper vJunos-switch](https://support.juniper.net/support/downloads/?p=vjunos) is a virtualized EX9214 switch identified with `juniper_vjunosswitch` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

Juniper vJunos-switch nodes launched with containerlab come up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## How to obtain the image

The qcow2 image can be downloaded from [Juniper website](https://support.juniper.net/support/downloads/?p=vjunos) and built with [vrnetlab](../vrnetlab.md).

## Managing Juniper vJunos-switch nodes

!!!note
    Containers with vJunos-switch inside will take ~15min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Juniper vJunos-switch node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running Juniper vJunos-switch container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the vJunos-switch CLI
    ```bash
    ssh admin@<container-name/id>
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh admin@<container-name> -p 830 -s netconf
    ```

!!!info
    Default user credentials: `admin:admin@123`

## Interfaces mapping

Juniper vJunos-switch container can have up to 11 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to a first data port of vJunos-Switch VM
* `eth2+` - second and subsequent data interface

When containerlab launches Juniper vJunos-switch node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach the management plane of the router.

Data interfaces `eth1+` need to be configured with IP addressing manually using CLI/management protocols or via a startup-config text file.

## Features and options

### Node configuration

Juniper vJunos-switch nodes come up with a basic configuration supplied by a mountable configuration disk to the main VM image. Users, management interfaces, and protocols such as SSH and NETCONF are configured.

#### Startup configuration

It is possible to make vJunos-switch nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: juniper_vjunosswitch
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.

## Lab examples

The following labs feature the Juniper vJunos-switch node:

* [SR Linux and Juniper vJunos-switch](../../lab-examples/srl-vjunos-switch.md)

## Known issues and limitations

* vJunos-switch requires Linux kernel 4.17+
* To check the boot log, use `docker logs -f <node-name>`.
