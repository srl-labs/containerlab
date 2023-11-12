---
search:
  boost: 4
---
# Arista vEOS

[Arista vEOS](https://www.arista.com/en/cg-veos-router/veos-router-overview) virtualized router is identified with `arista_veos` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

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

## Interfaces mapping

Arista vEOS container can have up to 144 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of vEOS line card
* `eth2+` - second and subsequent data interface

When containerlab launches Arista vEOS node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.

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
