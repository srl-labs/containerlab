---
search:
  boost: 4
---
# Cisco FTDv

[Cisco FTDv](https://www.cisco.com/c/en/us/products/collateral/security/firepower-ngfw-virtual/threat-defense-virtual-ngfwv-ds.html) is identified with `cisco_ftdv` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

## Managing FTDv nodes

!!!note
    Containers with Cisco FTDv inside will take ~1-2 min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Cisco FTDv node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running FTDv container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the FTDv shell (password `Admin@123`)
    ```bash
    ssh admin@<container-name>
    ```
=== "Telnet"
    serial port (console) is exposed over TCP port 5000:
    ```bash
    # from container host
    telnet <container-name> 5000
    ```  
    You can also connect to the container and use `telnet localhost 5000` if telnet is not available on your container host.
=== "HTTPS"
    HTTPS server is running over port 443 -- connect with any browser normally.

!!!info
    Default user credentials: `admin:Admin@123`

## Interfaces mapping

* `eth0` - management interface (Management0/0) connected to the containerlab management network
* `eth1+` - first and subsequent data interfaces (GigabitEthernet0/0, GigabitEthernet0/1, etc.)

When containerlab launches FTDv node, it will assign IPv4/6 address to the `eth0` interface. These addresses are used to reach the management plane of the router.

Data interfaces `eth1+` need to be configured with IP addressing manually using Web UI.

## Features and options

### Node configuration

Cisco FTDv nodes come up with a basic configuration where only the management interface and a default user are provisioned.

Nodes are configured for local management with Firepower Device Management (FDM) On-Box management service. FDM is available via HTTPS and takes a few minutes to come up after node boot up.

## Lab examples

The following simple lab consists of two Linux hosts connected via one FTDv node:

* [Cisco FTDv](../../lab-examples/ftdv01.md)
