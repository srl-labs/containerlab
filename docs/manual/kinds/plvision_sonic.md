---
search:
  boost: 4
kind_code_name: plvision_sonic
kind_display_name: "Enterprise SONiC Linux Distribution by PLVision"
---
# -{{ kind_display_name }}-
This document covers the VM flavor of [Enterprise SONiC Linux Distribution](https://plvision.eu/sonic-lite) from PLVision, identified with the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).

The image is built with the [`vrnetlab/plvision_sonic`](https://github.com/srl-labs/vrnetlab) vrnetlab project and packaged as a Qemu VM inside a Docker container.

## Getting -{{ kind_display_name }}- images

1. To download the image, log in to [Solutions: PLVision ServiceDesk](https://support.plvision.eu/support/solutions) and find the required version.
2. Extract the archive and place the `.img` file in the `-{{ kind_code_name }}-` directory of the vrnetlab repo.
3. Rename the file to `plvision_sonic-vs-<version>.qcow2` and run `make`.

After the build completes, the image will be available as `vrnetlab/plvision_sonic:<version>` (for example `vrnetlab/plvision_sonic:lite_1_13_202405`).

## System requirements

- CPU: 2 cores
- RAM: 4GB
- DISK: ~6.5GB

## Managing plvision_sonic nodes

A `-{{ kind_code_name }}-` node launched with containerlab takes approximately 1 minute to boot and can be managed via the following interfaces:

/// note
The default login credentials for the PLVision Enterprise SONiC VM are `admin:admin`
///

/// note | exec runs in the launcher container, not the SONiC VM
`-{{ kind_code_name }}-` is a VM-based (vrnetlab) kind, so the [`exec` node property](../nodes.md#exec) and the [`exec` command](../../cmd/exec.md) run inside the launcher container that wraps the VM, not inside the SONiC guest. SONiC CLI commands such as `show version` are not available via `exec` and fail with `executable file not found in $PATH`. Use the SSH method below to run commands against SONiC.
///

/// tab | SSH
To open a Linux shell, simply type in

```bash
ssh <node-name>
```

From within the Linux shell users can perform system configuration using Linux utilities, or connect to the SONiC CLI using the `vtysh` command.

///
/// tab | Telnet
To connect to the `-{{ kind_code_name }}-` CLI via telnet

```bash
telnet <container-name/id> 5000
```

///

## Interfaces mapping

The `-{{ kind_code_name }}-` container uses the following mapping for its Linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data (front-panel port) interface that is mapped to Ethernet0 port
* `eth2` - second data interface that is mapped to Ethernet4 port. Any new port will result in a "previous interface + 4" (Ethernet4) mapping.

When containerlab launches a `-{{ kind_code_name }}-` node, it will assign IPv4/6 address to the `eth0` interface. Data interface `eth1` mapped to `Ethernet0` port and needs to be configured with IP addressing manually.

## Features and Options

### Startup configuration

VM-based PLVision Enterprise SONiC supports the [`startup-config`](../nodes.md#startup-config) feature. The startup configuration file is a JSON file that is available in the VM's filesystem at `/etc/sonic/config_db.json`.

Extracting the config from a running node is possible with the `containerlab save` command. The config will be available in the lab directory.
