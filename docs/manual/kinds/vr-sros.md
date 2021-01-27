# Nokia SR OS

[Nokia SR OS](https://www.juniper.net/documentation/us/en/software/vr-sros/vr-sros-deployment/topics/concept/understanding-vr-sros.html) virtualized router is identified with `vr-sros` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

vr-sros nodes launched with containerlab comes up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## Managing vr-sros nodes

!!!note
    Containers with SR OS inside will take ~3min to fully boot.  
    You can monitor the progress with `watch docker ps` waiting till the status will change to `healthy`.

Nokia SR OS node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-sros container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the SR OS CLI
    ```bash
    ssh admin@<container-name/id>
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh root@<container-name> -p 830 -s netconf
    ```
=== "gNMI"
    using the best in class [gnmic](https://gnmic.kmrd.dev) gNMI client as an example:
    ```bash
    gnmic -a <container-name/node-mgmt-address> --insecure \
    -u admin -p admin \
    capabilities
    ```

!!!info
    Default user credentials: `admin:admin`

## Interfaces mapping
vr-sros container uses the following mapping for its interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of SR OS line card
* `eth2+` - second and subsequent data interface

When containerlab launches vr-sros node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.


## Features and options
### Node configuration
vr-sros nodes come up with a basic "blank" configuration where only the card/mda are provisioned, as well as the management interfaces such as Netconf, SNMP, gNMI.

#### User defined config
It is possible to make SR OS nodes to boot up with a user-defined config instead of a built-in one. With a [`config`](../nodes.md#config) property of the node/kind a user sets the path to the config file that will be mounted to a container and used as a startup config:

```yaml
name: sros_lab
topology:
  nodes:
    sros:
      kind: vr-sros
      config: myconfig.txt
```

With such topology file containerlab is instructed to take a file `myconfig.txt` from the current working directory, copy it to the lab directory for that specific node under the `/tftpboot/config.txt` name and mount that dir to the container. This will result in this config to act as a startup config for the node.

### License
Path to a valid license must be provided for all vr-sros nodes with a [`license`](../nodes.md#license) directive.

### File mounts
When a user starts a lab, containerlab creates a node directory for storing [configuration artifacts](../conf-artifacts.md). For `vr-sros` kind containerlab creates `tftpboot` directory where the license file will be copied.

## Lab examples
The following labs feature vr-sros node:

- [SR Linux and vr-sros](../../lab-examples/vr-sros.md)