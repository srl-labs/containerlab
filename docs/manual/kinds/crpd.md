# Juniper cRPD

[Juniper cRPD](https://www.juniper.net/documentation/us/en/software/crpd/crpd-deployment/topics/concept/understanding-crpd.html) is identified with `crpd` kind in the [topology file](../topo-def-file.md). A kind defines a supported feature set and a startup procedure of a `crpd` node.

cRPD nodes launched with containerlab comes up pre-provisioned with SSH service enabled, `root` user created and NETCONF enabled.

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

## Features and options
### Node configuration
cRPD nodes have a dedicated [`config`](#config-directory) directory that is used to persist the configuration of the node. It is possible to launch nodes of `crpd` kind with a basic "empty" config or to provide a custom config file that will be used as a startup config instead.

#### Default node configuration
When a node is defined without `config` statement present, containerlab will generate an empty config from [this template](https://github.com/srl-wim/container-lab/blob/master/templates/crpd/juniper.conf) and copy it to the config directory of the node.

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
It is possible to make cRPD nodes to boot up with a user-defined config instead of a built-in one. With a [`config`](../nodes.md#config) property of the node/kind a user sets the path to the config file that will be mounted to a container:

```yaml
name: srl_lab
topology:
  nodes:
    srl1:
      kind: srl
      type: ixr6
      license: lic.key
      config: myconfig.conf
```

With such topology file containerlab is instructed to take a file `myconfig.conf` from the current working directory, copy it to the lab directory for that specific node under the `/config/juniper.conf` name and mount that dir to the container. This will result in this config to act as a startup config for the node.

#### Saving configuration
Saving configuration with `containerlab save` command is not yet supported for cRPD nodes. Saving configuration via CLI command is still possible, of course.

### License
cRPD containers require a license file to have some features to be activated. With a [`license`](../nodes.md#license) directive it's possible to provide a path to a license file that will be used (work in progress).

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