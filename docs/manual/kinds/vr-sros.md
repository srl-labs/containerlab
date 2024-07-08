---
search:
  boost: 4
kind_code_name: nokia_sros
kind_display_name: Nokia SR OS
---
# Nokia SR OS

[Nokia SR OS](https://www.nokia.com/networks/products/service-router-operating-system/) virtualized router is identified with `[[[ kind_code_name ]]]` kind in the [topology file](../topo-def-file.md). It is built using [vrnetlab](../vrnetlab.md) project and essentially is a Qemu VM packaged in a docker container format.

Nokia SR OS nodes launched with containerlab come up pre-provisioned with SSH, SNMP, NETCONF and gNMI services enabled.

## Managing Nokia SR OS nodes

!!!note
    Containers with SR OS inside will take ~3min to fully boot.  
    You can monitor the progress with `watch docker ps` waiting till the status will change to `healthy`.

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
/// tab | "NETCONF"
NETCONF server is running over port 830

```bash
ssh root@<container-name> -p 830 -s netconf
```

///
/// tab | "gNMI"
using the best in class [gnmic](https://gnmic.kmrd.dev) gNMI client as an example:

```bash
gnmic -a <container-name/node-mgmt-address> --insecure \
-u admin -p admin \
capabilities
```

///
/// tab | "Telnet"
serial port (console) is exposed over TCP port 5000:

```bash
# from container host
telnet <node-name> 5000
```  

You can also connect to the container and use `telnet localhost 5000` if telnet is not available on your container host.
///

/// note
Default user credentials: `admin:admin`
///

## Interface naming

You can use [interfaces names](../topo-def-file.md#interface-naming) in the topology file like they appear in [[[ kind_display_name ]]].

The interface naming convention is: `1/1/X`, where `X` is the port number.

/// admonition
    type: warning
Nokia SR OS nodes currently only support the simplified interface alias `1/1/X`, where X denotes the port number.  
Multi-chassis, multi-linecard setups, and channelized interfaces are not supported by interface aliasing at the moment, and you must fall back to the old `ethX`-based naming scheme ([see below](#custom-variants)) in these scenarios.

Data port numbering starts at `1`, like one would normally expect in the NOS.
///

With that naming convention in mind:

* `1/1/1` - first data port available
* `1/1/2` - second data port, and so on...

The example ports above would be mapped to the following Linux interfaces inside the container running the [[[ kind_display_name ]]] VM:

* `eth0` - management interface connected to the containerlab management network
* `eth1` - first data interface, mapped to the first data port of the VM (rendered as `1/1/1`)
* `eth2+` - second and subsequent data interfaces, mapped to the second and subsequent data ports of the VM (rendered as `1/1/2` and so on)

When containerlab launches [[[ kind_display_name ]]] node the primary BOF interface gets assigned `10.0.0.15/24` address from the QEMU DHCP server. This interface is transparently stitched with container's `eth0` interface such that users can reach the management plane of the [[[ kind_display_name ]]] using containerlab's assigned IP.

Data interfaces `1/1/1+` need to be configured with IP addressing manually using CLI or other available management interfaces.

Nokia SR OS container uses the following mapping for its interfaces:

Interfaces can be defined in a non-sequential way, for example:

```yaml
  links:
    # sr1 port 3 is connected to sr2 port 5
    - endpoints: ["sr1:1/1/3", "sr2:1/1/5"] #(1)!
```

1. Or `endpoints: ["sr1:eth3", "sr2:eth5"]` in the Linux interface naming scheme.

## Features and options

### Variants

Virtual SR OS simulator can be run in multiple HW variants as explained in [the vSIM installation guide](https://documentation.nokia.com/cgi-bin/dbaccessfilename.cgi/3HE15836AAADTQZZA01_V1_vSIM%20Installation%20and%20Setup%20Guide%2020.10.R1.pdf).

Nokia SR OS container images come with [pre-packaged SR OS variants](https://github.com/hellt/vrnetlab/tree/master/sros#variants) as defined in the upstream repo as well as support [custom variant definition](https://github.com/hellt/vrnetlab/tree/master/sros#custom-variant). The pre-packaged variants are identified by the variant name and come up with cards and mda already configured. On the other hand, custom variants give users total flexibility in emulated hardware configuration, but cards and MDAs must be configured manually.

To make Nokia SR OS to boot in one of the packaged variants, set the type to one of the predefined variant values:

```yaml
topology:
  nodes:
    sros:
      kind: nokia_sros
      image: vrnetlab/nokia_sros:20.10.R1
      type: sr-1s # if omitted, the default sr-1 variant will be used
      license: license-sros20.txt
```

#### Custom variants

A custom variant can be defined by specifying the *TIMOS line* for the control plane and line card components:

```yaml
type: >- # (1)!
  cp: cpu=2 ram=4 chassis=ixr-e slot=A card=cpm-ixr-e ___
  lc: cpu=2 ram=4 max_nics=34 chassis=ixr-e slot=1 card=imm24-sfp++8-sfp28+2-qsfp28 mda/1=m24-sfp++8-sfp28+2-qsfp28
```

1. for distributed chassis CPM and IOM are indicated with markers `cp:` and `lc:`.

    notice the delimiter string `___` that **must** be present between CPM and IOM portions of a custom variant string.

    `max_nics` value **must** be set in the `lc` part and specifies a maximum number of network interfaces this card will be equipped with.

    Memory `mem` is provided in GB.

It is possible to define a custom variant with multiple line cards; just repeat the `lc` portion of a type. Note that each line card is a separate VM, increasing pressure on the host running such a node. You may see some issues starting multi line card nodes due to the VMs being moved between CPU cores unless a [cpu-set](../nodes.md#cpu-set) is used.

```yaml title="distributed chassis with multiple line cards"
type: >-
  cp: cpu=2 min_ram=4 chassis=sr-7 slot=A card=cpm5 ___
  lc: cpu=4 min_ram=4 max_nics=6 chassis=sr-7 slot=1 card=iom4-e mda/1=me6-10gb-sfp+ ___
  lc: cpu=4 min_ram=4 max_nics=6 chassis=sr-7 slot=2 card=iom4-e mda/1=me6-10gb-sfp+
```

/// details | How to define links in a multi line card setup?
    type: tip
When a node uses multiple line cards users should pay special attention to the way links are defined in the topology file. As explained in the [interface naming](#interface-naming) section, SR OS nodes use `ethX` notation for their interfaces, where `X` denotes a port number on a line card/MDA.

Things get a little more tricky when multiple line cards are provided. First, every line card must be defined with a `max_nics` property that serves a simple purpose - identify how many ports at maximum this line card can bear. In the example above both line cards are equipped with the same IOM/MDA and can bear 6 ports at max. Thus, `max_nics` is set to 6.

Another significant value of a line card definition is the `slot` position. Line cards are inserted into slots, and slot 1 comes before slot 2, and so on.

Knowing the slot number and the maximum number of ports a line card has, users can identify which indexes they need to use in the `link` portion of a topology to address the right port of a chassis. Let's use the following example topology to explain how this all maps together:

```yaml
topology:
  nodes:
    R1:
      kind: nokia_sros
      image: nokia_sros:22.7.R2
      type: >-
        cp: cpu=2 min_ram=4 chassis=sr-7 slot=A card=cpm5 ___
        lc: cpu=4 min_ram=4 max_nics=6 chassis=sr-7 slot=1 card=iom4-e mda/1=me6-10gb-sfp+ ___
        lc: cpu=4 min_ram=4 max_nics=6 chassis=sr-7 slot=2 card=iom4-e mda/1=me6-10gb-sfp+
    R2:
      kind: nokia_sros
      image: nokia_sros:22.7.R2
      type: >-
        cp: cpu=2 min_ram=4 chassis=sr-7 slot=A card=cpm5 ___
        lc: cpu=4 min_ram=4 max_nics=6 chassis=sr-7 slot=1 card=iom4-e mda/1=me6-10gb-sfp+ ___
        lc: cpu=4 min_ram=4 max_nics=6 chassis=sr-7 slot=2 card=iom4-e mda/1=me6-10gb-sfp+

  links:
  - endpoints: ["R1:eth1", "R2:eth3"]
  - endpoints: ["R1:eth7", "R2:eth8"]
```

Starting with the first pair of endpoints `R1:eth1 <--> eth3:R2`; we see that port1 of R1 is connected with port3 of R2. Looking at the slot information and `max_nics` value of 6 we see that the linecard in slot 1 can host maximum 6 ports. This means that ports from 1 till 6 belong to the line card equipped in slot=1. Consequently, links ranging from `eth1` to `eth6` will address the ports of that line card.

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

SR OS nodes launched with hellt/vrnetlab come up with some basic configuration that configures the management interfaces, line cards, mdas and power modules. This configuration is applied right after the node is booted.

Since this initial configuration is meant to provide a bare minimum configuration to make the node operational, users will likely want to apply their own configuration to the node to enable some features or to configure some interfaces. This can be done by providing a user-defined configuration file using [`startup-config`](../nodes.md#startup-config) property of the node/kind.

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

!!!note
    With the above configuration, the node will boot with the configuration specified in `myconfig.txt`, no other configuration will be applied. You have to provision interfaces, cards, power-shelves, etc. yourself.

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

Embedded partial configs will persist on containerlab's host and use the same directory as the [remote startup-config](../nodes.md#remote-startup-config) files.

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

If your SR OS license file is issued for a specific UUID, you can define it with custom type definition:

```yaml
# note, typically only the cp needs the UUID defined.
type: "cp: uuid=00001234-5678-9abc-def1-000012345678 cpu=4 ram=6 slot=A chassis=SR-12 card=cpm5 ___ lc: cpu=4 ram=6 max_nics=36 slot=1 chassis=SR-12 card=iom3-xp-c mda/1=m10-1gb+1-10gb"
```

### File mounts

When a user starts a lab, containerlab creates a node directory for storing [configuration artifacts](../conf-artifacts.md). For Nokia SR OS kind containerlab creates `tftpboot` directory where the license file will be copied.

## Lab examples

The following labs feature Nokia SR OS node:

* [SR Linux and vr-sros](../../lab-examples/vr-sros.md)

[^1]: `~` is the home directory of the user that runs containerlab.
[^2]: If a user wishes to provide a custom startup-config with public keys defined, then they should use key IDs from 1 onwards. This will minimize chances of key ID collision causing containerlab to overwrite user-defined keys.
