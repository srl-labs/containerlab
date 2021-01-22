# Cisco XRv

[Cisco XRv](https://www.juniper.net/us/en/products-services/routing/mx-series/XRv/) virtualized router is identified with `vr-xrv` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

vr-xrv nodes launched with containerlab comes up pre-provisioned with SSH, SNMP, NETCONF and gNMI (if available) services enabled.

## Managing vr-xrv nodes

!!!note
    Containers with XRv inside will take ~5min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Cisco XRv node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-xrv container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the XRv CLI
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
    -u clab -p clab@123 \
    capabilities
    ```

!!!info
    Default user credentials: `clab:clab@123`

## Interfaces mapping
vr-xrv container can have up to 90 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of XRv line card
* `eth2+` - second and subsequent data interface

When containerlab launches vr-xrv node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.


## Features and options
### Node configuration
vr-xrv nodes come up with a basic configuration where only the control plane and line cards are provisioned, as well as the `clab` user and management interfaces such as NETCONF, SNMP, gNMI.

## Lab examples
The following labs feature vr-xrv node:

- [SR Linux and Cisco XRv](../../lab-examples/vr-xrv.md)

## Known issues and limitations
* LACP and BPDU packets are not propagated to/from vrnetlab based routers launched with containerlab.
