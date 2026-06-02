---
search:
  boost: 4
kind_code_name: cisco_n9kv
kind_display_name: Cisco Nexus 9000v
---
# -{{ kind_display_name }}-

Cisco Nexus9000v virtualized router is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

Cisco Nexus 9000v nodes launched with containerlab comes up pre-provisioned with SSH, SNMP, NETCONF, NXAPI and gRPC services enabled.

/// details | N9kv Lite
If you have a Nexus 9000v Lightweight variant, you can use the same `-{{ kind_code_name }}-` to launch it

By default, Nexus 9kv image with require 10GB memory and 4 CPU. However `n9kv-lite` VM requires less resources, so you would want to tune the defaults down.

Following is sample for setting up lower memory and CPU for the `n9kv-lite`:

```yaml
topology:
  nodes:
    node:
      kind: -{{ kind_code_name }}-
      env:
        QEMU_MEMORY: 6144 # N9kv-lite requires minimum 6GB memory
        QEMU_SMP: 2 # N9kv-lite requires minimum 2 CPUs
```

Please refer to ['tuning qemu parameters'](../vrnetlab.md#tuning-qemu-parameters) section for more details.
///

## Managing -{{ kind_display_name }}- nodes

/// note
Containers with -{{ kind_display_name }}- inside will take ~5min to fully boot.  
You can monitor the progress with `docker logs -f <container-name>`.
///

-{{ kind_display_name }}- node launched with containerlab can be managed via the following interfaces:

/// tab | bash
to connect to a `bash` shell of a running -{{ kind_display_name }}- container:

```bash
docker exec -it <container-name/id> bash
```

///

/// tab | CLI
to connect to the -{{ kind_display_name }}- CLI

```bash
ssh admin@<container-name/id>
```

///

/// tab | NETCONF
NETCONF server is running over port 830

```bash
ssh admin@<container-name> -p 830 -s netconf
```

///

/// tab | gRPC
gRPC server is running over port 50051
///

## Credentials

Default user credentials: `admin:admin`

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in -{{ kind_display_name }}-.

The interface naming convention is: `Ethernet1/X` (or `Et1/X`), where `X` is the port number.

With that naming convention in mind:

* `Ethernet1/1` - first data port available
* `Ethernet1/2` - second data port, and so on...

/// admonition
    type: note
Data port numbering starts at `1`.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the -{{ kind_display_name }}- VM:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `Ethernet1/1`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `Ethernet1/2` and so on)

When containerlab launches -{{ kind_display_name }}- node the management interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the -{{ kind_display_name }}- using containerlab's assigned IP.

Data interfaces `Ethernet1/1+` need to be configured with IP addressing manually using CLI or other available management interfaces.

## Features and options

### Node configuration

-{{ kind_display_name }}- nodes come up with a basic configuration where only `admin` user and management interfaces such as NETCONF, NXAPI and GRPC provisioned.

#### Startup configuration

It is possible to make n9kv nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: -{{ kind_code_name }}-
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.
