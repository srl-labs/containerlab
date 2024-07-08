---
search:
  boost: 4
kind_code_name: arista_veos
kind_display_name: Arista vEOS
---
# Arista vEOS

[Arista vEOS](https://www.arista.com/en/cg-veos-router/veos-router-overview) virtualized router is identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

Arista vEOS nodes launched with containerlab comes up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## Managing Arista vEOS nodes

!!!note
    Containers with vEOS inside will take ~4min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Arista vEOS node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running Arista vEOS container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the vEOS CLI
    ```bash
    ssh admin@<container-name/id>
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh admin@<container-name> -p 830 -s netconf
    ```
=== "gNMI"
    using the best in class [gnmic](https://gnmic.kmrd.dev) gNMI client as an example:
    ```bash
    gnmic -a <container-name/node-mgmt-address>:6030 --insecure \
    -u admin -p admin \
    capabilities
    ```
    Note, gNMI service runs over 6030 port.

!!!info
    Default user credentials: `admin:admin`

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is: `Ethernet1/X` (or `Et1/X`), where `X` is the port number.

With that naming convention in mind:

* `Ethernet1/1` - first data port available
* `Ethernet1/2` - second data port, and so on...

/// admonition
    type: note
Data port numbering starts at `1`.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `Ethernet1/1`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `Ethernet1/2` and so on)

When containerlab launches [[[ kind_display_name ]]] node the management interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `Ethernet1/1+` need to be configured with IP addressing manually using CLI or other available management interfaces.

## Features and options

### Node configuration

Arista vEOS nodes come up with a basic configuration where only the control plane and line cards are provisioned, as well as the `admin` user and management interfaces such as NETCONF, SNMP, gNMI.

#### Startup configuration

It is possible to make vEOS nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: arista_veos
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.
