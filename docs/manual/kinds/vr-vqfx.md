# Juniper vQFX

[Juniper vQFX](https://www.juniper.net/us/en/dm/free-vqfx10000-software.html) virtualized router is identified with `vr-vqfx` or `vr-juniper_vqfx` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

!!!note
    vQFX images built with [hellt/vrnetlab](https://github.com/hellt/vrnetlab/tree/master/vqfx) have experimental support for vQFX version v18 and newer.

## Managing vr-vqfx nodes

!!!note
    Containers with vQFX inside will take ~7min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Juniper vQFX node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-vqfx container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the vQFX CLI (password `admin@123`)
    ```bash
    ssh admin@<container-name/id>
    ```
=== "NETCONF"
    Coming soon

!!!info
    Default user credentials: `admin:admin@123`

## Interfaces mapping
* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of vQFX line card
* `eth2+` - second and subsequent data interface

When containerlab launches vr-vqfx node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.


## Features and options
### Node configuration
vr-vqfx nodes come up with a basic configuration where only the control plane and line cards are provisioned, as well as the `admin` user with the provided password.

#### Startup configuration
It is possible to make vQFX nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: vr-vqfx
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.

## Lab examples
Coming soon.