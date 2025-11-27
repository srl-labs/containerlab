---
search:
  boost: 4
kind_code_name: cisco_asav
kind_display_name: Cisco ASAv
---

# Cisco ASAv

[Cisco ASAv](https://www.cisco.com/c/en/us/products/collateral/security/adaptive-security-virtual-appliance-asav/adapt-security-virtual-appliance-ds.html) is identified with `cisco_asav` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

## Managing ASAv nodes

/// note
Containers with Cisco ASAv inside will take ~5-7 min to fully boot.
You can monitor the progress with `docker logs -f <container-name>`.
///

To connect to a `bash` shell of a running ASAv container:

```bash
docker exec -it <container-name/id> bash
```

To connect to the ASAv CLI (password `CiscaoAsa1!`):

```bash
ssh admin@<container-name>
```

To connect to the serial port (console) exposed over TCP port 5000:

```bash
# from container host
telnet <container-name> 5000
```

You can also connect to the container and use `telnet localhost 5000` if telnet is not available on your container host.

/// note
Default user credentials: `admin:CiscoAsa1!`
///

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in -{{ kind_display_name }}-.

The interface naming convention is: `GigabitEthernet0/X` (or `Gi0/X`), where `X` is the port number.

With that naming convention in mind:

- `Gi0/0` - first data port available
- `Gi0/1` - second data port, and so on...

/// note
Data port numbering starts at `0`.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the -{{ kind_display_name }}- VM:

- `eth0` - management interface connected to the containerlab management network (rendered as `Management0/0` in the CLI)
- `eth1` - first data interface, mapped to the first data port of the VM (rendered as `GigabitEthernet0/0`)
- `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `GigabitEthernet0/1` and so on)

When containerlab launches -{{ kind_display_name }}- node the `Management0/0` interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the -{{ kind_display_name }}- using containerlab's assigned IP.

Data interfaces `GigabitEthernet0/0+` need to be configured with IP addressing manually using CLI or other available management interfaces.

## Features and options

### Node configuration

Cisco ASAv nodes come up with a basic configuration where only the management interface and default `admin` user are provisioned.

#### User defined startup config

It is possible to make ASAv nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    asav:
      kind: cisco_asav
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that contains the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.

## Lab examples

The following simple lab consists of two Linux hosts connected via one ASAv firewall node:

- [Cisco ASAv](../../lab-examples/asav01.md)
