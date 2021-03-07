# Arista vEOS

[Arista vEOS](https://www.arista.com/en/cg-veos-router/veos-router-overview) virtualized router is identified with `vr-veos` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

vr-veos nodes launched with containerlab comes up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## Managing vr-veos nodes

!!!note
    Containers with vEOS inside will take ~4min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Arista vEOS node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-veos container:
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
vr-veos container can have up to 144 interfaces and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of vEOS line card
* `eth2+` - second and subsequent data interface

When containerlab launches vr-veos node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.


## Features and options
### Node configuration
vr-veos nodes come up with a basic configuration where only the control plane and line cards are provisioned, as well as the `admin` user and management interfaces such as NETCONF, SNMP, gNMI.
