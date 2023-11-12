---
search:
  boost: 4
---
# MikroTik RouterOS Cloud-hosted router

[MikroTik RouterOS](https://mikrotik.com/download) cloud hosted router is identified with `mikrotik_ros` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

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

## Interfaces mapping

MikroTik RouterOS container can have up to 30 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to the `ether2` interface of the RouterOS
* `eth2+` - second and subsequent data interface

When containerlab launches MikroTik RouterOS node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.

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
