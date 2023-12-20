---
search:
  boost: 4
---
# Juniper vMX

[Juniper vMX](https://www.juniper.net/us/en/products/routers/mx-series/vmx-virtual-router-software.html) virtualized router is identified with `juniper_vmx` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

Juniper vMX nodes launched with containerlab come up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## Managing Juniper vMX nodes

!!!note
    Containers with vMX inside will take ~7min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Juniper vMX node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running Juniper vMX container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the vMX CLI
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
    gnmic -a <container-name/node-mgmt-address> --insecure \
    -u admin -p admin@123 \
    capabilities
    ```

!!!info
    Default user credentials: `admin:admin@123`

## Interfaces mapping

Juniper vMX container can have up to 90 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to a first data port of vMX line card
* `eth2+` - second and subsequent data interface

When containerlab launches Juniper vMX node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach the management plane of the router.

Data interfaces `eth1+` need to be configured with IP addressing manually using CLI/management protocols.

## Features and options

### Node configuration

Juniper vMX nodes come up with a basic configuration where only the control plane and line cards are provisioned, as well as the `admin` users and management interfaces such as NETCONF, SNMP, gNMI.

Starting with [hellt/vrnetlab](https://github.com/hellt/vrnetlab) v0.8.2 VMX will make use of the management VRF[^1].

#### Startup configuration

It is possible to make vMX nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: juniper_vmx
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.

## Lab examples

The following labs feature Juniper vMX node:

* [SR Linux and Juniper vMX](../../lab-examples/vr-vmx.md)

## Known issues and limitations

* when listing docker containers, Juniper vMX containers will always report unhealthy status. Do not rely on this status.
* vMX requires Linux kernel 4.17+
* To check the boot log, use `docker logs -f <node-name>`.

[^1]: https://github.com/hellt/vrnetlab/pull/98
