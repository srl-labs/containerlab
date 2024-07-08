---
search:
  boost: 4
kind_code_name: mikrotik_ros
kind_display_name: MikroTik RouterOS
---
# MikroTik RouterOS Cloud-hosted router

[MikroTik RouterOS](https://mikrotik.com/download) cloud hosted router is identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

## Managing MikroTik RouterOS nodes

MikroTik RouterOS node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running MikroTik RouterOS container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the MikroTik RouterOS CLI
    ```bash
    ssh admin@<container-name/id>
    ```
=== "Telnet"
    serial port (console) is exposed over TCP port 5000:
    ```bash
    # from container host
    telnet <node-name> 5000
    ```  
    You can also connect to the container and use `telnet localhost 5000` if telnet is not available on your container host.

!!!info
    Default user credentials: `admin:admin`

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is: `etherX`, where `X` is the port number.

With that naming convention in mind:

* `ether2` - first data port available
* `ether3` - second data port, and so on...

/// admonition
    type: warning
Data port numbering starts at `2`, as `ether1` is reserved for management connectivity. Attempting to use `ether1` in a containerlab topology will result in an error.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

* `eth0` - management interface connected to the containerlab management network (rendered as `ether1`)
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `ether2`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `ether3` and so on)

When containerlab launches [[[ kind_display_name ]]] node the management interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `ether2+` need to be configured with IP addressing manually using CLI or other available management interfaces.

### Node configuration

MikroTik RouterOS nodes come up with a basic "blank" configuration where only the management interface and user is provisioned.

#### User defined config

It is possible to make ROS nodes to boot up with a user-defined startup config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind a user sets the path to the config file that will be mounted to a container and used as a startup config:

```yaml
name: ros_lab
topology:
  nodes:
    ros:
      kind: mikrotik_ros
      startup-config: myconfig.txt
```

With such topology file containerlab is instructed to take a file `myconfig.txt` from the current working directory, copy it to the lab directory for that specific node under the `/ftpboot/config.auto.rsc` name and mount that dir to the container. This will result in this config to act as a startup config for the node via FTP. Mikrotik will automatically import any file with the .auto.rsc suffix.

### File mounts

When a user starts a lab, containerlab creates a node directory for storing [configuration artifacts](../conf-artifacts.md). For MikroTik RouterOS kind containerlab creates `ftpboot` directory where the config file will be copied as config.auto.rsc.
