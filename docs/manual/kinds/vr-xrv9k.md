# Cisco XRv9k

[Cisco XRv9k](https://www.cisco.com/c/en/us/products/collateral/routers/ios-xrv-9000-router/datasheet-c78-734034.html) virtualized router is identified with `vr-xrv9k` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

vr-xrv9k nodes launched with containerlab come up pre-provisioned with SSH, SNMP, NETCONF and gNMI (if available) services enabled.

!!!warning
    XRv9k node is a resource hungry image. As of XRv9k 7.2.1 version the minimum resources should be set to 2vcpu/14GB. These are the default setting set with containerlab for this kind.  
    Image will take 25 minutes to fully boot, be patient. You can monitor the loading status with `docker logs -f <container-name>`.

## Managing vr-xrv9k nodes
Cisco XRv9k node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-xrv9k container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the XRv9kCLI
    ```bash
    ssh clab@<container-name/id>
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
    -u clab -p clab@123 \
    capabilities
    ```

!!!info
    Default user credentials: `clab:clab@123`

## Interfaces mapping
vr-xrv9k container can have up to 90 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of XRv9k line card
* `eth2+` - second and subsequent data interface

When containerlab launches vr-xrv9k node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.


## Features and options
### Node configuration
vr-xrv9k nodes come up with a basic configuration where only the control plane and line cards are provisioned, as well as the `clab` user and management interfaces such as NETCONF, SNMP, gNMI.

## Lab examples
The following labs feature vr-xrv9k node:

- [SR Linux and Cisco XRv](../../lab-examples/vr-xrv9k.md)

## Known issues and limitations
* LACP and BPDU packets are not propagated to/from vrnetlab based routers launched with containerlab.
