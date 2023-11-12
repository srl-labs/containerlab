---
search:
  boost: 4
---
# Cisco Nexus 9000v

Cisco Nexus9000v virtualized router is identified with `cisco_n9kv` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

Cisco Nexus 9000v nodes launched with containerlab comes up pre-provisioned with SSH, SNMP, NETCONF, NXAPI and gRPC services enabled.

## Managing Cisco Nexus 9000v nodes

!!!note
    Containers with Nexus 9000v inside will take ~8-10min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Cisco Nexus 9000v node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running Cisco Nexus 9000v container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the Nexus 9000v CLI
    ```bash
    ssh admin@<container-name/id>
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh admin@<container-name> -p 830 -s netconf
    ```
=== "gRPC"
    gRPC server is running over port 50051

!!!info
    Default user credentials: `admin:admin`

## Interfaces mapping

Cisco Nexus 9000v container can have up to 128 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of Nexus 9000v line card
* `eth2+` - second and subsequent data interface

When containerlab launches Cisco Nexus 9000v node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.

## Features and options

### Node configuration

Cisco Nexus 9000v nodes come up with a basic configuration where only `admin` user and management interfaces such as NETCONF, NXAPI and GRPC provisioned.

#### Startup configuration

It is possible to make n9kv nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: cisco_n9kv
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.
