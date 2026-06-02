---
search:
  boost: 4
kind_code_name: huawei_vrp
kind_display_name: Huawei VRP
---
# -{{ kind_display_name }}-

-{{ kind_display_name }}- virtualized router is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

-{{ kind_display_name }}- currently supports Huawei N40e and CE12800 variants, the same kind value - `-{{ kind_code_name }}-` - is used for both.

-{{ kind_display_name }}- nodes launched with containerlab comes up pre-provisioned with SSH, NETCONF services enabled.

## Managing -{{ kind_display_name }}- nodes

/// note
Containers with -{{ kind_display_name }}- inside will take ~3min to fully boot without a startup config file. And ~5-7 minute if the startup config file is provided, since a node will undergo a reboot.  
You can monitor the progress with `docker logs -f <container-name>`.
///

-{{ kind_display_name }}- node launched with containerlab can be managed via the following interfaces:

/// tab | CLI
to connect to the -{{ kind_display_name }}- CLI

```bash
ssh admin@<container-name/id>
```

///
/// tab | bash
to connect to a `bash` shell of a running -{{ kind_display_name }}- container:

```bash
docker exec -it <container-name/id> bash
```

///

/// tab | NETCONF
NETCONF server is running over port 830

```bash
ssh admin@<container-name> -p 830 -s netconf
```

///

## Credentials

Default user credentials: `admin:admin`

## Interface naming

The example ports above would be mapped to the following Linux interfaces inside the container running the -{{ kind_display_name }}- VM:

* `eth0` - management interface connected to the containerlab management network (rendered as `GigabitEthernet0/0/0` in the VRP config)
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `Ethernet1/0/0`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `Ethernet1/0/1` and so on)

When containerlab launches -{{ kind_display_name }}- node the management interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the -{{ kind_display_name }}- using containerlab's assigned IP.

Data interfaces `Ethernet1/0/0+` need to be configured with IP addressing manually using CLI or other available management interfaces.

## Features and options

### Node configuration

-{{ kind_display_name }}- nodes come up with a basic configuration where only `admin` user and management interfaces such as SSH and NETCONF provisioned.

#### Startup configuration

It is possible to make -{{ kind_display_name }}- nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: -{{ kind_code_name }}-
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain both partial configuration snippets that you desire to add on top of the default config that a node boots up with as well as the full configuration extracted from the VRP.

When startup config is provided the node will undergo a reboot cycle after applying the bootstrap config, thus the startup time will be twice as long as the node boots up without a config.
