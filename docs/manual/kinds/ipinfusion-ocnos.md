---
search:
  boost: 4
kind_code_name: ipinfusion_ocnos
kind_display_name: IPInfusion OcNOS
---
# -{{ kind_display_name }}-
-{{ kind_display_name }}- virtualized router is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).
It is built using [srl-labs/vrnetlab](https://github.com/srl-labs/vrnetlab/tree/master/ipinfusion/ocnos) and essentially is a Qemu VM packaged in a docker container format.


ipinfusion_ocnos nodes launched with containerlab come up pre-provisioned with SSH, and NETCONF services enabled.

/// note
OcNOS 7.0.0-262 (`OcNOS-SP-PLUS-x86-7.0.0-262-GA.qcow2`) builds and boots with `make docker-image` without modification.
Older images such as 6.0.2 needed a GRUB change for the serial console, as shown [here](https://github.com/srl-labs/vrnetlab/pull/99).
///

## Getting the image

The OcNOS VM image for KVM is a free download from IP Infusion (registration required):
[OcNOS-SP VM for KVM](https://www.ipinfusion.com/free-software-demos/?vm=KVM&utm_source=containerlab&utm_medium=docs&utm_campaign=ocnos-vm).
The VM includes a 365-day license and is licensed for non-production evaluation only ([OcNOS EULA](https://www.ipinfusion.com/eula/)).
Decompress the image with `xz -d`, then copy the `.qcow2` file into the `ipinfusion/ocnos` directory of
[srl-labs/vrnetlab](https://github.com/srl-labs/vrnetlab/tree/master/ipinfusion/ocnos) and run `make docker-image`.

## Managing ipinfusion_ocnos nodes

!!!note
    Containers with OcNOS inside will take ~3min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>` and `docker exec -it <container-name> tail -f /console.log`.

-{{ kind_display_name }}- node launched with containerlab can be managed via the following interfaces:

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
