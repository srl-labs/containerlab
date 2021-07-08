# Nokia SR Linux

[Nokia SR Linux](https://www.nokia.com/networks/products/service-router-linux-NOS/) NOS is identified with `srl` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a node.

## Managing SR Linux nodes
There are many ways to manage SR Linux nodes, ranging from classic CLI management all the way up to the gNMI programming. Here is a short summary on how to access those interfaces:

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

!!!info
    Default user credentials: `admin:admin`

## Features and options
### Types
For SR Linux nodes [`type`](../nodes.md#type) defines the hardware variant that this node will emulate.

The available type values are: `ixr6`, `ixr10`, `ixrd1`, `ixrd2`, `ixrd3` which correspond to a hardware variant of Nokia 7250/7220 IXR chassis.

By default, `ixr6` type will be used by containerlab.

Based on the provided type, containerlab will generate the [topology file](#topology-file) that will be mounted to SR Linux container and make it boot in a chosen HW variant.
### Node configuration
SR Linux nodes have a dedicated [`config`](#config-directory) directory that is used to persist the configuration of the node. It is possible to launch nodes of `srl` kind with a basic "empty" config or to provide a custom config file that will be used as a startup config instead.
#### Default node configuration
When a node is defined without `config` statement present, containerlab will generate an empty config from [this template](https://github.com/srl-labs/containerlab/blob/master/nodes/srl/srl.cfg) and put it in that directory.

```yaml
# example of a topo file that does not define a custom config
# as a result, the config will be generated from a template
# and used by this node
name: srl_lab
topology:
  nodes:
    srl1:
      kind: srl
      type: ixr6
```

The generated config will be saved by the path `clab-<lab_name>/<node-name>/config/config.json`. Using the example topology presented above, the exact path to the config will be `clab-srl_lab/srl1/config/config.json`.

#### User defined startup config
It is possible to make SR Linux nodes to boot up with a user-defined config instead of a built-in one. With a [`startup-config`](../nodes.md#startup-config) property of the node/kind a user sets the path to the config file that will be mounted to a container:

```yaml
name: srl_lab
topology:
  nodes:
    srl1:
      kind: srl
      type: ixr6
      image: ghcr.io/nokia/srlinux
      config: myconfig.json
```

With such topology file containerlab is instructed to take a file `myconfig.json` from the current working directory, copy it to the lab directory for that specific node under the `config.json` name and mount that file to the container. This will result in this config to act as a startup config for the node.

#### Saving configuration
As was explained in the [Node configuration](#node-configuration) section, SR Linux containers can make their config persist because config files are provided to the containers from the host via bind mount. There are two options to make a running configuration to be saved in a file.

##### Rewriting startup configuration
When a user configures SR Linux node via CLI the changes are saved into the running configuration stored in memory. To save the running configuration as a startup configuration the user needs to execute the `tools system configuration save` CLI command. This will write the config to the `config.json` file that holds the startup config and is exposed to the host.

##### Generating config checkpoint
If the startup configuration must be left intact, use an alternative method of saving the configuration checkpoint: `tools system configuration generate-checkpoint`. This command will create a `checkpoint-x.json` file that you will be able to find in the same `config` directory.

Containerlab allows to perform a bulk configuration-save operation that can be executed with `containerlab save -t <path-to-topo-file>` command.

With this command, every node that supports the "save" operation will execute a command to save it's running configuration to a persistent location. For SR Linux nodes the `save` command will trigger the checkpoint generation:

```
❯ containerlab save -t srl02.clab.yml
INFO[0000] Getting topology information from ../srl02.clab.yml file...
INFO[0001] clab-srl02-srl1 output: /system:
    Generated checkpoint '/etc/opt/srlinux/checkpoint/checkpoint-0.json' with name 'checkpoint-2020-12-03T15:12:46.854Z' and comment ''

INFO[0004] clab-srl02-srl2 output: /system:
    Generated checkpoint '/etc/opt/srlinux/checkpoint/checkpoint-0.json' with name 'checkpoint-2020-12-03T15:12:49.892Z' and comment ''
```

### TLS
By default containerlab will generate TLS certificates and keys for each SR Linux node of a lab. The TLS related files that containerlab creates are located in the so-called CA directory which can be located by the `<lab-directory>/ca/` path. Here is a list of files that containerlab creates relative to the CA directory

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
#### Config directory
When a user starts a lab, containerlab creates a lab directory for storing [configuration artifacts](../conf-artifacts.md). For `srl` kind containerlab creates directories for each node of that kind.

```
~/clab/clab-srl02
❯ ls -lah srl1
drwxrwxrwx+ 6 1002 1002   87 Dec  1 22:11 config
-rw-r--r--  1 root root 2.8K Dec  1 22:11 license.key
-rw-r--r--  1 root root 4.4K Dec  1 22:11 srlinux.conf
-rw-r--r--  1 root root  233 Dec  1 22:11 topology.clab.yml
```

The `config` directory is mounted to container's `/etc/opt/srlinux/` in `rw` mode and will effectively contain configuration that SR Linux runs of as well as the files that SR Linux keeps in its `/etc/opt/srlinux/` directory:

```
❯ ls srl1/config
banner  cli  config.json  devices  tls  ztp
```

#### Topology file
The topology file that defines the emulated hardware type is driven by the value of the kinds `type` parameter. Depending on a specified `type` the appropriate content will be populated into the `topology.yml` file that will get mounted to `/tmp/topology.yml` directory inside the container in `ro` mode.