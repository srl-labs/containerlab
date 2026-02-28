---
search:
  boost: 4
kind_code_name: veesix_osvbng
kind_display_name: v::n osvbng
---
# -{{ kind_display_name }}-

[-{{ kind_display_name }}-](https://github.com/veesix-networks/osvbng) node is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).

The integration of -{{ kind_display_name }}- has been tested with v0.3.1 release. Note, that releases before v0.3.1 are not supported and will not work with containerlab.

## Getting -{{ kind_display_name }}- image

The -{{ kind_display_name }}- container image is available publicly on Docker Hub as [`veesixnetworks/osvbng`](https://hub.docker.com/r/veesixnetworks/osvbng).

```bash
docker pull veesixnetworks/osvbng:<tag>
```

To pull the latest available image:

```bash
docker pull veesixnetworks/osvbng:latest
```

## Managing -{{ kind_display_name }}- nodes

/// tab | API
The [osvbng Northbound API](https://docs.osvbng.v6n.io/getting-started/api/) is running on port 8080 by default. An OpenAPI Swagger UI is available at `http://<node-name>:8080/api/docs/`.

```bash
curl http://<node-name>:8080/api/show/protocols/ospf/neighbors
```

///

/// tab | Linux shell
SSH server is running on the management interface

```bash
ssh admin@<container-name>
```

///

/// tab | CLI
The `osvbngcli` utility provides a basic interactive CLI for managing and monitoring the osvbng node:

```bash
docker exec -it <container-name> osvbngcli
```

```
osvbng> show subscriber sessions
```

///

## Interfaces naming

-{{ kind_display_name }}- container uses the following mapping for its interfaces:

* `eth0` - management interface connected to the containerlab management network
* `eth1+` - data interfaces

## Features and options

### Startup configuration

-{{ kind_display_name }}- uses a YAML configuration file to define interfaces, subscriber groups, DHCP settings, AAA policies, and routing protocols.

With the [`startup-config`](../nodes.md#startup-config) property of the node/kind, a user sets the path to the local config file that will be mounted to the container at `/etc/osvbng/osvbng.yaml`.

```yaml
topology:
  nodes:
    bng1:
      kind: veesix_osvbng
      image: veesixnetworks/osvbng:v0.3.1
      startup-config: bng1/osvbng.yaml
```

### Environment variables

| Variable                     | Description                                                                 | Default |
| ---------------------------- | --------------------------------------------------------------------------- | ------- |
| `OSVBNG_WAIT_FOR_INTERFACES` | Wait for interfaces to be provisioned before starting (set automatically by containerlab) | `true`  |

## Quickstart

The `osvbng01` lab example demonstrates a minimal BNG topology with an instance of [BNG Blaster](https://github.com/rtbrick/bngblaster) as a subscriber simulator, an osvbng node, and a core router running FRR, giving you the ability to simulate a real-world BNG with QinQ IPoE termination.

The topology consists of three nodes:

- **subscribers** - a [BNG Blaster](https://github.com/rtbrick/bngblaster) container simulating subscribers with Q-in-Q tagged IPoE sessions over DHCPv4/DHCPv6
- **bng1** - the osvbng node performing subscriber termination
- **corerouter1** - an FRR router acting as the core/upstream router

```yaml
name: osvbng01

topology:
  nodes:
    bng1:
      kind: veesix_osvbng
      image: veesixnetworks/osvbng:v0.3.1
      startup-config: bng1/osvbng.yaml
    corerouter1:
      kind: linux
      image: frrouting/frr:v8.4.1
      binds:
        - corerouter1/daemons:/etc/frr/daemons
        - corerouter1/frr.conf:/etc/frr/frr.conf
    subscribers:
      kind: linux
      image: veesixnetworks/bngblaster:0.9.30
      binds:
        - subscribers/config.json:/config/config.json

  links:
    - endpoints: ["subscribers:eth1", "bng1:eth1"]
    - endpoints: ["bng1:eth2", "corerouter1:eth1"]
```
