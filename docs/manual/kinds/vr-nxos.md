---
search:
  boost: 4
---
# Cisco NXOS

[Cisco NXOS](https://www.cisco.com/c/en/us/products/ios-nx-os-software/nx-os/index.html) virtual appliance is identified with `vr-nxos` or `vr-cisco_nxos` kind in the [topology file](../topo-def-file.md). It is built using [hellt/vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

!!!note
    This is a Titanium based system, which is an older version of NX-OS.

vr-nxos nodes launched with containerlab come up pre-provisioned with SSH service enabled.

## Managing vr-nxos nodes
Cisco NXOS node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-nxos container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the NX-OS CLI
    ```bash
    ssh clab@<container-name/id>
    ```


!!!info
    Default user credentials: `admin:admin`

## Interfaces mapping
vr-nxos container can have up to 90 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of NX-OS line card
* `eth2+` - second and subsequent data interface

When containerlab launches vr-nxos node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.


## Features and options
### Node configuration
vr-nxos nodes come up with a basic configuration where only the control plane and line cards are provisioned, as well as the `clab` user.


#### Startup configuration
It is possible to make NXOS nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: vr-nxos
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.
