---
search:
  boost: 4
kind_code_name: paloalto_panos
kind_display_name: Cisco Nexus9000v
---
# Palo Alto PA-VM

Palo Alto PA-VM virtualized firewall is identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md). It is built using [boxen](https://github.com/carlmontanari/boxen/) project and essentially is a Qemu VM packaged in a docker container format.

Palo Alto PA-VM nodes launched with containerlab come up pre-provisioned with SSH, and HTTPS services enabled.

## Managing Palo Alto PA-VM nodes

!!!note
    Containers with Palo Alto PA-VM inside will take ~8min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

Palo Alto PA-VM node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running Palo Alto PA-VM container:
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

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is: `Ethernet1/X`, where `X` is the port number.

With that naming convention in mind:

* `Ethernet1/1` - first data port available
* `Ethernet1/2` - second data port, and so on...

/// admonition
    type: note
Data port numbering starts at `1`.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `Ethernet1/1`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `Ethernet1/2` and so on)

When containerlab launches [[[ kind_display_name ]]] node the management interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `Ethernet1/1+` need to be configured with IP addressing manually using CLI or other available management interfaces.

/// note
Palo Alto PA-VM container supports up to 24 interfaces (plus mgmt).

Interfaces will *not* show up in the cli (`show interfaces all`) until some configuration is made to the interface!
///

## Features and options

### Node configuration

Palo Alto PA-VM nodes come up with a basic configuration where only `admin` user and management interface is provisioned.

### User defined config

It is possible to make Palo Alto PA-VM nodes to boot up with a user-defined config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property a user sets the path to the config file that will be mounted to a container and used as a startup config:

```yaml
name: lab
topology:
  nodes:
    ceos:
      kind: paloalto_panos
      startup-config: myconfig.conf
```
