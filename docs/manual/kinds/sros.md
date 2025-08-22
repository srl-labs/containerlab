---
search:
  boost: 4
kind_code_name: nokia_srsim
kind_display_name: Nokia SR-SIM
---
# Nokia SR OS

<small>**Native container**</small>

The [Nokia SR OS](https://www.nokia.com/networks/products/service-router-operating-system/) containerized router is identified with the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). It is a fully containerized router that replaces the legacy virtual machine-based SR OS simulator or [vSIM](vr-sros.md)[^1].

The containerized Service Router Simulator, known as SR-SIM, is a cloud-native version of the SR OS software that runs on hardware platforms. The image can be downloaded from the [Nokia Support Portal](https://customer.nokia.com/support/s/) and requires an active SR-SIM license to operate.

Once downloaded, the image needs to be loaded to the container runtime:

```bash
docker image load -i srsim.tar.xz #(1)!
```

1. After loading the image, you can optionally tag it to your own container registry and push it there for easier access in the future. For example:

    ```bash
    docker tag nokia_srsim:[version] your.registry.tld/nokia_srsim:[version]
    docker push your.registry.tld/nokia_srsim:[version]
    ```

When loaded, the image will be available as `nokia_srsim:[version]` in the local image store.

The SR-SIM container emulates various hardware routers: either pizza-box systems with integrated line cards or chassis-based systems with multiple line cards per chassis. Operators can model both types of devices.

> :material-cpu-64-bit: Nokia SR-SIM is provided as a container image and is designed to run natively on x86_64 systems with the common container runtimes such as Docker.  
> :material-apple: The SR-SIM image runs on macOS with arm64 architecture using Rosetta virtualization.

Hardware elements (such as line cards, PSUs, fans, etc.) and software elements (such as interfaces, network protocols, and services) are emulated and configured just like physical SR OS platforms. Each line card runs as a separate container for emulation of multi-line card systems (distributed model).  
Pizza-box systems with integrated line cards run in an integrated model with one container per emulated system.

## Managing Nokia SR OS nodes

Nokia SR-SIM nodes launched with containerlab are pre-provisioned with SSH, SNMP, NETCONF, and gNMI services enabled.

/// tab | CLI
Connect to the SR OS CLI:

```bash
ssh admin@<node-name/node-mgmt-address>
```

///
/// tab | NETCONF
NETCONF server is running over port 830

```bash
ssh admin@<node-name> -p 830 -s netconf
```

///
/// tab | gNMI
Using the best-in-class [gnmic](https://gnmic.openconfig.net) gNMI client as an example:

```bash
gnmic -a <container-name/node-mgmt-address> --insecure \
-u admin -p NokiaSros1! \
capabilities
```

///

### Credentials

Admin user credentials for the Nokia SR OS launched by Containerlab are:

* username: `admin`
* password: `NokiaSros1!`

> Note: the admin password is changed by Containerlab from the default `admin:admin` combination.

### Logs

Logs can be retrieved with standard log commands for the given container runtime:

```bash
$ docker logs -f clab-sros-sr-sim1
NOKIA_SROS_CHASSIS=SR-1
NOKIA_SROS_SYSTEM_BASE_MAC=1c:30:00:00:00:00

** Container version: 25.7.R1 (Built on Wed Jul 16 21:43:17 UTC 2025)


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

Containerlab' [interface alias](../topo-def-file.md#interface-naming) feature allows operators to use the interface names in the topology file in the same format as they appear in -{{ kind_display_name }}- configuration.  
The interface naming convention inside the SR OS command line is typically: `L/xX/M/cC/P`, `L/xX/M/P`, `L/M/cC/P` or `L/M/P` where:

* `L` : line card number
* `X` : xiom number (when present)
* `M` : MDA position
* `C` : cage or connector number
* `P` : breakout port inside the port connector.

Here is an example on how Nokia SR-SIM's interface names are mapped to the cards, mdas, and connectors:

```
1/2/3       -> card 1, mda 2, port 3
1/2/c3/1    -> card 1, mda 2, connector 3, port 1
2/2/c3/4    -> card 2, mda 2, connector 3, port 4
1/x2/3/4    -> card 1, xiom 2, mda 3, port 4
1/x2/3/c4/5 -> card 1, xiom 2, mda 3, connector 4, port 5
```

SR OS interface names can be directly used in containerlab topology files, in the `links` section of the topology file.

```yaml
links:
  - endpoints: ["sr-sim1:1/1/c1/1", "sr-sim2:1/1/1"]                           #(1)!
  - endpoints: ["sr-sim-dist-iom-1:1/1/c1/1", "sr-sim-dist-iom-2:2/x1/1/c1/1"] #(2)!
  - endpoints: ["sr-sim-dist-iom-1:1/2/c1/1", "sr-sim-dist-iom3:3/1/c1/1"]     #(3)!
```

1. sr-sim1 port 1/1/c1/1 on line card 1 is connected to sr-sim2 port 1/1/1 on line card 1
2. sr-sim port 1/1/c1/1 on line card 1 is connected to sr-sim port 2/x1/1/c1/1 on line card 2
3. sr-sim port 1/2/c1/1 on line card 1, MDA 2 is connected to sr-sim port 3/1/c1/1 on line card 3, MDA 1

> Inside the SR OS container, the interfaces are converted to match the Linux interface name conventions: `eL-xX-M-cC-P`, `eL-xX-M-P`, `eL-M-cC-P`, or `eL-M-P`.  
> Note that the prefix `e` is added at the beginning of the port, and the forward slash `/` is replaced with a hyphen `-`.  
> You would see these Linux-compatible interface names used when SR-SIM is launched outside of Containerlab, for example, with Docker Compose or Kubernetes.

The interfaces can also be non-sequential, like in the example below.

```yaml
links:
  - endpoints: ["sr-sim1:1/1/c1/1", "sr-sim2:1/1/1"]
  - endpoints: ["sr-sim-dist-iom-1:1/1/c1/1", "sr-sim-dist-iom-2:2/x1/1/c1/1"]
  - endpoints: ["sr-sim-dist-iom-1:1/2/c1/1", "sr-sim-dist-iom3:3/1/c1/1"]
```

The management interface for the SR-SIM is mapped to `eth0` of the Linux namespace where the container is running.

When containerlab launches the `-{{ kind_display_name }}-` node, the primary BOF interface gets an address provided by the container runtime's IPAM driver. This address will only be allocated to the active CPM.

Data interfaces need to be configured with IP addressing manually using the SR OS CLI or other available management methods.

## SR-SIM variants

The SR-SIM can emulate different hardware platforms as explained in the [SR-SIM Installation, deployment and setup guide](https://documentation.nokia.com/sr/25-7/7750-sr/titles/sr-sim-installation-setup.html). These variants can be set using the `type` directive in the clab topology file or by overriding the different available environment variables such as the ones for the chassis (`NOKIA_SROS_CHASSIS`) or card (`NOKIA_SROS_CARD`).  
Users can then use environment variables to change the default behavior of a given container. If there is a conflict between the `type` field in the topology file and an environment variable in the topology file, the environment variable will take precedence.

> When `type` is not provided in the topology file, SR-SIM will start as `SR-1` platform.

If the chosen platform is chassis-based, the SR-SIM deployment needs to be done in a [distributed variant](#distributed), where each CPM and line card is a separate container. Otherwise, the SR-SIM will run in an [integrated](#integrated) mode with a single container emulating the whole system.

### Integrated

We call non-chassis-based systems like SR-1, SR-1s integrated variants. As these systems have a fixed form factor, they run as a single container and are represented as a single node in the topology file.

Besides setting the `type` to drive the platform selection, users can then modify some of the default settings on a per-node basis using the environment variables. For example, to change the default MDA, like shown in the example below.
/// tab | Integrated SR-SIM

```yaml
topology:
  nodes:
    sr-sim:
      kind: nokia_srsim
      image: nokia_srsim:25.7.R1
      type: SR-1s # overriding the default SR-1 type with SR-1s
      license: /opt/nokia/sros/license.txt
```

///
/// tab | Integrated SR-SIM with override

```yaml
topology:
  nodes:
    sr-sim1:
      kind: nokia_srsim
      image: nokia_srsim:25.7.R1
      type: SR-1
      license: /opt/nokia/sros/license.txt
      env:
        NOKIA_SROS_MDA_1: me12-100gb-qsfp28 # override default MDA type in slot 1
```

///

### Distributed

When the emulated platform is chassis-based, like SR-7, SR-14s, etc., the SR-SIM node must be defined in a distributed mode in the topology file.

A distributed SR-SIM node consists of two or more containers with a specific role: CPM or IOM. A node can boot in either mode depending on the settings of the `NOKIA_SROS_SLOT` environment variable and the SR-SIM node type.  
There are several other variables that will modify the default settings for a simulated chassis (e.g. SFM, XIOM, MDA, etc.), so please check the [SR-SIM Installation guide](https://documentation.nokia.com/sr/25-7/7750-sr/titles/sr-sim-installation-setup.html) for a full list of options.

Containerlab provides two ways to define the distributed variant:

1. Using a separate containerlab node definition per line card ([standard topology](#standard-topology))
2. With a single node definition with the components grouped as a list ([grouped topology](#grouped-topology)). This mode is currently in a preview and its configuration might change in the future.

/// details | Distributed SR-SIM considerations
Distributed systems require certain settings given the nature of the SR-SIM simulator:

1. Containers must all run in the same Linux namespace. This is currently achieved using the `network-mode` directive in Containerlab[^2].
2. The containers sharing namespace are all bridged internally to an internally created switch, which is simply a Linux bridge with uniquely named interfaces. Users do not need to configure the switch unless they have a specific need to use the `NOKIA_SROS_FABRIC_IF` environment variable  to override the default interfaces [^3].
3. Datapath links for the SR-SIM node SHOULD[^4] be connected to the container emulating the specific line card.
///

#### Standard topology

The standard distributed topology is defined by creating a separate node definition for each line card or CPM. This allows users to define the type of each line card and set the environment variables for each container individually.

Below are the key requirements to satisfy in order for the nodes to boot successfully:

1. The `type` for a single box must be the same.
2. For a dual CPM chassis, the CPM containers need to have the `NOKIA_SROS_SYSTEM_BASE_MAC` set to the same value.
3. The `NOKIA_SROS_SLOT` variable needs to be set uniquely for every SR-SIM container.
4. For a particular SR-SIM node, all its containers must be attached to the same Linux namespace using the `network-mode: container:<container-name>` directive. In the below examples, the container associated with the CPM-A is used.
5. When a node uses multiple line cards, users should pay attention to the way links are defined in the topology file. As explained in the [interface naming](#interface-naming) section, SR OS nodes SHOULD be mapped to a line card, XIOM, MDA or a port they use.
6. Similarly, if the users modify the management or fabric interfaces, they must take special care when creating the necessary wiring to such interfaces.

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
/// tab | with links

Note, how in the `links` section the particular SR-SIM's line card node (sr-14s-1) is used as an endpoint.

```yaml
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license.txt
      image: nokia_srsim:25.7.R1
  nodes:
    sr-14s-a:
      kind: nokia_srsim
      type: sr-14s
      env:
        NOKIA_SROS_SLOT: A
        NOKIA_SROS_SYSTEM_BASE_MAC: 1c:56:07:00:03:01 
    sr-14s-b:
      kind: nokia_srsim
      type: sr-14s
      network-mode: container:sr-14s-a
      env:
        NOKIA_SROS_SLOT: B
        NOKIA_SROS_SYSTEM_BASE_MAC: 1c:56:07:00:03:01 
    sr-14s-1:
      kind: nokia_srsim
      type: sr-14s
      network-mode: container:sr-14s-a
      env:
        NOKIA_SROS_SLOT: 1
    sr-14s-2:
      kind: nokia_srsim
      type: sr-14s 
      network-mode: container:sr-14s-a
      env:
        NOKIA_SROS_SLOT: 2

    cpe:
      kind: linux
      image: alpine:3

  links:
    - endpoints: ["cpe:eth1", "sr-14s-1:1/1/c1/1"]
```

///

/// tab | with overrides

```yaml
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license.txt
      image: nokia_srsim:25.7.R1
  nodes:
    sr-2se-a:
      kind: nokia_srsim
      type: SR-2se
      env:
        NOKIA_SROS_SLOT: A
        NOKIA_SROS_SYSTEM_BASE_MAC: 1c:58:07:00:03:01 # override Chassis MAC
        NOKIA_SROS_FABRIC_IF: eth1 # override fabric itf
        NOKIA_SROS_SFM: sfm-2se # override SFM
        NOKIA_SROS_CARD: cpm-2se # override CPM
    sros-2se-1:
      kind: nokia_srsim
      type: SR-2se
      network-mode: container:sr-2se-a
      env:
        NOKIA_SROS_SLOT: 1
        NOKIA_SROS_FABRIC_IF: eth2 # override fabric itf
        NOKIA_SROS_SFM: sfm-2se # override SFM
        NOKIA_SROS_CARD: xcm-2se # override IOM
        NOKIA_SROS_MDA_1: x2-s36-800g-qsfpdd-18.0t # override MDA
```

///

#### Grouped topology

/// admonition
    type: warning
This feature is a PREVIEW and should be implemented carefully in your lab
///

Users can simplify the topology file with distributed SR-SIM nodes by using the `components` directive in the node definition. In this case, every member in the `components` section will result in a spawned container emulating the corresponding component type (CPM or IOM). Similar to the standard topology, overrides are supported per container by setting the `env` directive for each component or per node.

/// tab | Distributed grouped SR-SIM

```yaml
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license.txt
      image: nokia_srsim:25.7.R1
  nodes:
    sr-sim1:
      kind: nokia_srsim
      type: SR-7
      components:
        - slot: A
        - slot: B
        - slot: 1
        - slot: 2
```

///
/// tab | with overrides

```yaml
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license.txt
      image: nokia_srsim:25.7.R1
  nodes:
    sr-sim1:
      kind: nokia_srsim
      type: SR-7
      components:
        - slot: A # containers will be attached to this Linux NS
        - slot: B
        - slot: 1
          type: iom5-e # equivalent to override NOKIA_SROS_CARD
          env:
            NOKIA_SROS_SFM: m-sfm6-7/12
            NOKIA_SROS_MDA_1: me6-100gb-qsfp28
            NOKIA_SROS_MDA_2: me3-400gb-qsfpdd
        - slot: 2
          env:
            NOKIA_SROS_SFM: m-sfm6-7/12
            NOKIA_SROS_CARD: iom5-e # (1)!
            NOKIA_SROS_MDA_1: me6-100gb-qsfp28
            NOKIA_SROS_MDA_2: me16-25gb-sfp28+2-100gb-qsfp28
```

1. As an example, the card type is set here as an env var, instead of the `type` field like we did for slot 1.

///
/// tab | with links

```yaml
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license.txt
      image: nokia_srsim:25.7.R1
  nodes: 
    sr-sim1:
      kind: nokia_srsim
      type: SR-7
      components:
        - slot: A
        - slot: B
        - slot: 1
        - slot: 2
    sr-sim2:
      kind: nokia_srsim
      type: SR-7
      components:
        - slot: A
        - slot: B
        - slot: 1
        - slot: 2
  links:
    - endpoints: ["srsim1:e1-1-c1-1", "srsim2:e1-1-c1-1"] #(1)!
    - endpoints: ["srsim1:e2-1-c1-1", "srsim2:e2-1-c1-1"]
```

1. As an example, we use here the Linux-compatible interface names. In Containerlab you could've also used the SR OS interface names, like `srsim1:1/1/c1/1`.

///

When a distributed SR-SIM node is defined using `components`, we need to take into account the following:

1. Individual containers will be attached to the namespace of the 1st element of the `components` list: CPM-A in the above examples.
2. When changing a MDA or card type from its default value, the environment variables for card, SFM and MDA must be also included.
3. Links can be added referring to the node name. The same [interface naming](#interface-naming) convention holds for all SR-SIM nodes.

## Node configuration

Nokia SR OS nodes come up with a default configuration where only the management interfaces such as NETCONF, SNMP, and gNMI are provisioned[^5].

### User-defined config

SR-SIM nodes are launched with a basic configuration that provisions the management interfaces, and adds SSH keys.  This initial configuration is applied after boot along with some partial startup config, when present.

Since this configuration is intended to provide the bare minimum to make the node operational, users will usually want to apply their own configuration to enable the line cards, add features or configure interfaces. This can be done by providing a user-defined configuration file using [`startup-config`](../nodes.md#startup-config) property of the node/kind.

/// tip
Configuration text can contain Go template logic as well as make use of [environment variables](../topo-def-file.md#environment-variables) allowing for runtime customization of the configuration.
///

#### Full startup-config

When a user provides a path to a file that has a complete configuration for the node, containerlab will copy that file to the lab directory for that specific node under the `<node>/config/cf3/config.cfg` name and mount that directory to the container. This will result in this config to act as a startup-config for the node:

```yaml
name: sros
topology:
  nodes:
    sros:
      kind: nokia_srsim
      startup-config: myconfig.txt
```

/// note
With the above configuration, the node will boot with the configuration specified in `myconfig.txt`, no other configuration will be applied. You must provision interfaces, cards, power-shelves, etc. yourself. Also, if the default node password is changed, the `save` command will fail.
///

#### Partial startup-config

Quite often it is beneficial to have a partial configuration that will be applied on top of the default configuration that containerlab applies. For example, users might want to add card configuration and some services on top of the default configuration provided by containerlab and do not want to manage the full configuration file.

This can be done by providing a partial configuration file that will be applied on top of the default configuration. The partial configuration file must have `.partial` string in its name, for example, `myconfig.partial.txt`.

```yaml
name: sros_lab
topology:
  nodes:
    sros:
      kind: nokia_srsim
      startup-config: myconfig.partial.txt
```

The partial config can contain configuration in a MD-CLI syntax that is accepted in the configuration mode of the SR OS. The way partial config is applied appending the lines to the existing startup config.
Both `flat`, `full-context` and normal syntax can be used in the partial config file. For example, the following partial config file adds a static route to the node in the regular CLI syntax:

```text
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

##### Remote partial files

It is possible to provide a partial config file that is located on a remote HTTP(S) server. This can be done by providing a URL to the file. The URL must start with `http://` or `https://` and must point to a file that is accessible from the containerlab host.

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

##### Embedded partial files

Users can also embed the partial config in the topology file itself, making it an atomic artifact that can be shared with others. This can be done by using multiline string in YAML:

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

### Configuration save

Containerlab's [`save`](../../cmd/save.md) command will perform a configuration save for Nokia SR OS nodes via NETCONF. The configuration will be saved under `config.cfg` file and can be found at the node's directory inside the lab parent directory:

```bash
# assuming the lab name is "cert01"
# and node name is "sr"
cat clab-cert01/sr/config/cf3/config.cfg
```

### Boot Options File

By default `nokia_srsim` nodes boot up with a pre-defined "Boot Options File" (BOF). This file includes boot settings including:

* license file location
* config file location

Some common BOF options can also be controlled using environmental variables as specified in the SR-SIM user's guide.

### SSH keys

Containerlab supports SSH key injection into the Nokia SR OS nodes prior to deployment. First containerlab retrieves all public keys from `~/.ssh`[^6] directory and `~/.ssh/authorized_keys` file, then it retrieves public keys from the ssh agent if one is running.

Next, it will filter out public keys that are not of RSA/ECDSA type. The remaining valid public keys will be configured for the admin user of the Nokia SR OS node using key IDs from 32 downwards[^7] at startup. This will enable key-based authentication when you connect to the node.

## Packet Capture

Currently, a packet capture on the veth interfaces of the `-{{ kind_display_name }}-` will only display traffic at the ingress direction[^8]. In order to capture traffic bidirectionally, a user needs to create a [mirror service](https://documentation.nokia.com/sr/25-7/7750-sr/books/oam-diagnostics/mirror-services.html) in the SR OS configuration. A simple example topology using [bridges in container namespace](bridge.md#bridges-in-container-namespace) and mirror configuration is provided below for convenience.

/// tab | Topology with mirror service

```yaml
name: "sros"
mgmt:
  network: srsim_mgmt
  ipv4-subnet: 10.78.140.0/24
topology:
  kinds:
    nokia_srsim:
      license: /opt/nokia/sros/license-sros25.txt
      image: nokia_srsim:25.7.R1
  nodes:
    sr-sim10:
      kind: nokia_srsim
      type: SR-1 # Implicit default
    sr-sim11:
      kind: nokia_srsim
    # In-namespace bridges for mirroring:
    mirror|sr-sim10:
      kind: bridge
      network-mode: container:sr-sim10
    mirror|sr-sim11:
      kind: bridge
      network-mode: container:sr-sim11
  links:
    # Data Interfaces
    - endpoints: ["sr-sim10:1/1/c1/1", "sr-sim11:1/1/c1/1"]
    - endpoints: ["sr-sim10:1/1/c1/2", "sr-sim11:1/1/c1/2"]
    # Mirror port mapped to in-namespace bridge:
    - endpoints: ["sr-sim10:1/1/c1/3", "mirror|sr-sim10:mirror0"]
    - endpoints: ["sr-sim11:1/1/c1/3", "mirror|sr-sim11:mirror0"]

```

///
/// tab | SR OS Mirror configuration

```
/configure port 1/1/c1/3 admin-state enable
/configure port 1/1/c1/3 ethernet mode hybrid
/configure mirror mirror-dest "mirror0" admin-state enable
/configure mirror mirror-dest "mirror0" service-id 999
/configure mirror mirror-dest "mirror0" { sap 1/1/c1/3:0 }
/configure mirror mirror-source "mirror0" admin-state enable
/configure mirror mirror-source "mirror0" port 1/1/c1/1 ingress true
/configure mirror mirror-source "mirror0" port 1/1/c1/1 egress true
/configure mirror mirror-source "mirror0" port 1/1/c1/2 ingress true
/configure mirror mirror-source "mirror0" port 1/1/c1/2 egress true
```

///

/// tab | tcpdump example

```bash
$ sudo ip netns exec  clab-sros-sr-sim10  tcpdump -nnei mirror0 icmp
tcpdump: verbose output suppressed, use -v[v]... for full protocol decode
listening on mirror0, link-type EN10MB (Ethernet), snapshot length 262144 bytes

10:00:40.281090 aa:c1:ab:0b:55:94 > aa:c1:ab:d7:6e:ae, ethertype IPv4 (0x0800), length 98: 10.0.0.10 > 10.0.0.11: ICMP echo request, id 251, seq 16385, length 64
10:00:40.282415 aa:c1:ab:d7:6e:ae > aa:c1:ab:0b:55:94, ethertype IPv4 (0x0800), length 98: 10.0.0.11 > 10.0.0.10: ICMP echo reply, id 251, seq 16385, length 64
```

///

## License

Path to a valid license must be provided for all Nokia SR OS nodes with a [`license`](../nodes.md#license) directive. If no valid license is provided, the nodes will not complete the deployment phase.

## Filesystem mounts

When the user starts a lab, containerlab creates a node directory for storing [configuration artifacts](../conf-artifacts.md). For the -{{ kind_display_name }}-  kind containerlab creates a node directory where the license file and the initial config will be copied. The filesystem for the flash cards that contain the system is mounted under the `config` directory. This same filesystem is visible inside the CPM containers `/home/sros/flashX` directory when logging in via SHELL or using the `file` command utility via SR OS CLI.

/// tab | Host Filesystem View

```bash
$ tree srsim10
srsim10
├── A
│   └── config
│       ├── cf1
│       ├── cf2
│       └── cf3
│           ├── bof.cfg
│           ├── bootlog.txt
│           ├── config.cfg
│           ├── config.cfg.1
│           ├── i386-isa-aa.tim
│           ├── license.txt
│           ├── nvsys.info
│           ├── restcntr.txt
│           └── yang.tim
├── B
│   └── config ...
└── license.key
```

///

/// tab | Container SHELL Filesystem View

```bash
$ docker exec -it clab-sros-srsim10-A tree /home/sros/flash3
/home/sros/flash3
|-- bof.cfg
|-- bootlog.txt
|-- config.cfg
|-- config.cfg.1
|-- i386-isa-aa.tim
|-- license.txt
|-- nvsys.info
|-- restcntr.txt
`-- yang.tim
```

///

/// tab | SR OS CLI Filesystem View

```
[/]
A:admin@srsim10-A# file list

Volume in drive cf3 on slot A has no label.

Directory of cf3:\

07/15/2025  04:57p      <DIR>          .commit-history/
07/15/2025  04:57p                 264 bof.cfg
07/15/2025  04:57p                2498 bootlog.txt
07/15/2025  04:57p               14649 config.cfg
07/15/2025  04:57p               13722 config.cfg.1
07/15/2025  04:57p             8009312 i386-isa-aa.tim
07/15/2025  04:57p                1014 license.txt
07/15/2025  04:57p                 321 nvsys.info
07/15/2025  04:57p                   1 restcntr.txt
07/15/2025  04:57p             7778672 yang.tim
               9 File(s)               15820453 bytes.
               1 Dir(s)            643914854400 bytes free.

```

///

## Lab examples

The following labs feature Nokia SR OS (SR-SIM) node:

* [SR Linux and SR OS](../../lab-examples/sr-sim.md)

[^1]: Support for the containerized SR-SIM is first introduced in containerlab v0.69.0.
[^2]: There are some caveats to this, for instance, if the container referred by the `network-mode` directive is stopped for any reason, all the other depending containers will stop working properly.
[^3]: If needed, switches can be created using the clab kind `bridge` or using `iproute2` commands. MTU needs to be set to 9000 at least.
[^4]: The word SHOULD is interpreted as [RFC2129](https://datatracker.ietf.org/doc/html/rfc2119) and [RFC8174](https://datatracker.ietf.org/doc/html/rfc8174). Links will come up as long as they are attached to the same Linux namespace.
[^5]: This is a change from the [Vrnetlab](../vrnetlab.md) based vSIM where line cards and MDAs were pre-provisioned for some cases.
[^6]: `~` is the home directory of the user that runs containerlab.
[^7]: If a user wishes to provide a custom startup-config with public keys defined, then they should use key IDs from 1 onwards. This will minimize chances of key ID collision causing containerlab to overwrite user-defined keys.
[^8]: See Github issue [#2741](https://github.com/srl-labs/containerlab/issues/2741)
