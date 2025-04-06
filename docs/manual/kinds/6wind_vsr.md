---
search:
  boost: 4
---
# 6WIND VSR

[6WIND VSR](https://www.6wind.com/vrouter-vsr-solutions/) is identified with `6wind_vsr` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a `6wind_vsr` node.

6WIND VSR nodes launched with containerlab comes up pre-provisioned with SSH service enabled, `root` and `admin` user created and NETCONF enabled.

## Getting 6WIND VSR image

6WIND VSR images can be retrieved from your 6WIND customer portal.
Free evaluation images with associated licenses are available upon [registration](https://portal.6wind.com/register.php).

## Managing 6WIND VSR nodes

6WIND VSR node launched with containerlab can be managed via the following interfaces:

/// tab | CLI
to connect to the 6WIND VSR CLI

```bash
ssh admin@<container-name>
```

///
/// tab | NETCONF
NETCONF server is running over port 830

```bash
ssh admin@<container-name> -p 830 -s netconf
```

///

### Credentials

Default credentials: `admin:admin`

Containerlab will automatically enable public-key authentication for the `admin` user if public key files are found at `~/.ssh` directory.

Root access is also available but should not be used to configure 6WIND VSR nodes.
The configuration entrypoint is the NETCONF server.
The CLI is a local NETCONF client running inside the node.

## Host server requirements

6WIND VSR documentation lists the host [requirements](https://doc.6wind.com/new/vsr-3/latest/vsr-guide/getting-started/run-container/host-requirements.html#node-requirements-and-configuration) needed to run VSR as a container.

Among them, it is important to load the required [kernel modules](https://doc.6wind.com/new/vsr-3/latest/vsr-guide/getting-started/run-container/host-requirements.html#kernel-modules), as well as increasing the maximum number of [inotify watchers](https://doc.6wind.com/new/vsr-3/latest/vsr-guide/getting-started/run-container/host-requirements.html#setting-maximum-number-of-inotify-watchers).

To make the settings persist reboots append `fs.inotify.max_user_instances=2048` and `fs.inotify.max_user_watches=1048576` to `/etc/sysctl.conf`. You can use the following one-liner:

```shell
echo -e "fs.inotify.max_user_instances=2048\nfs.inotify.max_user_watches=1048576" | sudo tee -a /etc/sysctl.conf
```

## Interfaces mapping

6WIND VSR container uses the following mapping for its interfaces:

* `eth0` - management interface connected to the containerlab management network

When containerlab launches a 6WIND VSR node, it will assign IPv4/6 address to the `eth0` interface.

### Data plane interfaces

There is no restriction on the user-defined data interfaces name.
Data plane interfaces needs to be configured with IP addressing manually.

Network interfaces added by containerlab to a 6WIND VSR node appears as infrastructure interfaces.
They are probed at the container startup.
The infrastructure interfaces are identified by a port name, corresponding to the name of the interface at container start.
This port name stays the same during the container life, even if the interface is renamed or moved to another VRF.
This output explains how to list the interfaces, and use them in the CLI

```
ssh admin@<containername>
Warning: Permanently added '<containername>' (ED25519) to the list of known hosts.
#######################################################################
# Welcome to 6WIND Virtual Service Router                             #
#                                                                     #
# Most useful commands at that step:                                  #
#                                                                     #
# edit running          # to edit the running configuration           #
# show interface        # for interface names, state and IP addresses #
# show summary          # for the vRouter state summary               #
# ?                     # for the list of available commands          #
# help <cmd>            # for detailed help on the <cmd> command      #
#                                                                     #
# Feel free to customize this banner using                            #
# cmd banner post-login message                                       #
#######################################################################

Last login: Mon Mar 17 09:49:25 2025 from 3fff:172:20:20::1
gNodeB-north> show state / network-port
network-port infra-eth2
    mac-address aa:c1:ab:a9:75:35
    interface eth2
    type virtual
    ..
network-port infra-eth1
    mac-address aa:c1:ab:d8:3f:93
    interface eth1
    type virtual
    ..
network-port infra-eth0
    mac-address 02:42:ac:14:14:09
    interface eth0
    type virtual
    ..
network-port pci-b2s2
    bus-addr 0000:02:02.0
    vendor VMware
    model "VMXNET3 Ethernet Controller"
    type physical
    ..

```

By default, the input packets received on an infrastructure interface is not received by the [fast path](https://doc.6wind.com/new/vsr-3/latest/vsr-guide/user-guide/cli/system/fast-path.html) (6WIND's accelerated networking stack), but directly by the control plane.
Most of the time, this is a good option since these interfaces are used for management. To change this behavior, it is possible to add the network port to the fast path configuration.
Here is a configuration example:

```
VSR> edit running
VSR running config# system fast-path virtual-port infrastructure infra-eth1
VSR running infrastructure infra-eth1# / vrf data interface infrastructure eth1 port infra-eth1
VSR running infrastructure infra-eth1# / vrf data interface infrastructure eth1 ipv4 address 192.168.0.1/24
VSR running infrastructure infra-eth1# commit
Configuration committed.
VSR running infrastructure infra-eth1# show interface vrf data
Name   State L3vrf   IPv4 Addresses IPv6 Addresses               Description
====   ===== =====   ============== ==============               ===========
lo     UP    default 127.0.0.1/8    ::1/128                      loopback_data
eth1   UP    default 192.168.0.1/24 fe80::a8c1:abff:fed8:3f93/64 infra-eth1
fptun0 UP    default                fe80::6470:74ff:fe75:6e30/64
```

As you see, the data plane interface `eth1` is configured in the requested VRF.

## Features and options

### Node configuration

#### Default node configuration

When a node is defined without `startup-config` statement present, containerlab will generate an empty config from [this template](https://github.com/srl-labs/containerlab/blob/main/nodes/6wind_vsr/6wind_vsr_default_config.go.tpl) and copy it to the config directory of the node.

#### Startup config

It is possible to make 6WIND VSR nodes to boot up with a user-defined startup config. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind a user sets the path to the config file that will be mounted to a container:

```yaml
name: vsr_lab
topology:
  nodes:
    vsr:
      kind: 6wind_vsr
      startup-config: myconfig.conf
```

With such topology file containerlab is instructed to take a file `myconfig.conf` from the current working directory, copy it to the lab directory for that specific node under the `init-config.cli` name and mount that file to the container. This will result in this config to act as a startup config for the node.

Note: The startup configuration will only be used if no user-defined configuration was saved as explained below.
Also, the content of the default template is always appended to the configuration, in order to ease the connection to the container.

#### Devices

6WIND VSR requires devices to be usable by the container.
The following devices are automatically added when using the `6wind_vsr` kind:

* /dev/ppp
* /dev/net/tun
* /dev/vhost-net

#### Capabilities

6WIND VSR required either to be run in privileged mode, or in non-privileged mode provided some capabilities.
For now, containerlab executes all the containers in privileged mode. To make sure the `6wind_vsr` kind is not impacted if this default policy changes or becomes configurable, the list of capabilites required by the 6WIND VSR container is automatically added. Additional capabilities are listed below:

* NET_ADMIN
* SYS_ADMIN
* NET_BROADCAST
* NET_RAW
* SYS_NICE
* IPC_LOCK
* SYSLOG
* AUDIT_WRITE
* SYS_PTRACE
* SYS_TIME

#### Saving configuration

With [`containerlab save`](../../cmd/save.md) command it's possible to save running 6WIND VSR configuration into a file.
6WIND VSR nodes have a dedicated [`config`](../conf-artifacts.md#identifying-a-lab-directory) directory that is used to persist the configuration of the node. The configuration will be saved by `user-startup.conf` path in the relevant node directory. When present, this file will be used as a startup configuration instead of the file provided by the [`startup-config`](../nodes.md#startup-config) property of the node/kind.

### License

6WIND VSR containers require a license. The license serial is provided through the [configuration](https://doc.6wind.com/new/vsr-3/latest/vsr-guide/user-guide/cli/system/license/install.html#online-license).

## Container configuration

Please refer to 6WIND VSR [User's guide](https://doc.6wind.com/new/vsr-3/latest/vsr-guide/user-guide/cli/index.html) for the configuration of the node.
