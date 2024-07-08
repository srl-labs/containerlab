---
search:
  boost: 4
kind_code_name: juniper_vjunosswitch
kind_display_name: Juniper vJunos-switch
---
# Juniper vJunos-switch

[Juniper vJunos-switch](https://www.juniper.net/documentation/product/us/en/vjunos-switch/) is a virtualized EX9214 switch identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

Juniper vJunos-switch nodes launched with containerlab come up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## How to obtain the image

The qcow2 image can be freely downloaded from the [Juniper support portal](https://support.juniper.net/support/downloads/?p=vjunos-switch) without a Juniper account and built with [vrnetlab](../vrnetlab.md).

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

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is: `et-0/0/X` (or `ge-0/0/X`, `xe-0/0/X`, all are accepted), where X denotes the port number.

With that naming convention in mind:

* `et-0/0/0` - first data port available
* `et-0/0/1` - second data port, and so on...

/// admonition
    type: note
Data port numbering starts at `0`.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

Juniper vJunosEvolved container can have up to 17 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to a first data port of vJunosEvolved VM, which is `et-0/0/0` **and not `et-0/0/1`**.
* `eth2+` - second and subsequent data interface

When containerlab launches [[[ kind_display_name ]]] node the management interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `et-0/0/0+` need to be configured with IP addressing manually using CLI or other available management interfaces.

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
