# Dell FTOSv (OS10) / ftosv

Dell FTOSv (OS10) virtualized router/switch is identified with `vr-ftosv` or `vr-dell_ftosv` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

vr-ftosv nodes launched with containerlab comes up pre-provisioned with SSH and SNMP services enabled.

## Managing vr-ftosv nodes

!!!note
    Containers with FTOS10v inside will take ~2-4min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Dell FTOS10v node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-ftosv container:
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
vr-ftosv container can have different number of available interfaces which depends on platform used under FTOS10 virtualization .qcow2 disk and container image built using [vrnetlab](../vrnetlab.md) project. Interfaces uses the following mapping rules (in topology file):

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of FTOS10v line card
* `eth2+` - second and subsequent data interface

When containerlab launches vr-ftosv node, it will assign IPv4/6 address to the `eth0` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.


## Features and options
### Node configuration
vr-ftosv nodes come up with a basic configuration where only `admin` user and management interfaces such as SSH provisioned.
