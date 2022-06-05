# Juniper vMX

[Juniper vMX](https://www.juniper.net/us/en/products/routers/mx-series/vmx-virtual-router-software.html) virtualized router is identified with `vr-vmx` or `vr-juniper_vmx` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

vr-vmx nodes launched with containerlab comes up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## Managing vr-vmx nodes

!!!note
    Containers with vMX inside will take ~7min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Juniper vMX node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-vmx container:
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
vr-vmx container can have up to 90 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of vMX line card
* `eth2+` - second and subsequent data interface

When containerlab launches vr-vmx node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.


## Features and options
### Node configuration
vr-vmx nodes come up with a basic configuration where only the control plane and line cards are provisioned, as well as the `admin` users and management interfaces such as NETCONF, SNMP, gNMI.

#### Startup configuration
It is possible to make vMX nodes boot up with a user-defined startup-config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
topology:
  nodes:
    node:
      kind: vr-vmx
      startup-config: myconfig.txt
```

With this knob containerlab is instructed to take a file `myconfig.txt` from the directory that hosts the topology file, and copy it to the lab directory for that specific node under the `/config/startup-config.cfg` name. Then the directory that hosts the startup-config dir is mounted to the container. This will result in this config being applied at startup by the node.

Configuration is applied after the node is started, thus it can contain partial configuration snippets that you desire to add on top of the default config that a node boots up with.

## Lab examples
The following labs feature vr-vmx node:

- [SR Linux and Juniper vMX](../../lab-examples/vr-vmx.md)

## Known issues and limitations

* when listing docker containers, vr-vmx container will always report unhealthy status. Do not rely on this status.
* vMX requires Linux kernel 4.17+
* To check the boot log, use `docker logs -f <node-name>`.