---
search:
  boost: 4
kind_code_name: nokia_srsim
kind_display_name: Nokia SR-SIM
---
# Nokia SR-SIM

[Nokia SR-SIM](https://www.nokia.com/networks/products/service-router-operating-system/) containerized router is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). It is a fully containerized router that replaces the Virtual Machine based SR-OS simulator (vSIM).

The Containerized Service Router Simulator, known as the SR-SIM, is a containerized version of the SR OS software that runs on the hardware platforms and it is available to Nokia customers who have an active SR-SIM license. The SR-SIM container emulates a number of hardware routers. These routers are either pizza-box systems with integrated linecards, or chassis-based systems with multiple linecards per chassis. The operator can model both types of devices. This tool is provided as a container image and designed to run on an x86 system with common container runtimes such as Docker.

The configuration of hardware elements (such as provisioning linecards, PSUs, etc.) and software elements (such as interfaces, network protocols, and services) is performed the same way as on the physical SR OS platforms with each linecard running as a separate container for emulation of multi-linecard systems (distributed model).  Pizza-box systems with integrated linecards run in an integrated model with one container per emulated sytem.

Nokia SR-SIM nodes launched with containerlab come up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled. Note that the default `admin` password is changed.

## Managing Nokia SR OS nodes

Nokia SR OS node launched with containerlab can be managed via the following interfaces:

/// tab | CLI
to connect to the SR OS CLI

```bash
ssh admin@<node-name/node-mgmt-address>
```

///
/// tab | NETCONF
NETCONF server is running over port 830

```bash
ssh admin@<node-name> -p 830 -s netconf
```

or using [netconf-console2](https://github.com/hellt/netconf-console2-container) container:

```bash
docker run --rm --network clab -i -t \
ghcr.io/hellt/netconf-console2:3.0.1 \
--host <node-name> --port 830 -u admin -p 'NokiaSros1!' \
--hello
```

///
/// tab | gNMI
using the best in class [gnmic](https://gnmic.openconfig.net) gNMI client as an example:

```bash
gnmic -a <container-name/node-mgmt-address> --insecure \
-u admin -p NokiaSros1! \
capabilities
```
///

/// admonition
    type: Default_admin_password

/// tab | Containerlab 
`admin:NokiaSros1!`
///

/// tab | Other
`admin:admin`
///
///

The logs can be retrieved with standard log commands for the given container runtime:
```bash
$ docker logs clab-sros-sr-sim1
NOKIA_SROS_CHASSIS=SR-1
NOKIA_SROS_SYSTEM_BASE_MAC=1c:30:00:00:00:00

** Container version: 0.0.I8161 (Built on Mon Jun 23 01:37:24 UTC 2025)


** using configuration file: /etc/opt/nokia/sros.cfg
mgmtIf=eth0
ifDynamic=1
cfDirs=/home/sros/chroot/cf1:;/home/sros/chroot/cf2:;/home/sros/chroot/cf3:
logDir=/var/opt/nokia/log
bootString=TIMOS: slot=a chassis=sr-1 card=cpm-1 mda/1=me6-100gb-qsfp28 mda/2=me12-100gb-qsfp28 features=2048
cpuCount=2
** linking /home/sros/chroot/cf1: to /home/sros/cf1:
** linking /home/sros/chroot/cf2: to /home/sros/cf2:
** linking /home/sros/chroot/cf3: to /home/sros/cf3:

Looking for cf3:/bof.cfg ... OK, reading
<SNIP>

```

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in -{{ kind_display_name }}-.

The interface naming convention inside the SR OS command line is typically: `L/X/M/C/P`,  `L/M/C/P` or `L/M/P` where `L` is the linecard number, `X` the xiom number (when present), `M` the MDA position, `C` is the cage or connector number, and `P` is the breakout port inside the port connector. This mapping is represented in the containerlab topology file with the following linux interface name convention: `eL-xX-M-cC-P`, `eL-M-cC-P`, `eL-M-P`. In brief, the prefix `e` is added at the beginning of the port and the forward slash character `/` is replaced with a dash or hyphen `-` as separator. Some practical examples are shown belown.

/// admonition
    type: Port_Naming
Nokia SR-SIM port naming convention examples
```
e1-2-3       -> card 1, mda 2, port 3
e1-2-c3-1    -> card 1, mda 2, connector 3, port 1
e2-2-c3-4    -> card 2, mda 2, connector 3, port 4
e1-x2-3-4    -> card 1, xiom 2, mda 3, port 4
e1-x2-3-c4-5 -> card 1, xiom 2, mda 3, connector 4, port 5
```
///

Interfaces can be defined in a non-sequential way on the `links` section of the topology file as shown in the following example:

```yaml
  links:
    # sr-sim1 port 1 on LC1 is connected to sr-sim2 port 1 on LC1
    - endpoints: ["sr-sim1:e1-1-c1-1", "sr-sim2:e1-1-1"]    
    # sr-sim port 1 on LC1 is connected to sr-sim port 1 on LC2
    - endpoints: ["sr-sim-dist-iom-1:e1-1-c1-1", "srsim-dist-iom-2:e2-x1-1-c1-1"]
    # sr-sim port 1 on LC1/MDA2 is connected to sr-sim port 1 on LC3/MDA1
    - endpoints: ["sr-sim-dist-iom1:e1-2-c1-1", "sr-sim-dist-iom3:e3-1-c1-1"]

```

The management interface for the SR-SIM will be typically mapped to the `eth0` of the Linux namespace where the container is running. 

The interfaces of an integrated system are defined with an endpoint to the container node as usual.

Distributed systems require some special acommodations given the nature of the SR-SIM:
  
  1. Containers need to all run on the same Linux namespace. This is achieved using the clab directive: `network-mode`.
  2. Attachments to the "network fabric", which in our case can be a linux bridge with arbitrary interface naming as long as they are unique, e.g.  `eth1`, `eth2`, etc. The fabric interface name can be assigned using the enviroment variable `NOKIA_SROS_FABRIC_IF`. 
  3. The data plane links for the SR-SIM node need to be connected to the container emulating the specific linecard.

An example topology for Integraded and Distributed nodes can be seen belown:
/// tab | Integrated SR-SIM
```yaml
name: "sros"
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license.txt
      image: nokia_srsim:25.7.R1
  nodes:
    sr-sim1:
      kind: nokia_srsim
      type: SR-1 # Implicit default
    sr-sim2: 
      kind: nokia_srsim
      type: VSR-I
    sr-sim3:
      kind: nokia_srsim
      type: SR-1s
    sr-sim4:
      kind: nokia_srsim
      type: SAR-1

  links:
    # Data Interfaces
    - endpoints: ["sr-sim1:e1-1-c1-1", "sr-sim2:e1-1-1"]    
    - endpoints: ["sr-sim1:e1-1-2", "sr-sim3:e1-1-c1-1"]    
    - endpoints: ["sr-sim3:e1-1-c2-1", "sr-sim4:e1-1-c1-1"]
    - endpoints: ["sr-sim4:e1-1-c2-1", "sr-sim1:e1-1-c2-1"]
```
///

/// tab | Distributed SR-SIM

```yaml
## Required bridge:
# sudo ip link add name fab type bridge
# sudo ip link set fab mtu 9000
# sudo ip link set fab up
name: "sros"
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license.txt
      image: nokia_srsim:25.7.R1
  nodes: 
    fab: # Fabric Bridge
      kind: bridge
    sr-2s-a:  # CPM-A
      kind: nokia_srsim
      type: SR-2s
      env: 
        NOKIA_SROS_SLOT: A
        NOKIA_SROS_SYSTEM_BASE_MAC: 1c:58:07:00:03:01 
        NOKIA_SROS_FABRIC_IF: eth1
    sr-2s-b: #CPM-B
      kind: nokia_srsim
      type: SR-2s
      network-mode: container:sr-2s-a
      env: 
        NOKIA_SROS_SLOT: B
        NOKIA_SROS_SYSTEM_BASE_MAC: 1c:58:07:00:03:01 
        NOKIA_SROS_FABRIC_IF: eth2
    sr-2s-1: #LINE-CARD 1
      kind: nokia_srsim
      type: SR-2s
      network-mode: container:sr-2s-a
      env: 
        NOKIA_SROS_SLOT: 1
        NOKIA_SROS_FABRIC_IF: eth3
    sr-2s-2: # LINE-CARD 2
      kind: nokia_srsim
      type: SR-2s
      network-mode: container:sr-2s-a
      env: 
        NOKIA_SROS_SLOT: 2
        NOKIA_SROS_FABRIC_IF: eth4
  links:
    ## FABRIC LINKS
    - endpoints: ["sr-2s-a:eth1", "fab:veth1"]
    - endpoints: ["sr-2s-b:eth2", "fab:veth2"]
    - endpoints: ["sr-2s-1:eth3", "fab:veth3"]
    - endpoints: ["sr-2s-2:eth4", "fab:veth4"]
    ## DATA LINKS
    - endpoints: ["sr-2s-1:e1-1-c1-1", "sr-2s-2:e2-1-c1-1"]
    - endpoints: ["sr-2s-1:e1-1-c2-1", "sr-2s-2:e2-1-c2-1"]
```
///

When containerlab launches the -{{ kind_display_name }}- node, the primary BOF interface gets an address provided by the container runtime IPAM driver. This address, will only be allocated to the active CPM. Containers emulating a secondary CPM or a linecard will not have a management interface attached, unless explicitly defined using the enviroment variable `NOKIA_SROS_MGMT_IF`

Data interfaces need to be configured with IP addressing manually using the SR OS CLI or other available management interfaces.


## Features and options

### Variants

SR OS container simulator can be run in multiple HW variants as explained in [the cSIM installation guide](TBD). These variants can be set using the `type` directive on the clab topology file or by overriding the enviroment variable for the chassis (`NOKIA_SROS_CHASSIS`) or card `NOKIA_SROS_CARD`. There are serveral other variables that will modify the default types for a simulated chassis (i.e. SFM, XIOM, MDA, etc.), so please check the Users' guide for a full list of variables.

Nokia SR OS container images can emulate any variant and use enviromental variables to change the default behavior of a given container

To make Nokia SR OS to boot in one of the packaged variants, set the type to one of the predefined variant values:
/// tab | Integrated SR-SIM
```yaml
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license.txt
      image: nokia_srsim:25.7.R1
  nodes:
    sr-sim:
      kind: nokia_srsim
      type: SR-1s
```
///
/// tab | Distributed SR-SIM
```yaml
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license.txt
      image: nokia_srsim:25.7.R1
  nodes:
    sr-sim: 
      kind: nokia_srsim
      type: SR-1x-92S
      env: 
         NOKIA_SROS_SLOT: A
    sr-sim-iom:
      kind: nokia_srsim
      type: SR-1x-92S
      network-mode: container:sr-sim
      env:
        NOKIA_SROS_SLOT: 1 
```
///
/// tab | Distributed SR-SIM2
```yaml
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license.txt
      image: nokia_srsim:25.7.R1
  nodes:
    sros-14s-a:
      kind: nokia_srsim
      type: sr-14s 
      kind: nokia_srsim
      type: SR-14s
      network-mode: container:sr-14s-a
      env:
        NOKIA_SROS_SLOT: A 
    sros-14s-b:
      kind: nokia_srsim
      type: sr-14s 
      kind: nokia_srsim
      type: SR-14s
      network-mode: container:sr-14s-a
      env:
        NOKIA_SROS_SLOT: B 
    sros-14s-1:
      kind: nokia_srsim
      type: sr-14s 
      network-mode: container:sr-14s-a
      env:
        NOKIA_SROS_SLOT: 1 
    sros-14s-2:
      kind: nokia_srsim
      type: sr-14s 
      license: license-sros25.txt
      network-mode: container:sr-14s-a
      env:
        NOKIA_SROS_SLOT: 2
```
///

#### Custom variants

A custom variant can be defined by specifying enviromental variables for the linecards or specific chassis
/// tab | Integrated SR-SIM
```yaml
topology:
  nodes:
    sr-sim1:
      kind: nokia_srsim
      type: SR-1
      env: 
        NOKIA_SROS_MDA_1: me12-100gb-qsfp28 #override default card
```
///

/// tab | Distributed SR-SIM
```yaml
  nodes:
    sr-2se-a: 
      kind: nokia_srsim
      type: SR-2se
      env: 
        NOKIA_SROS_SLOT: A
        NOKIA_SROS_SYSTEM_BASE_MAC: 1c:58:07:00:03:01 # override Chassis MAC
        NOKIA_SROS_FABRIC_IF: eth1 # override fabric itf
        NOKIA_SROS_CARD: cpm-2se #override CPM
        NOKIA_SROS_SFM: sfm-2se # override SFM
    sros-2se-1:
      kind: nokia_srsim
      image: nokia_srsim:25.7.R1
      type: sr-2se 
      license: license-sros25.txt
      network-mode: container:sr-2s-a
      env:
        NOKIA_SROS_SLOT: 1  
        NOKIA_SROS_CARD: xcm-2se #override IOM
        NOKIA_SROS_MDA_1: x2-s36-800g-qsfpdd-18.0t #override MDA
```
///

When a node uses multiple linecards users should pay special attention to the way links are defined in the topology file. As explained in the [interface naming](#interface-naming) section, SR OS nodes will need to be mapped to the linecard, xiom, mda or port they use, therefore the endpoints array need to indicate the linecard where the connections are made.


### Node configuration

Nokia SR OS nodes come up with a basic "blank" configuration where only the card/mda are provisioned, as well as the management interfaces such as Netconf, SNMP, gNMI.

#### User-defined config

SR-SIM nodes launched come up with some basic configuration that configures the management interfaces, linecards, mdas and power modules need to be provisioned. This initial configuration is applied right after the node is booted.

Since this initial configuration is meant to provide a bare minimum configuration to make the node operational, users will likely want to apply their own configuration to the node to enable some features or to configure some interfaces. This can be done by providing a user-defined configuration file using [`startup-config`](../nodes.md#startup-config) property of the node/kind.

/// tip
Configuration text can contain Go template logic as well as make use of [environment variables](../topo-def-file.md#environment-variables) allowing for runtime customization of the configuration.
///

##### Full startup-config

When a user provides a path to a file that has a complete configuration for the node, containerlab will copy that file to the lab directory for that specific node under the `<node>/config/config.cfg` name and mount that dir to the container. This will result in this config to act as a startup-config for the node:

```yaml
name: sros
topology:
  nodes:
    sros:
      kind: nokia_srsim
      startup-config: myconfig.txt
```

/// note
With the above configuration, the node will boot with the configuration specified in `myconfig.txt`, no other configuration will be applied. You have to provision interfaces, cards, power-shelves, etc. yourself.
///

##### Partial startup-config

Quite often it is beneficial to have a partial configuration that will be applied on top of the default configuration that containerlab applies. For example, users might want to add some services on top of the default configuration provided by containerlab and do not want to have the full configuration file.

This can be done by providing a partial configuration file that will be applied on top of the default configuration. The partial configuration file must have `.partial` string in its name, for example, `myconfig.partial.txt`.

```yaml
name: sros_lab
topology:
  nodes:
    sros:
      kind: nokia_srsim
      startup-config: myconfig.partial.txt
```

The partial config can contain configuration in a MD-CLI syntax that is accepted in the configuration mode of the SR OS. The way partial config is applied is by sending lines from the partial config file to the SR OS via SSH. A few important things to note:

1. Entering the configuration mode is not required, containerlab will do that for you. `edit-config exclusive` mode is used by containerlab.
2. `commit` command **must not** be included in the partial config file, containerlab will do that for you.

Both `flat` and normal syntax can be used in the partial config file. For example, the following partial config file adds a static route to the node in the regular CLI syntax:

```bash
    configure {
       router "Base" {
           static-routes {
               route 192.168.200.200/32 route-type unicast {
                   next-hop "192.168.0.1" {
                       admin-state enable
                   }
               }
           }
       }
    }
```

###### Remote partial files

It is possible to provide a partial config file that is located on a remote http(s) server. This can be done by providing a URL to the file. The URL must start with `http://` or `https://` and must point to a file that is accessible from the containerlab host.

/// note
The URL **must have** `.partial` in its name:
///

```yaml
name: sros_lab
topology:
  nodes:
    sros:
      kind: nokia_srsim
      startup-config: https://gist.com/<somehash>/staticroute.partial.cfg
```

###### Embedded partial files

Users can also embed the partial config in the topology file itself, making it a hermetic artifact that can be shared with others. This can be done by using multiline string in YAML:

```yaml
name: sros_lab
topology:
  nodes:
    sros:
      kind: nokia_srsim
      startup-config: | #(1)!
        /configure system location "I am an embedded config"
```

1. It is mandatory to use YAML's multiline string syntax to denote that the string below is a partial config and not a file.

Embedded partial configs will persist on containerlab's host and use the same directory as the [remote startup-config](../config-mgmt.md#remote) files.

#### Configuration save

Containerlab's [`save`](../../cmd/save.md) command will perform a configuration save for `Nokia SR OS` nodes via Netconf. The configuration will be saved under `config.cfg` file and can be found at the node's directory inside the lab parent directory:

```bash
# assuming the lab name is "cert01"
# and node name is "sr"
cat clab-cert01/sr/config/config.cfg
```

#### Boot Options File WIP HERE

By default `nokia_srsim` nodes boot up with a pre-defined "Boot Options File" (BOF). This file includes boot settings including:

* license file location
* config file location

Some common BOF options can also be controlled using eviromental variables as specified in the cSIM user's guide.


#### SSH keys

Containerlab v0.48.0+ supports SSH key injection into the Nokia SR OS nodes. First containerlab retrieves all public keys from `~/.ssh`[^1] directory and `~/.ssh/authorizde_keys` file, then it retrieves public keys from the ssh agent if one is running.

Next it will filter out public keys that are not of RSA/ECDSA type. The remaining valid public keys will be configured for the admin user of the Nokia SR OS node using key IDs from 32 downwards[^2]. This will enable key-based authentication next time you connect to the node.

/// details | Skipping keys injection
If you want to disable this feature (e.g. when using classic CLI mode), you can do so by setting the `CLAB_SKIP_SROS_SSH_KEY_CONFIG=true` env variable:

```bash
sudo CLAB_SKIP_SROS_SSH_KEY_CONFIG=true -E clab deploy -t <topo-file>
```

///

### License

Path to a valid license must be provided for all Nokia SR OS nodes with a [`license`](../nodes.md#license) directive.

### File mounts

When a user starts a lab, containerlab creates a node directory for storing [configuration artifacts](../conf-artifacts.md). For Nokia SR OS kind containerlab creates `tftpboot` directory where the license file will be copied.

## Lab examples

The following labs feature Nokia SR OS node:

* [SR Linux and vr-sros](../../lab-examples/vr-sros.md)

[^1]: `~` is the home directory of the user that runs containerlab.
[^2]: If a user wishes to provide a custom startup-config with public keys defined, then they should use key IDs from 1 onwards. This will minimize chances of key ID collision causing containerlab to overwrite user-defined keys.
