---
search:
  boost: 4
kind_code_name: dell_sonic
kind_display_name: Dell Enterprise SONiC
---
# -{{ kind_display_name }}-
-{{ kind_display_name }}- VM is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).
It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.


## Managing Dell SONiC nodes

Dell SONiC node launched with containerlab can be managed via the following interfaces:

/// note

1. Dell SONiC node will take ~2min to fully boot.  
You can monitor the progress with `docker logs -f <container-name>`.

2. Default credentials are `admin:admin`
///

/// tab | SSH
To open a linux shell simply type in

```bash
ssh <node-name>
```

You will enter the bash shell of the VM:

```
❯ ssh <node name>
Debian GNU/Linux 10
admin@clab-dell_sonic-ds's password: 
Linux ds 5.10.0-21-amd64 #1 SMP Debian 5.10.162-1 (2023-01-21) x86_64
You are on
  ____   ___  _   _ _  ____
 / ___| / _ \| \ | (_)/ ___|
 \___ \| | | |  \| | | |
  ___) | |_| | |\  | | |___
 |____/ \___/|_| \_|_|\____|

-- Software for Open Networking in the Cloud --

Unauthorized access and/or use are prohibited.
All access and/or use are subject to monitoring.

Help:    http://azure.github.io/SONiC/
admin@sonic:~$
```

From within the Linux shell users can perform system configuration using linux utilities, or connect to the SONiC CLI using `vtysh` command.

```
admin@sonic:~$ vtysh

Hello, this is FRRouting (version 8.2.2).
Copyright 1996-2005 Kunihiro Ishiguro, et al.

sonic#
```

///
/// tab | Telnet
to connect to sonic-vm CLI via telnet

```bash
telnet <container-name/id> 5000
```

///

## Interfaces mapping

Dell SONiC container uses the following mapping rules for its interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data (front-panel port) interface that is mapped to Ethernet0 port
* `eth2` - second data interface that is mapped to Ethernet4 port. Any new port will result in a "previous interface + 4" (Ethernet4) mapping.

When containerlab launches sonic-vs node, it will assign IPv4/6 address to the `eth0` interface. Data interface `eth1` mapped to `Ethernet0` port and needs to be configured with IP addressing manually.

## Features and options

### Startup configuration

VM-based Dell SONiC supports the [`startup-config`](../nodes.md#startup-config) feature. The startup configuration must be provided in a form of a json file extracted from the VM's `/etc/sonic/config_db.json` path. Consequently, the startup config must be provided in full, partial configuration is not supported.

When the startup config is provided, the default containerlab config is overridden and the startup config is used instead. The user-provided startup config file is copied over to the VM's `/etc/sonic/config_db.json` path and `sudo config load -y` command is executed to apply it.

### Saving configuration

Extracting the config from a running node is possible with the [`containerlab save`](../../cmd/save.md) command. The config will be available in the lab directory under the node's subdirectory.
