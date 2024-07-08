---
search:
  boost: 4
kind_code_name: cisco_csr1000v
kind_display_name: Cisco CSR1000v
---
# Cisco CSR1000v

Cisco CSR1000v virtualized router is identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

Cisco CSR1000v nodes launched with containerlab comes up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## Managing Cisco CSR1000v nodes

!!!note
    Containers with CSR1000v inside will take ~6min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Cisco CSR1000v node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running Cisco CSR1000v container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the CSR1000v CLI
    ```bash
    ssh admin@<container-name/id>
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh admin@<container-name> -p 830 -s netconf
    ```

!!!info
    Default user credentials: `admin:admin`

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is: `GigabitEthernetX` (or `GiX`), where `X` is the port number.

With that naming convention in mind:

* `Gi2` - first data port available
* `Gi3` - second data port, and so on...

/// admonition
    type: warning
Data port numbering starts at `2`, as `Gi1` is reserved for management connectivity. Attempting to use `Gi1` in a containerlab topology will result in an error.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

* `eth0` - management interface connected to the containerlab management network (rendered as `GigabitEthernet1` in the CLI)
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `GigabitEthernet2`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `GigabitEthernet3` and so on)

When containerlab launches [[[ kind_display_name ]]] node the `GigabitEthernet1` interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `GigabitEthernet2+` need to be configured with IP addressing manually using CLI or other available management interfaces.

## Features and options

### Node configuration

Cisco CSR1000v nodes come up with a basic configuration where only `admin` user and management interfaces such as NETCONF provisioned.

#### Startup configuration

It is possible to make CSR1000V nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: cisco_csr1000v
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.
