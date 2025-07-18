---
search:
  boost: 4
---
# Juniper cRPD

[Juniper cRPD](https://www.juniper.net/documentation/us/en/software/crpd/crpd-deployment/topics/concept/understanding-crpd.html) is identified with `crpd` or `juniper_crpd` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a `crpd` node.

cRPD nodes launched with containerlab comes up pre-provisioned with SSH service enabled, `root` user created and NETCONF enabled.

Once downloaded, load the Docker image:

```bash
# load cRPD container image, shows up as crpd:24.2R1.14 in docker images
sudo docker load -i junos-routing-crpd-docker-24.2R1.14.tgz
```

## Managing cRPD nodes

Juniper cRPD node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running cRPD container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the cRPD CLI
    ```bash
    docker exec -it <container-name/id> cli
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh root@<container-name> -p 830 -s netconf
    ```

!!!info
    Default user credentials: `root:clab123`

## Interfaces mapping

cRPD container uses the following mapping for its linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface

When containerlab launches cRPD node, it will assign IPv4/6 address to the `eth0` interface. Data interface `eth1` needs to be configured with IP addressing manually.

???note "cRPD interfaces output"
    This output demonstrates the IP addressing of the linux interfaces of cRPD node.
    ```
    ❯ docker exec -it clab-crpd-crpd bash

    ===>
            Containerized Routing Protocols Daemon (CRPD)
    Copyright (C) 2020, Juniper Networks, Inc. All rights reserved.
                                                                        <===

    root@crpd:/# ip a
    1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
        link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
        inet 127.0.0.1/8 scope host lo
        valid_lft forever preferred_lft forever
        inet6 ::1/128 scope host
        valid_lft forever preferred_lft forever

    <SNIP>

    5767: eth0@if5768: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1450 qdisc noqueue state UP group default
        link/ether 02:42:ac:14:14:03 brd ff:ff:ff:ff:ff:ff link-netnsid 0
        inet 172.20.20.3/24 brd 172.20.20.255 scope global eth0
        valid_lft forever preferred_lft forever
        inet6 3fff:172:20:20::3/80 scope global nodad
        valid_lft forever preferred_lft forever
        inet6 fe80::42:acff:fe14:1403/64 scope link
        valid_lft forever preferred_lft forever
    5770: eth1@if5769: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
        link/ether b6:d3:63:f1:cb:7b brd ff:ff:ff:ff:ff:ff link-netnsid 1
        inet6 fe80::b4d3:63ff:fef1:cb7b/64 scope link
        valid_lft forever preferred_lft forever
    ```
    This output shows how the linux interfaces are mapped into the cRPD OS.
    ```
    root@crpd> show interfaces routing
    Interface        State Addresses
    lsi              Up
    tunl0            Up    ISO   enabled
    sit0             Up    ISO   enabled
                        INET6 ::172.20.20.3
                        INET6 ::127.0.0.1
    lo.0             Up    ISO   enabled
                        INET6 fe80::1
    ip6tnl0          Up    ISO   enabled
                        INET6 fe80::42a:e9ff:fede:a0e3
    gretap0          Down  ISO   enabled
    gre0             Up    ISO   enabled
    eth1             Up    ISO   enabled
                        INET6 fe80::b4d3:63ff:fef1:cb7b
    eth0             Up    ISO   enabled
                        INET  172.20.20.3
                        INET6 3fff:172:20:20::3
                        INET6 fe80::42:acff:fe14:1403
    ```
    As you see, the management interface `eth0` inherits the IP address that docker assigned to cRPD container.

## Features and options

### Node configuration

cRPD nodes have a dedicated [`config`](../conf-artifacts.md#identifying-a-lab-directory) directory that is used to persist the configuration of the node. It is possible to launch nodes of `crpd` kind with a basic "empty" config or to provide a custom config file that will be used as a startup config instead.

#### Default node configuration

When a node is defined without `config` statement present, containerlab will generate an empty config from [this template](https://github.com/srl-labs/containerlab/blob/main/nodes/crpd/crpd.cfg) and copy it to the config directory of the node.

```yaml
# example of a topo file that does not define a custom config
# as a result, the config will be generated from a template
# and used by this node
name: crpd
topology:
  nodes:
    crpd:
      kind: crpd
```

The generated config will be saved by the path `clab-<lab_name>/<node-name>/config/juniper.conf`. Using the example topology presented above, the exact path to the config will be `clab-crpd/crpd/config/juniper.conf`.

#### User defined config

It is possible to make cRPD nodes to boot up with a user-defined config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind a user sets the path to the config file that will be mounted to a container:

```yaml
name: crpd_lab
topology:
  nodes:
    crpd:
      kind: crpd
      startup-config: myconfig.conf
```

With such topology file containerlab is instructed to take a file `myconfig.conf` from the current working directory, copy it to the lab directory for that specific node under the `/config/juniper.conf` name and mount that dir to the container. This will result in this config to act as a startup config for the node.

#### Saving configuration

With [`containerlab save`](../../cmd/save.md) command it's possible to save running cRPD configuration into a file. The configuration will be saved by `/config/juniper.conf` path in the relevant node directory.

### License

cRPD containers require a license file to have some features to be activated. With a [`license`](../nodes.md#license) directive it's possible to provide a path to a license file that will be copied over to the nodes configuration directory by the `/config/license/safenet/junos_sfnt.lic` path and will get applied automatically on boot.

## Container configuration

To launch cRPD, containerlab uses the deployment instructions that are provided in the [TechLibrary](https://www.juniper.net/documentation/us/en/software/crpd/crpd-deployment/topics/task/crpd-linux-server-install.html) as well as leveraging some setup steps outlined by Matt Oswalt in [this blog post](https://oswalt.dev/2020/03/building-your-own-junos-router-with-crpd-and-linuxkit/).

The SSH service is already enabled for root login, so nothing is needed to be done additionally.

The `root` user is created already with the `clab123` password.

### File mounts

When a user starts a lab, containerlab creates a node directory for storing [configuration artifacts](../conf-artifacts.md). For `crpd` kind containerlab creates `config` and `log` directories for each crpd node and mounts these folders by `/config` and `/var/log` paths accordingly.

```
❯ tree clab-crpd/crpd
clab-crpd/crpd
├── config
│   ├── juniper.conf
│   ├── license
│   │   └── safenet
│   └── sshd_config
└── log
    ├── cscript.log
    ├── license
    ├── messages
    ├── mgd-api
    ├── na-grpcd
    ├── __policy_names_rpdc__
    └── __policy_names_rpdn__

4 directories, 9 files
```

## Lab examples

The following labs feature cRPD node:

* [SR Linux and cRPD](../../lab-examples/srl-crpd.md)
