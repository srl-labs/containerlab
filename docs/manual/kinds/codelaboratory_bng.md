---
search:
  boost: 4
kind_code_name: codelaboratory_bng
kind_display_name: Code Laboratory BNG
---
# Code Laboratory BNG

Code Laboratory's eBPF/XDP-based Broadband Network Gateway is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).

The BNG uses eBPF/XDP for kernel-level DHCP fast path processing, achieving sub-100μs DHCP response times for cached subscribers. Unlike VPP-based BNGs, it requires only `NET_ADMIN` and `BPF` Linux capabilities — no hugepages, DPDK, or dedicated NICs.

## Getting -{{ kind_display_name }}- image

The -{{ kind_display_name }}- container image is available from GitHub Container Registry:

```bash
docker pull ghcr.io/codelaboratoryltd/bng:latest
```

Source code and build instructions are available on [GitHub](https://github.com/codelaboratoryltd/bng).

## Managing -{{ kind_display_name }}- nodes

### Health check

The BNG exposes a health endpoint on the metrics port:

```bash
curl http://<node-name>:9090/health
```

### Prometheus metrics

Metrics are served at the configured metrics address (default `:9090`):

```bash
curl http://<node-name>:9090/metrics
```

Key metrics include DHCP fast/slow path latencies, cache hit rates, pool utilization, and active session counts.

## Interfaces naming

-{{ kind_display_name }}- nodes use the following interface naming convention:

| Interface | Purpose |
|-----------|---------|
| `eth0` | Management (containerlab default) |
| `eth1` | Access / subscriber-facing |
| `eth2+` | Core / upstream links |

The `eth0` interface can only be used when `network-mode` is set to `none`.

There are no other restrictions on the interface naming besides the generic Linux interface naming rules.

## Features and options

### Startup configuration

The [`startup-config`](../nodes.md#startup-config) property sets the path to a YAML config file that is mounted to `/etc/bng/config.yaml` inside the container.

The config file uses a flat `key: value` format where each key matches a CLI flag name (without `--`). CLI flags take precedence over config file values.

Example minimal config:

```yaml
interface: eth1
pool-network: 10.0.1.0/24
pool-gateway: 10.0.1.1
pool-dns: "8.8.8.8,8.8.4.4"
lease-time: 3600s
metrics-addr: ":9090"
log-level: info
```

Example with advanced features:

```yaml
interface: eth1
pool-network: 10.0.1.0/24
pool-gateway: 10.0.1.1
pool-dns: "8.8.8.8,8.8.4.4"
lease-time: 3600s
metrics-addr: ":9090"
log-level: info

# Anti-spoofing (disabled, strict, loose, log-only)
antispoof-mode: log-only

# Walled garden captive portal
walled-garden: "false"
walled-garden-portal: "10.255.255.1:8080"

# NAT44/CGNAT
nat-enabled: "true"
nat-inside-interface: eth1
nat-outside-interface: eth2

# PPPoE
pppoe-enabled: "false"

# HA failover (active or standby)
ha-role: ""
ha-peer: ""
```

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BNG_INTERFACE` | `eth1` | Subscriber-facing interface |
| `BNG_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |

### Linux capabilities

The -{{ kind_display_name }}- kind automatically adds the following capabilities:

| Capability | Purpose |
|------------|---------|
| `NET_ADMIN` | Network interface and eBPF map management |
| `BPF` | Loading eBPF/XDP programs into the kernel |

This is significantly lighter than VPP-based BNGs which additionally require `SYS_ADMIN`, `IPC_LOCK`, `SYS_NICE`, and `SYS_RAWIO` for DPDK hugepage management.

### QinQ (802.1ad) support

The BNG supports QinQ double VLAN tagging at the eBPF/XDP data plane level. Subscribers are identified by S-TAG (service VLAN, outer) and C-TAG (customer VLAN, inner) pairs, supporting up to 100,000 concurrent VLAN-based subscribers.

The [bng01 lab example](../../lab-examples/bng01.md) includes both untagged and QinQ subscriber configs for [BNG Blaster][blaster].

[blaster]: https://github.com/rtbrick/bngblaster

## Quickstart

The following topology creates a minimal BNG lab with a subscriber simulator and core router:

```yaml
name: bng-quickstart

topology:
  nodes:
    bng1:
      kind: codelaboratory_bng
      image: ghcr.io/codelaboratoryltd/bng:latest
      startup-config: bng1/config.yaml
      exec:
        - ip addr add 10.0.1.1/24 dev eth1
        - ip addr add 10.0.0.1/24 dev eth2
    subscribers:
      kind: linux
      image: veesixnetworks/bngblaster:0.9.30
      binds:
        - subscribers/config.json:/config/config.json
    corerouter1:
      kind: linux
      image: frrouting/frr:v8.4.1
      binds:
        - corerouter1/daemons:/etc/frr/daemons
        - corerouter1/frr.conf:/etc/frr/frr.conf

  links:
    - endpoints: ["subscribers:eth1", "bng1:eth1"]
    - endpoints: ["bng1:eth2", "corerouter1:eth1"]
```

See the [bng01 lab example](../../lab-examples/bng01.md) for a complete working topology with all config files.
