---
search:
  boost: 4
---
# IPInfusion OcNOS

IPInfusion OcNOS virtualized router is identified with `ipinfusion_ocnos` kind in the [topology file](../topo-def-file.md). It is built using [srl-labs/vrnetlab](https://github.com/srl-labs/vrnetlab/tree/master/ipinfusion/ocnos) and essentially is a Qemu VM packaged in a docker container format.

ipinfusion_ocnos nodes launched with containerlab come up pre-provisioned with SSH, and NETCONF services enabled.

!!!warning
    OcNOS VM disk images need to be altered to support telnet serial access and ethX interfaces name style. This can be done by modifying the grub config file, as shown [here](https://github.com/srl-labs/vrnetlab/pull/99).

## Managing ipinfusion_ocnos nodes

!!!note
    Containers with OcNOS inside will take ~3min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>` and `docker exec -it <container-name> tail -f /console.log`.

IPInfusion OcNOS node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running ipinfusion_ocnos container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the OcNOS CLI
    ```bash
    ssh ocnos@<container-name/id>
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh ocnos@<container-name> -p 830 -s netconf
    ```

!!!info
    Default user credentials: `admin:admin@123`

## Interfaces mapping

ipinfusion_ocnos container can have up to 63 interfaces (eth management + 62 additional data interfaces defined in the topology file) and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of OcNOS line card
* `eth2+` - second and subsequent data interface

When containerlab launches ipinfusion_ocnos node, it will assign IPv4 address to the `eth0` interface. This address can be used to reach management plane of the router.

Data interfaces `eth1+` need to be configured with IP addressing manually using CLI/management protocols.
