---
search:
  boost: 4
kind_code_name: veesix_osvbng
kind_display_name: v::n osvbng
---
# -{{ kind_display_name }}-

[-{{ kind_display_name }}-](https://github.com/veesix-networks/osvbng) node is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).

The integration of -{{ kind_display_name }}- has been tested with v0.1.2 release. Note, that releases <= v0.1.2 are not supported and will not work with containerlab.

## Getting -{{ kind_display_name }}- image

The -{{ kind_display_name }}- container image is available publicly on Docker Hub as [`veesixnetworks/osvbng`](https://hub.docker.com/r/veesixnetworks/osvbng).

```bash
docker pull veesixnetworks/osvbng:<tag>
```

## Managing -{{ kind_display_name }}- nodes

/// tab | CLI
The `osvbngcli` utility provides a basic interactive CLI for managing and monitoring the osvbng node:

```bash
docker exec -it <container-name> osvbngcli
```

```
osvbng> show subscriber sessions
```

///

/// tab | Linux shell
SSH server is running on the management interface

```bash
ssh admin@<container-name>
```

///

/// tab | API
The [osvbng Northbound API](https://docs.osvbng.v6n.io/getting-started/api/) is running on port 8080 by default.

```bash
curl http://<node-name>/api/show/protocols/isis/neighbors
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
      image: veesixnetworks/osvbng:v0.1.2
      startup-config: bng1/osvbng.yaml
```

### Environment variables

| Variable                     | Description                                                                 | Default |
| ---------------------------- | --------------------------------------------------------------------------- | ------- |
| `OSVBNG_WAIT_FOR_INTERFACES` | Wait for interfaces to be provisioned before starting (set automatically by containerlab) | `true`  |

## Quickstart

The `osvbng01` lab example demonstrates a minimal BNG topology with a subscriber, an osvbng node, and a core router running FRR, simulating a real-world BNG with QinQ IPoE termination, IS-IS, IPv6 and BGP pre-configured.

The topology consists of three nodes:

- **subscriber** - an Alpine Linux container simulating a subscriber with Q-in-Q tagged traffic
- **bng1** - the osvbng node performing subscriber termination
- **corerouter1** - an FRR router acting as the core/upstream router

```yaml
name: osvbng01

topology:
  nodes:
    bng1:
      kind: veesix_osvbng
      image: veesixnetworks/osvbng:v0.1.2
      startup-config: bng1/osvbng.yaml
    corerouter1:
      kind: linux
      image: frrouting/frr:v8.4.1
      binds:
        - corerouter1/daemons:/etc/frr/daemons
        - corerouter1/frr.conf:/etc/frr/frr.conf
    subscriber:
      kind: linux
      image: alpine:latest
      exec:
        - ip link add link eth1 name eth1.100 type vlan id 100 protocol 802.1ad
        - ip link add link eth1.100 name eth1.100.10 type vlan id 10
        - ip link set eth1.100 up
        - ip link set eth1.100.10 up
        - udhcpc -i eth1.100.10 -q

  links:
    - endpoints: ["subscriber:eth1", "bng1:eth1"]
    - endpoints: ["bng1:eth2", "corerouter1:eth1"]
```