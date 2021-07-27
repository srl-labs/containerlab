# Palo Alto PA-VM

Palo Alto PA-VM virtualized firewall is identified with `vr-pan` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

vr-pan nodes launched with containerlab comes up pre-provisioned with SSH, and HTTPS services enabled.

## Managing vr-pan nodes

!!!note
    Containers with Palo Alto PA-VM inside will take ~8min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Palo Alto PA-VM node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running vr-pan container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the Palo Alto PA-VM CLI
    ```bash
    ssh admin@<container-name/id>
    ```
=== "HTTPS"
    HTTPS server is running over port 443 -- connect with any browser normally.

!!!info
    Default user credentials: `admin:Admin@123`

## Interfaces mapping
vr-pan container supports up to 24 interfaces (plus mgmt) and uses the following mapping rules:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to first data port of PAN VM
* `eth2+` - second and subsequent data interface

When containerlab launches vr-pan node, it will assign IPv4/6 address to the `mgmt` interface. These addresses can be used to reach management plane of the router.

Data interfaces `eth1+` needs to be configured with IP addressing manually using CLI/management protocols.

!!!info
    Interfaces will *not* show up in the cli (`show interfaces all`) until some configuration is made to the interface!


## Features and options
### Node configuration
vr-pan nodes come up with a basic configuration where only `admin` user and management interface is provisioned.
