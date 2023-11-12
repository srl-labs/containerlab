---
search:
  boost: 4
---
# Dell FTOSv (OS10) / ftosv

Dell FTOSv (OS10) virtualized router/switch is identified with `dell_ftosv` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

Dell FTOSv nodes launched with containerlab comes up pre-provisioned with SSH and SNMP services enabled.

## Managing Dell FTOSv nodes

!!!note
    Containers with FTOS10v inside will take ~2-4min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Dell FTOS10v node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running Dell FTOSv container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the Dell FTOSv CLI
    ```bash
    ssh admin@<container-name/id>
    ```

!!!info
    Default user credentials: `admin:admin`

## Interfaces mapping

Dell FTOSv container can have different number of available interfaces which depends on platform used under FTOS10 virtualization .qcow2 disk and container image built using [vrnetlab](../vrnetlab.md) project. Interfaces uses the following mapping rules (in topology file):

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of FTOS10v line card
* `eth2+` - second and subsequent data interface

When containerlab launches Dell FTOSv node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.

## Features and options

### Node configuration

Dell FTOSv nodes come up with a basic configuration where only `admin` user and management interfaces such as SSH provisioned.

#### Startup configuration

It is possible to make vMX nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: dell_ftosv
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.
