# Nokia SR Linux

[Nokia SR Linux](https://www.nokia.com/networks/products/service-router-linux-NOS/) NOS is identified with `srl` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a node.

## Managing SR Linux nodes
There are many ways to manage SR Linux nodes, ranging from classic CLI management all the way up to the gNMI programming.

=== "bash"
    to connect to a `bash` shell of a running SR Linux container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI/SSH"
    to connect to the SR Linux CLI
    ```bash
    docker exec -it <container-name/id> sr_cli
    ```  
    or with SSH `ssh admin@<container-name>`
=== "gNMI"
    using the best in class [gnmic](https://gnmic.kmrd.dev) gNMI client as an example:
    ```bash
    gnmic -a <container-name/node-mgmt-address> --skip-verify \
    -u admin -p admin \
    -e json_ietf \
    get --path /system/name/host-name
    ```
=== "JSON-RPC"
    SR Linux has a JSON-RPC interface over port 80/443 for HTTP/HTTPS schemas accordingly.

    HTTPS server uses the same TLS certificate as gNMI server.

!!!info
    Default credentials: `admin:admin`  
    Containerlab will automatically enable public-key authentication for `root`, `admin` and `linuxadmin` users if public key files are found at `~/.ssh` directory[^1].

## Interfaces mapping
SR Linux system expects interfaces inside the container to be named in a specific way - `eX-Y` - where `X` is the line card index, `Y` is the port.

With that naming convention in mind:

* `e1-1` - first ethernet interface on a line card #1
* `e1-2` - second interface on a line card #1
* `e2-1` - first interface on a line card #1

These interface names are seen in the Linux shell; however, when configuring the interfaces via SR Linux CLI, the interfaces should be named as `ethernet-X/Y` where `X/Y` is the `linecard/port` combination.

Interfaces can be defined in a non-sequential way, for example:

```yaml
  links:
    # srlinux port ethernet-1/5 is connected to sros port 2
    - endpoints: ["srlinux:e1-5", "sros:eth2"]
```

### Breakout interfaces
If the interface is intended to be configured as a breakout interface, its name must be changed accordingly.

The breakout interfaces will have the name `eX-Y-Z` where `Z` is the breakout port number. For example, if interface `ethernet-1/3` on an IXR-D3 system is meant to act as a breakout 100Gb to 4x25Gb, then the interfaces in the topology must use the following naming:

```yaml
  links:
    # srlinux's first breakout port ethernet-1/3/1 is connected to sros port 2
    - endpoints: ["srlinux:e1-3-1", "sros:eth2"]
```

## Features and options
### Types
For SR Linux nodes [`type`](../nodes.md#type) defines the hardware variant that this node will emulate.

The available type values are: `ixr6`, `ixr10`, `ixrd1`, `ixrd2`, `ixrd3`, `ixrd2l`, `ixrd3l`, `ixrh2` and `ixrh3`, which correspond to a hardware variant of Nokia 7250/7220 IXR chassis.

By default, `ixrd2` type will be used by containerlab.

Based on the provided type, containerlab will generate the topology file that will be mounted to the SR Linux container and make it boot in a chosen HW variant.
### Node configuration
SR Linux uses a `/etc/opt/srlinux/config.json` file to persist its configuration. By default, containerlab starts nodes of `srl` kind with a basic "default" config, and with the `startup-config` parameter, it is possible to provide a custom config file that will be used as a startup one.

#### Default node configuration
When a node is defined without the `startup-config` statement present, containerlab will make [additional configurations](https://github.com/srl-labs/containerlab/blob/master/nodes/srl/srl.go#L38) on top of the factory config:

```yaml
# example of a topo file that does not define a custom startup-config
# as a result, the default configuration will be used by this node

name: srl_lab
topology:
  nodes:
    srl1:
      kind: srl
      type: ixr6
```

The generated config will be saved by the path `clab-<lab_name>/<node-name>/config/config.json`. Using the example topology presented above, the exact path to the config will be `clab-srl_lab/srl1/config/config.json`.

Additional configurations that containerlab adds on top of the factory config:

* enabling interfaces (`admin-state enable`) referenced in the topology's `links` section
* enabling LLDP
* enabling gNMI/JSON-RPC
* creating tls server certificate

#### User defined startup config
It is possible to make SR Linux nodes boot up with a user-defined config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind a user sets the path to the local config file that will be used as a startup config.

The startup configuration file can be provided in two formats:

* full SR Linux config in JSON format
* partial config in SR Linux CLI format

##### CLI
A typical lab scenario is to make nodes to boot with a pre-configured use case. The easiest way to do that is to capture the intended changes as CLI commands.

On SR Linux, users can configure the use case and output the changes in the form of `set` instructions with the `info flat` command. These CLI commands can be saved in a file and used as a startup configuration.

???info "flat config CLI example"
    this can be `myconfig.cli` referred below
    ```bash
    set / network-instance default protocols bgp admin-state enable
    set / network-instance default protocols bgp router-id 10.10.10.1
    set / network-instance default protocols bgp autonomous-system 65001
    set / network-instance default protocols bgp group ibgp ipv4-unicast admin-state enable
    set / network-instance default protocols bgp group ibgp export-policy export-lo
    set / network-instance default protocols bgp neighbor 192.168.1.2 admin-state enable
    set / network-instance default protocols bgp neighbor 192.168.1.2 peer-group ibgp
    set / network-instance default protocols bgp neighbor 192.168.1.2 peer-as 65001
    ```


```yaml
name: srl_lab
topology:
  nodes:
    srl1:
      kind: srl
      type: ixr6
      image: ghcr.io/nokia/srlinux
      # a path to the partial config in CLI format relative to the current working directory
      startup-config: myconfig.cli
```

In that case, SR Linux will first boot with the default configuration, and then the CLI commands from the `myconfig.cli` will be applied.

##### JSON
SR Linux persists its configuration as a JSON file that can be found by the `/etc/opt/srlinux/config.json` path. Users can use this file as a startup configuration like that:

```yaml
name: srl_lab
topology:
  nodes:
    srl1:
      kind: srl
      type: ixr6
      image: ghcr.io/nokia/srlinux
      # a path to the full config in JSON format relative to the current working directory
      startup-config: myconfig.json
```

Containerlab will take the `myconfig.json` file, copy it to the lab directory for that specific node under the `config.json` name, and mount that directory to the container. This will result in this config acting as a startup-config for the node.

#### Saving configuration
As was explained in the [Node configuration](#node-configuration) section, SR Linux containers can make their config persistent because config files are provided to the containers from the host via the bind mount.

When a user configures the SR Linux node, the changes are saved into the running configuration stored in memory. To save the running configuration as a startup configuration, the user needs to execute the `tools system configuration save` CLI command. This command will write the config to the `/etc/opt/srlinux/config.json` file that holds the startup-config and is exposed to the host.

SR Linux node also supports the [`containerlab save -t <topo-file>`](../../cmd/save.md) command, which will execute the command to save the running-config on all lab nodes. For SR Linux node, the `tools system configuration save` will be executed:

```
❯ containerlab save -t quickstart.clab.yml
INFO[0000] Parsing & checking topology file: quickstart.clab.yml
INFO[0001] saved SR Linux configuration from leaf1 node. Output:
/system:
    Saved current running configuration as initial (startup) configuration '/etc/opt/srlinux/config.json'

INFO[0001] saved SR Linux configuration from leaf2 node. Output:
/system:
    Saved current running configuration as initial (startup) configuration '/etc/opt/srlinux/config.json'
```

#### User defined custom agents for SR Linux nodes
SR Linux supports custom "agents", i.e. small independent pieces of software that extend the functionality of the core platform and integrate with the CLI and the rest of the system. To deploy an agent, a YAML configuration file must be placed under `/etc/opt/srlinux/appmgr/`. This feature adds the ability to copy agent YAML file(s) to the config directory of a specific SRL node, or all such nodes.

```yaml
name: srl_lab_with_custom_agents
topology:
  nodes:
    srl1:
      kind: srl
      ...
      extras:
        srl-agents:
        - path1/my_custom_agent.yml
        - path2/my_other_agent.yml
```

### TLS
By default, containerlab will generate TLS certificates and keys for each SR Linux node of a lab. The TLS related files that containerlab creates are located in the so-called CA directory, which can be found by the `<lab-directory>/ca/` path. Here is a list of files that containerlab creates relative to the CA directory

1. Root CA certificate - `root/root-ca.pem`
2. Root CA private key - `root/root-ca-key.pem`
3. Node certificate - `<node-name>/<node-name>.pem`
4. Node private key - `<node-name>/<node-name>-key.pem`

The generated TLS files will persist between lab deployments. This means that if you destroyed a lab and deployed it again, the TLS files from initial lab deployment will be used.

In case a user-provided certificates/keys need to be used, the `root-ca.pem`, `<node-name>.pem` and `<node-name>-key.pem` files must be copied by the paths outlined above for containerlab to take them into account when deploying a lab.

In case only `root-ca.pem` and `root-ca-key.pem` files are provided, the node certificates will be generated using these CA files.

### License
SR Linux container can run without any license :partying_face:.  
In that license-less mode the datapath is limited to 100PPS and the sr_linux process will reboot once a week.

The license file lifts these limitations and a path to it can be provided with [`license`](../nodes.md#license) directive.

## Container configuration
To start an SR Linux NOS containerlab uses the configuration that is described in [SR Linux Software Installation Guide](https://documentation.nokia.com/cgi-bin/dbaccessfilename.cgi/3HE16113AAAATQZZA01_V1_SR%20Linux%20R20.6%20Software%20Installation.pdf)

=== "Startup command"
    `sudo bash -c /opt/srlinux/bin/sr_linux`
=== "Syscalls"
    ```
    net.ipv4.ip_forward = "0"
    net.ipv6.conf.all.disable_ipv6 = "0"
    net.ipv6.conf.all.accept_dad = "0"
    net.ipv6.conf.default.accept_dad = "0"
    net.ipv6.conf.all.autoconf = "0"
    net.ipv6.conf.default.autoconf = "0"
    ```
=== "Environment variables"
    `SRLINUX=1`

### File mounts
When a user starts a lab, containerlab creates a lab directory for storing [configuration artifacts](../conf-artifacts.md). For `srl` kind, containerlab creates directories for each node of that kind.

```
~/clab/clab-srl02
❯ ls -lah srl1
drwxrwxrwx+ 6 1002 1002   87 Dec  1 22:11 config
-rw-r--r--  1 root root  233 Dec  1 22:11 topology.clab.yml
```

The `config` directory is mounted to container's `/etc/opt/srlinux/` path in `rw` mode. It will contain configuration that SR Linux runs of as well as the files that SR Linux keeps in its `/etc/opt/srlinux/` directory:

```
❯ ls srl1/config
banner  cli  config.json  devices  tls  ztp
```

The topology file that defines the emulated hardware type is driven by the value of the kinds `type` parameter. Depending on a specified `type`, the appropriate content will be populated into the `topology.yml` file that will get mounted to `/tmp/topology.yml` directory inside the container in `ro` mode.

#### authorized keys
Additionally, containerlab will mount the `authorized_keys` file that will contain contents of every public key found in `~/.ssh` directory. This file will be mounted to `~/.ssh/authorized_keys` path for the following users:

* `root`
* `linuxadmin`
* `admin`

This will enable passwordless access for the users above if any public key is found in the user's directory.

[^1]: The `authorized_keys` file will be created with the content of all found public keys. This file will be bind-mounted using the respecting paths inside SR Linux to enable password-less access.
