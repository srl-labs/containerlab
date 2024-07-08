---
search:
  boost: 4
kind_code_name: cisco_ftdv
kind_display_name: Cisco FTDv
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

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is: `GigabitEthernet0/X` (or `GiX`), where `X` is the port number.

With that naming convention in mind:

* `Gi0` - first data port available
* `Gi1` - second data port, and so on...

/// note
Data port numbering starts at `0`.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

* `eth0` - management interface connected to the containerlab management network (rendered as `Management0/0` in the CLI)
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `GigabitEthernet0/0`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `GigabitEthernet0/1` and so on)

When containerlab launches [[[ kind_display_name ]]] node the `Management0/0` interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `GigabitEthernet2+` need to be configured with IP addressing manually using Web UI or other available management interfaces.

## Features and options

### Node configuration

Cisco FTDv nodes come up with a basic configuration where only the management interface and a default user are provisioned.

Nodes are configured for local management with Firepower Device Management (FDM) On-Box management service. FDM is available via HTTPS and takes a few minutes to come up after node boot up.

## Lab examples

The following simple lab consists of two Linux hosts connected via one FTDv node:

* [Cisco FTDv](../../lab-examples/ftdv01.md)
