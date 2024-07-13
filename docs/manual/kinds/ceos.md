---
search:
  boost: 4
---
# Arista cEOS

Arista cEOS is identified with `ceos` or `arista_ceos` kind in the [topology file](../topo-def-file.md). The `ceos` kind defines a supported feature set and a startup procedure of a `ceos` node.

cEOS nodes launched with containerlab comes up with

* their management interface `eth0` configured with IPv4/6 addresses as assigned by docker
* hostname assigned to the node name
* gNMI, Netconf and eAPI services enabled
* `admin` user created with password `admin`

## Getting cEOS image
<!-- --8<-- [start:ceos-get-image] -->
Arista requires its users to register with arista.com before downloading any images. Once you created an account and logged in, go to the [software downloads](https://www.arista.com/en/support/software-download) section and download ceos64 tar archive for a given release.

Once downloaded, import the archive with docker:

```bash
# import container image and save it under ceos:4.32.0F name
docker import cEOS64-lab-4.32.0F.tar.xz ceos:4.32.0F
```
<!-- --8<-- [end:ceos-get-image] -->
## Managing ceos nodes

Arista cEOS node launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running ceos container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the ceos CLI
    ```bash
    docker exec -it <container-name/id> Cli
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh root@<container-name> -p 830 -s netconf
    ```
=== "gNMI"
    gNMI server is running over port 6030 in non-secure mode
    using the best in class [gnmic](https://gnmic.kmrd.dev) gNMI client as an example:
    ```bash
    gnmic -a <container-name/node-mgmt-address>:6030 --insecure \
    -u admin -p admin \
    capabilities
    ```

!!!info
    Default user credentials: `admin:admin`

## Interfaces mapping

ceos container uses the following mapping for its linux interfaces:

* `eth0`[^5] - management interface connected to the containerlab management network
* `eth1` - first data interface

When containerlab launches ceos node, it will set IPv4/6 addresses as assigned by docker to the `eth0` interface and ceos node will boot with that addresses configured. Data interfaces `eth1+` need to be configured with IP addressing manually.

???note "ceos interfaces output"
    This output demonstrates the IP addressing of the linux interfaces of ceos node.
    ```
    bash-4.2# ip address
    1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
        link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
        inet 127.0.0.1/24 scope host lo
        valid_lft forever preferred_lft forever
        inet6 ::1/128 scope host
        valid_lft forever preferred_lft forever
    <SNIP>
    5877: eth0@if5878: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
        link/ether 02:42:ac:14:14:02 brd ff:ff:ff:ff:ff:ff link-netnsid 0
        inet 172.20.20.2/24 brd 172.20.20.255 scope global eth0
        valid_lft forever preferred_lft forever
        inet6 2001:172:20:20::2/80 scope global
        valid_lft forever preferred_lft forever
        inet6 fe80::42:acff:fe14:1402/64 scope link
        valid_lft forever preferred_lft forever
    ```
    This output shows how the linux interfaces are mapped into the ceos OS.
    ```
    ceos>sh ip int br
                                                                                Address
    Interface         IP Address           Status       Protocol           MTU    Owner
    ----------------- -------------------- ------------ -------------- ---------- -------
    Management0       172.20.20.2/24       up           up                1500

    ceos>sh ipv6 int br
    Interface       Status        MTU       IPv6 Address                     Addr State    Addr Source
    --------------- ------------ ---------- -------------------------------- ---------------- -----------
    Ma0             up           1500       fe80::42:acff:fe14:1402/64       up            link local
                                            2001:172:20:20::2/80             up            config
    ```
    As you see, the management interface `Ma0` inherits the IP address that docker assigned to ceos container management interface.

### User-defined interface mapping

!!!note
    Supported in cEOS >= 4.28.0F

It is possible to make ceos nodes boot up with a user-defined interface layout. With the [`binds`](../nodes.md#binds) property, a user sets the path to the interface mapping file that will be mounted to a container and used during bootup. The underlying linux `eth` interfaces (used in the containerlab topology file) are mapped to cEOS interfaces in this file. The following shows an example of how this mapping file is structured:

```json
{
  "ManagementIntf": {
    "eth0": "Management1"
  },
  "EthernetIntf": {
    "eth1": "Ethernet1/1",
    "eth2": "Ethernet2/1",
    "eth3": "Ethernet27/1",
    "eth4": "Ethernet28/1",
    "eth5": "Ethernet3/1/1",
    "eth6": "Ethernet5/2/1"
  }
}
```

Linux's `eth0` interface is always used to map the management interface.

With the following topology file, containerlab is instructed to take a `mymapping.json` file located in the same directory as the topology and mount that to the container as `/mnt/flash/EosIntfMapping.json`. This will result in this interface mapping being considered during the bootup of the node. The destination for that bind has to be `/mnt/flash/EosIntfMapping.json`.

1. Craft a valid interface mapping file.
2. Use `binds` config option for a ceos node/kind to make this file available in the container's filesystem:

    ```yaml
    name: ceos

    topology:
      nodes:
        ceos1:
          kind: ceos
          image: ceos:4.32.0F
          binds:
            - mymapping.json:/mnt/flash/EosIntfMapping.json:ro # (1)!
        ceos2:
          kind: ceos
          image: ceos:4.32.0F
          binds:
            - mymapping.json:/mnt/flash/EosIntfMapping.json:ro
      links:
        - endpoints: ["ceos1:eth1", "ceos2:eth1"]
    ```

    1. If all ceos nodes use the same interface mapping file, it is easier to set the bind instruction on a kind level

    ```yaml
        topology:
          kinds:
            ceos:
              binds:
                - mymapping.json:/mnt/flash/EosIntfMapping.json:ro
          nodes:
            ceos1:
              kind: ceos
              image: ceos:4.32.0F
            ceos2:
              kind: ceos
              image: ceos:4.32.0F
    ```

    This way the bind is set only once, and nodes of `ceos` kind will have these binds applied.

### Additional interface naming considerations

While many users will be fine with the default ceos naming of `eth`, some ceos users may find that they need to name their interfaces `et`. Interfaces named `et` provide consistency with the underlying interface mappings within ceos. This enables the correct operation of commands/features which depend on `et` format interface naming.

In order to align interfaces in this manner, the `INTFTYPE` environment variable must be set to `et` in the topology definition file and the links which are defined must be named `et`, as opposed to `eth`. This naming requirement does not apply to the `eth0` interface automatically created by containerlab. This is only required for links that are used for interconnection with other elements in a topology.

example:

```yaml
topology:
  defaults:
    env:
      INTFTYPE: et
  nodes:
  # --snip--
  links:
    - endpoints: ["ceos_rtr1:et1", "ceos_rtr2:et1"]
    - endpoints: ["ceos_rtr1:et2", "ceos_rtr3:et1"]
```

If the only purpose of renaming the interfaces is to add breakouts ("/1", etc.) to the interface naming to match the future physical setup, it is possible to use underscores ("_") in the interface names.

```yaml
name: ceos

topology:
  nodes:
    ceos1:
      kind: ceos
      image: ceos:4.32.0F
    ceos2:
      kind: ceos
      image: ceos:4.32.0F
  links:
    - endpoints: ["ceos1:eth1_1", "ceos2:eth2_1_1"]
```

This topology will be equivalent to `ceos1:Ethernet1/1` connected to `ceos2:Ethernet2/1/1`.

!!!note
    This feature can not be used together with interface mapping. If the interface mapping is in use, all names must be redefined in the map and the underscore naming option will not work. Also, it's only possible to rename Ethernet interfaces this way, not management ports.

## Features and options

### Node configuration

cEOS nodes have a dedicated [`config`](../conf-artifacts.md#identifying-a-lab-directory) directory that is used to persist the configuration of the node. It is possible to launch nodes of `ceos` kind with a basic config or to provide a custom config file that will be used as a startup config instead.

#### Default node configuration

When a node is defined without `startup-config` statement present, containerlab will generate an empty config from [this template](https://github.com/srl-labs/containerlab/blob/main/nodes/ceos/ceos.cfg) and copy it to the config directory of the node.

```yaml
# example of a topo file that does not define a custom config
# as a result, the config will be generated from a template
# and used by this node
name: ceos
topology:
  nodes:
    ceos:
      kind: ceos
```

The generated config will be saved by the path `clab-<lab_name>/<node-name>/flash/startup-config`. Using the example topology presented above, the exact path to the config will be `clab-ceos/ceos/flash/startup-config`.

cEOS Ma0 interface will be configured with a random MAC address with `00:1c:73` OUI part. Containerlab will also create a `system_mac_address` file in the node's lab directory with the value of a System MAC address. The System MAC address value is calculated as `Ma0-MAC-addr + 1`.

A default ipv4 route is also created with a next-hop of the management network to allow for outgoing connections.

#### MGMT VRF

The default empty configuration supports placing the management interface into a VRF to isolate it from the main device routing table.  Passing the environment variable `CLAB_MGMT_VRF` in either the kind or node definition will activate this behavior, and alter the management services configuration to also reflect the management VRF.  You can duplicate this when using the `startup-config` by starting from the linked template below.

```yaml
# example topo file with management VRF
# node1 will have vrf MGMT
# node2 will have vrf FOO
name: ceos_vrf
topology:
  kinds:
    ceos:
      env:
        CLAB_MGMT_VRF: MGMT
  nodes:
    node1:
      kind: ceos
    node2:
      kind: ceos
      env:
        CLAB_MGMT_VRF: FOO
```

#### User defined config

It is possible to make ceos nodes to boot up with a user-defined config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property a user sets the path to the config file that will be mounted to a container and used as a startup config:

```yaml
name: ceos_lab
topology:
  nodes:
    ceos:
      kind: ceos
      startup-config: myconfig.conf
```

When a config file is passed via `startup-config` parameter it will be used during an initial lab deployment. However, a config file that might be in the lab directory of a node takes precedence over the startup-config[^3].

With such topology file containerlab is instructed to take a file `myconfig.conf` from the current working directory, copy it to the lab directory for that specific node under the `/flash/startup-config` name and mount that dir to the container. This will result in this config to act as a startup config for the node.

It is possible to change the default config which every ceos node will start with with the following steps:

1. Craft a valid startup configuration file[^2].
2. Use this file as a startup-config for ceos kind:

    ```yaml
    name: ceos

    topology:
      kinds:
        ceos:
        startup-config: ceos-custom-startup.cfg
      nodes:
        # ceos1 will boot with ceos-custom-startup.cfg as set in the kind parameters
        ceos1:
          kind: ceos
          image: ceos:4.32.0F
        # ceos2 will boot with its own specific startup config, as it overrides the kind variables
        ceos2:
          kind: ceos
          image: ceos:4.32.0F
          startup-config: node-specific-startup.cfg
      links:
        - endpoints: ["ceos1:eth1", "ceos2:eth1"]
    ```

#### Saving configuration

In addition to cli commands such as `write memory` user can take advantage of the [`containerlab save`](../../cmd/save.md) command. It saves running cEOS configuration into a startup config file effectively calling the `write` CLI command.

## Container configuration

To start an Arista cEOS node containerlab uses the following configuration:

=== "Startup command"
    `/sbin/init systemd.setenv=INTFTYPE=eth systemd.setenv=ETBA=1 systemd.setenv=SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT=1 systemd.setenv=CEOS=1 systemd.setenv=EOS_PLATFORM=ceoslab systemd.setenv=container=docker systemd.setenv=MAPETH0=1 systemd.setenv=MGMT_INTF=eth0`
=== "Environment variables"
    `CEOS:1`
    `EOS_PLATFORM":ceoslab`
    `container:docker`
    `ETBA:1`
    `SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT:1`
    `INTFTYPE:eth`
    `MAPETH0:1`
    `MGMT_INTF:eth0`

### File mounts

When a user starts a lab, containerlab creates a node directory for storing [configuration artifacts](../conf-artifacts.md). For `ceos` kind containerlab creates `flash` directory for each ceos node and mounts these folders by `/mnt/flash` paths.

```
❯ tree clab-srlceos01/ceos
clab-srlceos01/ceos
└── flash
    ├── AsuFastPktTransmit.log
    ├── debug
    │   └── proc
    │       └── modules
    ├── fastpkttx.backup
    ├── Fossil
    ├── kickstart-config
    ├── persist
    │   ├── local
    │   ├── messages
    │   ├── persistentRestartLog
    │   ├── secure
    │   └── sys
    ├── schedule
    │   └── tech-support
    │       └── ceos_tech-support_2021-01-14.0907.log.gz
    ├── SsuRestoreLegacy.log
    ├── SsuRestore.log
    ├── system_mac_address
    └── startup-config

9 directories, 11 files
```

## Copy to `flash`

If there is a need to copy ceos-specific configuration or override files to the ceos node in the topology use `.extras.ceos-copy-to-flash` config option. These files will be copied to the node's flash directory and evaluated on startup.

```yaml
name: ceos
topology:
  nodes:
    ceos1:
      kind: ceos
      ...
      extras:
        ceos-copy-to-flash:
        - ceos-config # (1)!
        - toggle_override
```

1. Paths are relative to the topology file. Absolute paths like `~/some/path` or `/some/path` are also possible.

## Lab examples

The following labs feature a cEOS node:

* [SR Linux and cEOS](../../lab-examples/srl-ceos.md)

## Known issues or limitations

### cgroups v1

In versions prior to EOS-4.32.0F, the ceos-lab image requires a cgroups v1 environment. For many users, this should not require any changes to the runtime environment. However, some Linux distributions (ref: [#467](https://github.com/srl-labs/containerlab/issues/467)) may be configured to use cgroups v2 out-of-the-box[^4], which will prevent ceos-lab image from booting. In such cases, the users will need to configure their system to utilize a cgroups v1 environment.

Consult your distribution's documentation for details regarding configuring cgroups v1 in case you see similar startup issues as indicated in [#467](https://github.com/srl-labs/containerlab/issues/467).

Starting with EOS-4.32.0F, ceos-lab will automatically determine whether the container host is using cgroups v1 or cgroups v2 and act appropriately. No configuration is required.

??? "Switching to cgroup v1 in Ubuntu 21.04"
    To switch back to cgroup v1 in Ubuntu 21+ users need to add a kernel parameter `systemd.unified_cgroup_hierarchy=0` to GRUB config. Below is a snippet of `/etc/default/grub` file with the added `systemd.unified_cgroup_hierarchy=0` parameter.

    Note that `sudo update-grub` is needed once changes are made to the file.

    ```bash
    # If you change this file, run 'update-grub' afterwards to update
    # /boot/grub/grub.cfg.
    # For full documentation of the options in this file, see:
    #   info -f grub -n 'Simple configuration'

    GRUB_DEFAULT=0
    GRUB_TIMEOUT_STYLE=hidden
    GRUB_TIMEOUT=0
    GRUB_DISTRIBUTOR=`lsb_release -i -s 2> /dev/null || echo Debian`
    GRUB_CMDLINE_LINUX_DEFAULT="transparent_hugepage=never quiet splash systemd.unified_cgroup_hierarchy=0"
    GRUB_CMDLINE_LINUX=""
    ```

### WSL

When running under WSL2 ceos datapath might appear not working. As of Feb 2022 users would need to manually enter the following iptables rules inside ceos container:

```
sudo iptables -P INPUT ACCEPT
sudo ip6tables -P INPUT ACCEPT
```

[^2]: feel free to omit the IP addressing for Management interface, as it will be configured by containerlab when ceos node boots.
[^3]: if startup config needs to be enforced, either deploy a lab with `--reconfigure` flag, or use [`enforce-startup-config`](../nodes.md#enforce-startup-config) setting.
[^4]: for example, Ubuntu 21.04 comes with cgroup v2 [by default](https://askubuntu.com/a/1369957).
[^5]: interface name can also be `et` instead of `eth`.

### Scale

From version 4.32.0F, the ceos-lab image supports up to 50 nodes per host. On previous releases and/or with higher scale there might be issues cores inside the ceos-lab nodes and errors like `Error: Too many open files`.

Example solution for 60 ceos-lab nodes:

1. On the host run:

```
sudo sh -c 'echo "fs.inotify.max_user_instances = 75000" > /etc/sysctl.d/99-zceoslab.conf'
sudo sysctl --load /etc/sysctl.d/99-zceoslab.conf
```

where 75000 is `60 (# of nodes) * 1250`.

2. Bind newly created file into the ceos-lab containers:

```
...
topology:
  kinds:
    ceos:
      ...
      binds:
        - /etc/sysctl.d/99-zceoslab.conf:/etc/sysctl.d/99-zceoslab.conf:ro
...
```
