---
search:
  boost: 4
---
# Juniper vSRX

[Juniper vSRX](https://www.juniper.net/us/en/dm/download-next-gen-vsrx-firewall-trial.html) virtualized firewall is identified with `vr-vsrx` or `vr-juniper_vsrx` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

## Managing vr-vsrx nodes

!!!note
    Containers with vSRX inside will take ~7min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Juniper vSRX node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-vsrx container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the vSRX CLI (password `admin@123`)
    ```bash
    ssh admin@<container-name/id>
    ```
=== "NETCONF"
    Coming soon

!!!info
    Default user credentials: `admin:admin@123`

## Interfaces mapping

* `eth0` - management interface (fxp0) connected to the containerlab management network
* `eth1+` - second and subsequent data interface

When containerlab launches vr-vsrx node, it will assign IPv4/6 address to the `eth0` interface. These addresses are used to reach the management plane of the router.

Data interfaces `eth1+` need to be configured with IP addressing manually using CLI/management protocols.

## Features and options

### Node configuration

`vr-vsrx` nodes come up with a basic configuration where only the control plane and line cards are provisioned and the `admin` user with the provided password.

#### Startup configuration

It is possible to make vSRX nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: vr-vsrx
      startup-config: myconfig.txt
```

With this knob, containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started. Thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.

## Lab examples

Coming soon.
