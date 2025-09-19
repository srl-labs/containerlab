---
search:
  boost: 4
---
# Arrcus ArcOS

[Arrcus ArcOS](https://arrcus.com/connected-edge/arcos) is identified with `arrcus_arcos` kind in the [topology file](../topo-def-file.md). ArcOS nodes launched with containerlab comes up with

* their management interface `eth0` configured with IPv4/6 addresses as assigned by docker
* hostname assigned to the node name
* SSH services enabled
* `root` user created with password `clab@123`
* `clab` user created with password `clab@123`

## Getting ArcOS

ArcOS image is available for download only for users who have an active service account.

The obtained image archive can be loaded to local docker image store with:

```bash
docker image load -i ArcOS_4.3.1B_DOCKER.xz
```

## Managing ArcOS nodes

There are several management interfaces supported by ArcOS nodes:

/// tab | CLI
to connect to a ArcOS CLI shell of a running ArcOS container:

```bash
docker exec -it <container-name/id> /usr/bin/cli
```

///

/// tab | bash
to connect to a `bash` shell of a running ArcOS container:

```bash
docker exec -it <container-name/id> bash
```

///

/// tab | SSH
to connect to a ArcOS CLI, simply SSH to the node:

```
ssh clab@<container-name>
Password: clab@123
```

///

/// tab | Netconf
Netconf server runs on 830 port:

```bash
ssh clab@<container-name> -p 830 -s netconf
```

///

### Credentials

Default user credentials:

* Usernane: `clab`
* Password: `clab@123`

## Interfaces mapping

ArcOS container uses the following mapping for its linux interfaces:

* `eth0` - management interface connected to the containerlab management network
* `swpX` - data interface

## Features and options

### Node configuration

#### Default node configuration

It is possible to launch nodes of `arrcus_arcos` kind with a basic config or to provide a custom config file that will be used as a startup config instead.

When a node is defined without `startup-config` statement present, containerlab will generate config from [this template](https://github.com/srl-labs/containerlab/blob/main/nodes/arrcus_arcos/arcos.cfg) and copy it to the config directory of the node.

#### User defined config

With a [`startup-config`](../nodes.md#startup-config) property a user sets the path to the config file that will be mounted to a container and used as a startup-config:

```yaml
name: r1
topology:
  nodes:
    arcos:
      kind: arrcus_arcos
      startup-config: r1.cfg
```

When a config file is passed via `startup-config` parameter it will be used during an initial lab deployment. However, a config file that might be in the lab directory of a node takes precedence over the startup-config[^1].

With such topology file containerlab is instructed to take a file `r1.cfg` from the current working directory and copy it to the lab directory for that specific node under the `/startup.cfg` name. This will result in this config acting as a startup-config for the node.

To provide a user-defined config, take the [default configuration template](https://github.com/srl-labs/containerlab/blob/main/nodes/arrcus_arcos/arcos.cfg) and add the necessary configuration commands without changing the rest of the file. This will result in proper automatic assignment of IP addresses to the management interface, as well as applying user-defined commands.

## Known issues and limitations

### OS Version

* Currently, only v4.X is supported.

### management interface

* The interface name is currently set to `eth0`.
* Even if the IP address is changed in the configuration, it will not actually be reflected.
* The management VRF, which is intended for the management interface, is currently not usable.

### boot

* After deployment, the network connection is established immediately, but it takes about 50 seconds for the config to be loaded.

## Lab examples

The following labs feature ArcOS nodes:

* [SR Linux and ArcOS](../../lab-examples/srl-arcos.md)

[^1]: if startup config needs to be enforced, either deploy a lab with `--reconfigure` flag, or use [`enforce-startup-config`](../nodes.md#enforce-startup-config) setting.
