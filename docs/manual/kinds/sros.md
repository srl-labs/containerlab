---
search:
  boost: 4
kind_code_name: nokia_srsim
kind_display_name: Nokia SR-SIM
---
# Nokia SR-SIM

[Nokia SR-SIM](https://www.nokia.com/networks/products/service-router-operating-system/) containerized router is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). It is a fully containerized router that replaces the Virtual Machine based SR-SROS simulator.

The Containerized Service Router Simulatior, known as the SR-SIM, is a containerized version of the SR OS software that simulates the software that runs on the hardware platforms and it is available to Nokia customers who have an active SR-SIM subscription. The SR-SIM tool emulates a number of hardware routers. These routers are either pizza-box systems with integrated linecards, or chassis-based systems with multiple linecards per chassis. The operator can model both types of devices. This tool is provided as a container and designed to run on an x86 system within common container runtimes.

The configuration of hardware elements (such as provisioning line cards) and software elements (such as interfaces, network protocols, and services) is performed the same way as on the physical SR OS platforms with each linecard running as a separate container for emulation of multi-linecard systems (Distributed model).  Pizza-box systems with integrated linecards run in an integrated model with one container per emulated system.

Nokia SR-SIM nodes launched with containerlab come up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## Managing Nokia SR OS nodes

!!!note

Nokia SR OS node launched with containerlab can be managed via the following interfaces:

/// tab | bash
to connect to a `bash` shell of a running Nokia SR OS container:

```bash
docker exec -it <container-name/id> bash
```

///
/// tab | CLI
to connect to the SR OS CLI

```bash
ssh admin@<container-name/id>
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
--host <node-name> --port 830 -u admin -p 'admin' \
--hello
```

///
/// tab | gNMI
using the best in class [gnmic](https://gnmic.openconfig.net) gNMI client as an example:

```bash
gnmic -a <container-name/node-mgmt-address> --insecure \
-u admin -p admin \
capabilities
```

/// note
Default user credentials: `admin:admin`
///

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in -{{ kind_display_name }}-.

The interface naming convention is typically: `1/1/cN/M`, where `N` is the cage or connector number and `M` is the breakout port inside the port connector.

/// admonition
    type: warning
Nokia SR-SIM support other interface names, the format will be one of the following:
```
e1-2-3       -> card 1, mda 2, port 3
e1-2-c3-1    -> card 1, mda 2, connector 3, port 1
e2-2-c3-4    -> card 2, mda 2, connector 3, port 4
e1-x2-3-4    -> card 1, xiom 2, mda 3, port 4
e1-x2-3-c4-5 -> card 1, xiom 2, mda 3, connector 4, port 5
```
Data port numbering starts at `1`, like one would normally expect in the NOS.
///


The mgmt interface for the SR-SIM will be typically mapped to the `eth0` of the Linux namespace where the container is running, distributed systems might require attachments to the "network fabric" which in our case can be a bridge with arbitrary interface naming as long as they are unique, e.g.  `eth1, eth2, etc` 

When containerlab launches -{{ kind_display_name }}- node the primary BOF interface gets assigned an address given by the container runtime. This interface is In case of emulating a device with mutiple CPMs, the address will only be allocated to the active CPM.

Data interfaces need to be configured with IP addressing manually using CLI or other available management interfaces.

Nokia SR OS container uses the following mapping for its interfaces:

Interfaces can be defined in a non-sequential way, for example:

```yaml
  links:
    # sr-sim port 1 on LC1 is connected to sr-sim port 1 on LC2
    - endpoints: ["sr-14s-1:e1-1-c1-1", "sr-14s-2:e2-x1-1-c1-1"]
    # sr-sim port 1 on LC1/MDA2 is connected to sr-sim port 1 on LC3/MDA1
    - endpoints: ["sr-14s-1:e1-2-c1-1", "sr-14s-2:e3-1-c1-1"]


```

1. Or `endpoints: ["sr1:eth3", "sr2:eth5"]` in the Linux interface naming scheme.

## Features and options

### Variants

SR OS container simulator can be run in multiple HW variants as explained in [the cSIM installation guide](TBD).

Nokia SR OS container images can emulate any variant and use enviromental variables to change the default behavior of a given container

To make Nokia SR OS to boot in one of the packaged variants, set the type to one of the predefined variant values:

```yaml
topology:
  nodes:
    sros-14s-a:
      kind: nokia_srsim
      image: nokia_srsim:25.7.R1
      type: sr-14s # if omitted, the default sr-1 variant will be used
      license: license-sros25.txt
      kind: nokia_srsim
      type: SR-14s
      network-mode: container:sr-14s-a
      env:
        NOKIA_SROS_SLOT: A 
    sros-14s-1:
      kind: nokia_srsim
      image: nokia_srsim:25.7.R1
      type: sr-14s 
      license: license-sros25.txt
      kind: nokia_srsim
      type: SR-14s
      network-mode: container:sr-14s-a
      env:
        NOKIA_SROS_SLOT: 1 
```

#### Custom variants

A custom variant can be defined by specifying enviromental variables for the linecards or specific chassis

```yaml
  nodes:
    sros-2se-1:
      kind: nokia_srsim
      image: nokia_srsim:25.7.R1
      type: sr-2se 
      license: license-sros25.txt
      kind: nokia_srsim
      type: SR-14s
      network-mode: container:sr-14s-a
      env:
        NOKIA_SROS_SLOT: 1 
        NOKIA_SROS_CARD: xcm-2se
        NOKIA_SROS_MDA_1: x2-s36-800g-qsfpdd-18.0t
```

1. for distributed chassis, a `node` container needs to be defined for each CPM and IOM. 
```yaml
  nodes:
    sr-sim1: 
      kind: nokia_srsim
      type: SR-1x-92S
      startup-config: config.cfg
      env: 
         NOKIA_SROS_SLOT: A
    sr-sim1-iom:
      kind: nokia_srsim
      type: SR-1x-92S
      network-mode: container:sr-sim1
      env:
        NOKIA_SROS_SLOT: 1 
```

/// details | How to define links in a multi line card setup?
    type: tip
When a node uses multiple line cards users should pay special attention to the way links are defined in the topology file. As explained in the [interface naming](#interface-naming) section, SR OS nodes will need to be mapped to the linecard, xiom, mda or port they use, therefore the endpoints array need to indicate the linecard where the connections are made.

Another significant value of a line card definition is the `slot` position. Line cards are inserted into slots, and slot 1 comes before slot 2, and so on.

Knowing the slot number and the maximum number of ports a line card has, users can identify which indexes they need to use in the `link` portion of a topology to address the right port of a chassis. Let's use the following example topology to explain how this all maps together:

```yaml
  nodes:
    sr-sim1:
      kind: nokia_srsim
    sr-sim2: 
      kind: nokia_srsim
      type: SR-1x-92S
      startup-config: config.cfg
      env: 
         NOKIA_SROS_SLOT: A
    sr-sim2-iom:
      kind: nokia_srsim
      type: SR-1x-92S
      network-mode: container:sr-sim2
      env:
        NOKIA_SROS_SLOT: 1 
    sr-sim3:
      kind: nokia_srsim
      type: SR-1x-92S
      startup-config: config.cfg
      env: 
         NOKIA_SROS_SLOT: A
    sr-sim3-iom:
      kind: nokia_srsim
      type: SR-1x-92S
      network-mode: container:sr-sim3
      env: 
         NOKIA_SROS_SLOT: 1


  links:
    # Data Interfaces
    - endpoints: ["sr-sim1:e1-1-c1-1", "sr-sim2-iom:e1-1-c1-1"]    
    - endpoints: ["sr-sim1:e1-1-c2-1", "sr-sim3-iom:e1-1-c1-1"]    
    - endpoints: ["sr-sim2-iom:e1-1-c22-1", "sr-sim3-iom:e1-1-c22-1"]


```

Starting with the first pair of endpoints `sr-sim1:e1-1-c1-1 <--> sr-sim2-iom:e1-1-c1-1`; we see that port1 of SR-SIM1 is connected with port1 of SR-SIM2 IOM-1. 
WIP HERE
The second pair of endpoints `R1:eth7 <--> eth8:R2` addresses the ports on a line card equipped in the slot 2. This is driven by the fact that the first six interfaces belong to line card in slot 1 as we just found out. This means that our second line card that sits in slot 2 and has as well six ports, will be addressed by the interfaces `eth7` till `eth12`, where `eth7` is port1 and `eth12` is port6.
///
An integrated variant is provided with a simple TIMOS line:

```yaml
type: "cpu=2 ram=4 slot=A chassis=ixr-r6 card=cpiom-ixr-r6 mda/1=m6-10g-sfp++4-25g-sfp28" # (1)!
```

1. No `cp` nor `lc` markers are needed to define an integrated variant.

### Node configuration

Nokia SR OS nodes come up with a basic "blank" configuration where only the card/mda are provisioned, as well as the management interfaces such as Netconf, SNMP, gNMI.

#### User-defined config

SR OS nodes launched with [hellt/vrnetlab](https://github.com/hellt/vrnetlab) come up with some basic configuration that configures the management interfaces, line cards, mdas and power modules. This configuration is applied right after the node is booted.

Since this initial configuration is meant to provide a bare minimum configuration to make the node operational, users will likely want to apply their own configuration to the node to enable some features or to configure some interfaces. This can be done by providing a user-defined configuration file using [`startup-config`](../nodes.md#startup-config) property of the node/kind.

/// tip
Configuration text can contain Go template logic as well as make use of [environment variables](../topo-def-file.md#environment-variables) allowing for runtime customization of the configuration.
///

##### Full startup-config

When a user provides a path to a file that has a complete configuration for the node, containerlab will copy that file to the lab directory for that specific node under the `/tftpboot/config.txt` name and mount that dir to the container. This will result in this config to act as a startup-config for the node:

```yaml
name: sros_lab
topology:
  nodes:
    sros:
      kind: nokia_sros
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
      kind: nokia_sros
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
      kind: nokia_sros
      startup-config: https://gist.com/<somehash>/staticroute.partial.cfg
```

###### Embedded partial files

Users can also embed the partial config in the topology file itself, making it a hermetic artifact that can be shared with others. This can be done by using multiline string in YAML:

```yaml
name: sros_lab
topology:
  nodes:
    sros:
      kind: nokia_sros
      startup-config: | #(1)!
        /configure system location "I am an embedded config"
```

1. It is mandatory to use YAML's multiline string syntax to denote that the string below is a partial config and not a file.

Embedded partial configs will persist on containerlab's host and use the same directory as the [remote startup-config](../config-mgmt.md#remote) files.

#### Configuration save

Containerlab's [`save`](../../cmd/save.md) command will perform a configuration save for `Nokia SR OS` nodes via Netconf. The configuration will be saved under `config.txt` file and can be found at the node's directory inside the lab parent directory:

```bash
# assuming the lab name is "cert01"
# and node name is "sr"
cat clab-cert01/sr/tftpboot/config.txt
```

#### Boot Options File

By default `nokia_sros` nodes boot up with a pre-defined "Boot Options File" (BOF). This file includes boot settings including:

* license file location
* config file location

When the node is up and running you can make changes to this BOF. One popular example of such changes is the addition of static-routes to reach external networks from within the SR OS node. Although you can save the BOF from within the SROS system, the location the file is written to is not persistent across container restarts. It is also not possible to define a BOF target location.  
A workaround for this limitation is to automatically execute a CLI script that configures BOF once the system boots.

SR OS has an option (introduced in SR OS 16.0.R1) to automatically execute a script upon successful boot. This option is accessible in SR OS by the `/configure system boot-good-exec` MD-CLI path:

```bash
[pr:/configure]
A:admin@sros1# system boot-good-exec ?

 boot-good-exec <string>
 <string>  - <1..180 characters>

    CLI script file to execute following successful boot-up
```

By mounting a script to SR OS container node and using the `boot-good-exec` option, users can make changes to the BOF the second the node boots and thus complete the task of having a *somewhat* persistent BOF.

As an example the following SR OS MD-CLI script was created to persist custom static routes to the BOF:

```bash
########################################
# Configuring static management routes
########################################
/bof private
router "management" static-routes route 10.0.0.0/24 next-hop 172.31.255.29
router "management" static-routes route 10.0.1.0/24 next-hop 172.31.255.29
router "management" static-routes route 192.168.0.0/24 next-hop 172.31.255.29
router "management" static-routes route 172.20.20.0/24 next-hop 172.31.255.29
commit
exit all
```

This script is then placed somewhere on the disk, for example in the containerlab's topology root directory, and mounted to `nokia_sros` node tftpboot directory using [binds](../nodes.md#binds) property:

```yaml
  nodes:
    sros1:
      mgmt-ipv4: [mgmt-ip]
      kind: nokia_sros
      image: [container-image-repo]
      type: sr-1s
      license: license-sros.txt
      binds:
        - post-boot-exec.cfg:/tftpboot/post-boot-exec.cfg #(1)!
```

1. `post-boot-exec.cfg` file contains the script referenced above and it is mounted to `/tftpboot` directory that is available in SR OS node.

Once the script is mounted to the node, users need to instruct SR OS to execute the script upon successful boot. This is done by adding the following configuration line on SR OS MD-CLI:

```bash
[pr:/configure system]
A:admin@sros1# info | match boot-goo
    boot-good-exec "tftp://172.31.255.29/post-boot-exec.cfg" #(1)!
```

1. The tftpboot location is always at `tftp://172.31.255.29/` address and the name of the file needs to match the file you used in the binds instruction.

By combining file bindings and the automatic script execution of SROS it is possible to create a workaround for persistent BOF settings.

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
