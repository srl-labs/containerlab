---
search:
  boost: 4
kind_code_name: cisco_viosl2
kind_display_name: Cisco vIOSL2
---
# Cisco vIOSL2

Cisco vIOSL2 virtualized layer-2 switch is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

## Managing Cisco vIOSL2 nodes

Cisco vIOSL2 node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running Cisco vIOSL2 container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the vIOSL2 CLI
    ```bash
    ssh cisco@<container-name/id>
    ```

!!!info
    Default user credentials: `cisco:cisco`

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in -{{ kind_display_name }}-.

The interface naming convention is: `GigabitEthernetX` (or `GiX`), where `X` is the port number.

With that naming convention in mind:

* `Gi0` - first data port available
* `Gi1` - second data port, and so on...

The example ports above would be mapped to the following Linux interfaces inside the container running the -{{ kind_display_name }}- VM:

* `eth0` - management interface connected to the containerlab management network (rendered as `GigabitEthernet0/0` in the CLI)
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `GigabitEthernet0`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `GigabitEthernet1` and so on)

When containerlab launches -{{ kind_display_name }}- node the management interface gets assigned an address from the containerlab management network.

Data interfaces `GigabitEthernet0+` need to be configured with IP addressing manually using CLI or other available management interfaces.

## Features and options

### Node configuration

Cisco vIOSL2 nodes come up with a basic configuration where only `cisco` user and management interface are provisioned.

#### Startup configuration

It is possible to make vIOSL2 nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: cisco_viosl2
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.
