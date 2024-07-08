---
search:
  boost: 4
kind_code_name: freebsd
kind_display_name: FreeBSD
---
# FreeBSD

[FreeBSD](https://freebsd.org/) is identified with `[[[kind_code_name]]]` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

## Getting FreeBSD image

To build FreeBSD docker container image you will need to download a custom-built `qcow2` VM image with pre-installed [cloud-init](https://cloudinit.readthedocs.io/en/latest/) from https://bsd-cloud-image.org/.

If, for some reason, you're unable to obtain an image from https://bsd-cloud-image.org/, you can build it yourself with the script from [this repository](https://github.com/goneri/pcib).

## Managing FreeBSD nodes

!!!note
    Containers with FreeBSD inside will take ~1-2 min to fully boot.  
    You can monitor the progress with `docker logs -f <container-name>`.

FreeBSD node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running FreeBSD container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the FreeBSD shell (password `admin`)
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

!!!info
    Default user credentials: `admin:admin`

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is: `vtnetX`, where `X` denotes the port number.

With that naming convention in mind:

* `vtnet1` - first data port available
* `vtnet2` - second data port, and so on...

/// admonition
    type: warning
Data port numbering starts at `1`, as `vtnet0` is reserved for management connectivity. Attempting to use `vtnet0` in a containerlab topology will result in an error.
///

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

* `eth0` - management interface connected to the containerlab management network (rendered as `vtnet0` in the CLI)
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `vtnet1`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `vtnet2` and so on)

When containerlab launches [[[ kind_display_name ]]] node the `vtnet0` interface of the VM gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `vtnet1+` need to be configured with IP addressing manually using CLI or other available management interfaces.

## Features and options

### Node configuration

FreeBSD nodes come up with a basic configuration where only the management interface and a default user are provisioned.

#### Configuration save

Containerlab's `save` command will perform a configuration backup for `FreeBSD` nodes via SCP. The entire `/etc` directory of each node will be archived and saved under `backup.tar.gz` file and can be found at the node's directory inside the lab parent directory:

```bash
# assuming the lab name is "freebsd01"
# and node name is "fbsd1"
ls clab-freebsd01/fbsd1/config/
backup.tar.gz
```

If the backup file is present upon the node's boot, it will be transferred to the node and extracted. The node will then reboot to apply the restored configuration.

## Lab examples

The following simple lab consists of two Linux hosts connected via one FreeBSD host:

* [FreeBSD](../../lab-examples/freebsd01.md)
