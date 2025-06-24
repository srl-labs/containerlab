---
search:
  boost: 4
---

# VyOS Networks VyOS

Containerized VyOS network operating system is identified with the `vyosnetworks_vyos` kind in the [topology file](../topo-def-file.md).

VyOS nodes will launch with the following features

* Their management interface `eth0` configured with IPv4/6 addresses as assigned by the container runtime
* Hostname assigned to the node name
* The [VyOS HTTP API](https://docs.vyos.io/en/latest/automation/vyos-api.html) enabled
* Default credentials of`admin:admin`
* Available SSH keys installed into the `admin` user authorized keys

/// warning
VyOS node in Containerlab has only been tested with v1.5 Q1 Stream or higher
///

## Getting VyOS image
<!-- --8<-- [start:vyos-get-image] -->
VyOS does not provide a native container; you must create a container from the ISO image that can be [obtained](https://vyos.net/get/) in three ways:

1. LTS release which requires paying for a [subscription](https://vyos.io/subscriptions/software)
2. A release called [Stream](https://vyos.net/get/stream/), this is the basis for what will become the next LTS release. Similar to CentOS Stream and RHEL
3. A nightly [rolling release](https://vyos.net/get/nightly-builds/) with the bleeding edge changes

Once you've obtained an ISO you have to extract the squashfs filesystem and convert it to a container. VyOS provides instructions on their [site](https://docs.vyos.io/en/latest/installation/virtual/docker.html#deploy-container-from-iso) on how to do this.

To simplify the process, you may use the following Dockerfile and script created for a Debian or its derivative (but can be adapted to another distribution).

### Extract the filesystem

```bash
# Install required tools
sudo apt-get update
sudo apt-get install -y squashfs-tools-ng libarchive-tools

bsdtar -xf vyos-1.5-stream-2025-Q1-generic-amd64.iso live/filesystem.squashfs
sqfs2tar live/filesystem.squashfs > rootfs.tar
```

### Docker container

```Docker
FROM scratch

ADD rootfs.tar /

RUN for service in\
    getty.target \
    auditd.service \
    ;do systemctl mask $service; done && \
    systemctl disable kea-dhcp-ddns-server.service 

HEALTHCHECK --start-period=10s CMD systemctl is-system-running

CMD ["/sbin/init"]
```

<!-- --8<-- [end:vyos-get-image] -->

## Managing VyOS nodes

VyOS nodes launched with containerlab can be managed via the following interfaces:

/// tab | CLI
to connect to the VyOS CLI

```bash
docker exec -it <container-name/id> su - admin
```

///
/// tab | SSH
SSH server is running on the management interface

```bash
ssh admin@<container-name>
```

///
/// tab | API
The [VyOS HTTP API](https://docs.vyos.io/en/latest/automation/vyos-api.html) is running on the https port of the management interface. It uses the TLS certificate provided by containerlab. It uses the password as the API key.

```bash
curl --cacert <clab-folder>/.tls/ca/ca.pem --request POST  https://<node-name>/retrieve --form data='{"op": "showConfig", "path": []}' --form key="admin"
```

///

## Default user credentials

User credentials: `admin:admin`

## Interfaces mapping

VyOS only allows interfaces in the format `ethN`. The `eth0` interface is reserved for the management interface

## Features and options

### Node configuration

VyOS nodes have a dedicated [`config`](../conf-artifacts.md#identifying-a-lab-directory) directory that is used to persist the configuration of the node. It is possible to launch nodes of `vyosnetworks_vyos` kind with a basic config or to provide a custom config file that will be used as a startup config instead.

#### Default node configuration

When a node is defined without `startup-config` statement present, containerlab will generate an empty config from [this template](https://github.com/srl-labs/containerlab/blob/main/nodes/vyos/vyos.config.boot) and copy it to the config directory of the node.

```yaml
# example of a topo file that does not define a custom config
# as a result, the config will be generated from a template
# and used by this node
name: vyos_lab
topology:
  nodes:
    vyos:
      kind: vyosnetworks_vyos
```

The generated config will be saved to the path `clab-<lab_name>/<node-name>/config/config.boot`. Using the example topology presented above, the exact path to the config will be `clab-vyos/vyos/config/config.boot`.

#### User defined config

You may specify a customer startup configuration using the `startup-config` property.

```yaml
name: vyos_lab
topology:
  nodes:
    ceos:
      kind: vyosnetworks_vyos
      startup-config: myconfig.conf
```

When a config file is passed via `startup-config` parameter it will be used during an initial lab deployment. However, a config file that might be in the lab directory of a node takes precedence over the startup-config[^1].

It is possible to change the default config which every VyOS node will start with by specifying it in the `topology.kinds.vyos.startup-config`

```yaml
name: vyos_lab

topology:
  kinds:
    vyosnetworks_vyos:
    startup-config: vyos-custom-startup.cfg
  nodes:
    # vyos1 will boot with vyos-custom-startup.cfg as set in the kind parameters
    vyos1:
      kind: vyosnetworks_vyos
      image: vyos:latest
    # vyos2 will boot with its own specific startup config, as it overrides the kind variables
    vyos2:
      kind: vyosnetworks_vyos
      image: vyos:latest
      startup-config: node-specific-startup.cfg
  links:
    - endpoints: ["vyos1:eth1", "vyos2:eth1"]
```

[^1]: if startup config needs to be enforced, either deploy a lab with `--reconfigure` flag, or use [`enforce-startup-config`](../nodes.md#enforce-startup-config) setting.
