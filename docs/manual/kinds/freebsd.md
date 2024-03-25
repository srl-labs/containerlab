---
search:
  boost: 4
---
# FreeBSD

[FreeBSD](https://freebsd.org/) is identified with `freebsd` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

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

## Interfaces mapping

* `eth0` - management interface (vtnet0) connected to the containerlab management network
* `eth1+` - second and subsequent data interfaces (vtnet1, vtnet2, etc.)

When containerlab launches FreeBSD node, it will assign IPv4/6 address to the `eth0` interface. These addresses are used to reach the management plane of the router.

Data interfaces `eth1+` need to be configured with IP addressing manually using CLI.

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
