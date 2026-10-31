---
search:
  boost: 4
kind_code_name: mikrotik_chr
kind_display_name: MikroTik RouterOS CHR
---
# -{{ kind_display_name }}-

[MikroTik RouterOS](https://mikrotik.com/software) Cloud Hosted Router (CHR) from the **official [`mikrotik/chr`](https://hub.docker.com/r/mikrotik/chr) image** is identified with the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). The image is published by MikroTik for `amd64` and `arm64` and is used as is: no [vrnetlab](../vrnetlab.md) build is needed.

The image is a QEMU VM (Alpine, qemu and the CHR disk) in a container. Its own `/start.sh` is not made for orchestrated links ([#3116](https://github.com/srl-labs/containerlab/issues/3116)), so the kind writes a launcher script into the node's lab directory, mounts it read-only and runs it as the entrypoint instead. The launcher uses only the tools that ship in the image.

/// note
The [`mikrotik_ros`](vr-ros.md) kind runs CHR from an image you build with vrnetlab. Both kinds use the same interface naming and startup config format, so a lab moves from one to the other by changing `kind` and `image`.
///

## Getting -{{ kind_display_name }}- image

```bash
docker pull mikrotik/chr:7.24.5
```

Tags follow RouterOS versions, plus `stable`, `long-term`, `testing` and `development`. This kind has been tested with 7.24.5 (stable), 7.23.8 (long-term) and 7.25rc2 (testing).

## Managing -{{ kind_display_name }}- nodes

/// tab | CLI
```bash
ssh admin@<node-name>
```
///
/// tab | REST API
```bash
curl -u admin:admin http://<node-name>/rest/system/resource
```
///
/// tab | Winbox / API
Winbox (TCP 8291) and the RouterOS API (TCP 8728, 8729 for TLS) are reachable on the node's management address.
///
/// tab | Console
The serial console is on port 5000 inside the container:
```bash
docker exec -it <node-name> nc 127.0.0.1 5000
```
///
/// tab | bash
```bash
docker exec -it <node-name> sh
```
///

The node's management address is RouterOS's own `ether1` address ([transparent management](#management-connectivity)), so every service RouterOS runs answers on it directly, with nothing forwarded.

/// note
Default credentials are `admin:admin`. With custom [`credentials`](../nodes.md#credentials) the kind creates that user in the `full` group and disables the factory `admin` account, which has a blank password on a fresh CHR disk.
///

## Interface naming

You can use [interface names](../topo-def-file.md#interface-naming) in the topology file like they appear in RouterOS: `etherX`, where `X` is the port number.

* `ether2` - first data port
* `ether3` - second data port, and so on

/// admonition
    type: warning
Data ports start at `ether2`: `ether1` is the management interface. Using `ether1` in a topology results in an error.
///

The container interface `ethN` is RouterOS `ether(N+1)`, also when links leave gaps: a link on `eth12` alone still lands on `ether13`, with `ether2` to `ether12` present and unlinked. Interface MTU follows the link MTU, and LLDP frames are forwarded, so `/ip neighbor` shows the peers.

## Node configuration

-{{ kind_display_name }}- nodes boot with the default CHR configuration plus the credentials, the identity (set to the node name) and the management setup described below.

### Startup configuration

A RouterOS script (`.rsc`) set with [`startup-config`](../nodes.md#startup-config) is imported once the router is up:

```yaml
name: chr_lab
topology:
  nodes:
    r1:
      kind: mikrotik_chr
      image: mikrotik/chr:7.24.5
      startup-config: r1.rsc
```

The file is copied to the node's lab directory as `ftpboot/config.auto.rsc` and imported over the REST API. RouterOS imports line by line and stops at the first error: in that case the container log carries RouterOS's own reason with the line number, for example

```
chr-launch: startup config import FAILED: Script Error: bad parameter network-type (line 3 column 59); lines before the failing one were applied, the rest were not
```

and the node does not become ready.

### Saving configuration

[`containerlab save`](../../cmd/save.md) writes each node's `/export` to `ftpboot/config.auto.rsc` in its lab directory. The save filter omits `add` entries tagged `clab-mgmt`, plus IPv4/IPv6 address, DHCP-client and IPv6 neighbor-discovery `add` entries for `ether1`, even without that tag. These management settings belong to the deployment and are recreated by the launcher; custom entries in those `ether1` sections are also omitted. Other interfaces and untagged configuration in other sections are preserved.

Saving explicitly retains an intentional `discover-interface-list=static` choice, which RouterOS normally omits as its default, so it survives redeployment. The launcher-owned discovery lists are recreated before the saved configuration is imported.

### Node readiness

The node is ready once RouterOS accepted the credentials and the startup config, reaches its gateway from the management address and, unless the configuration disables SSH, its SSH server answers. An SSH server that does not answer within `CHR_BOOT_WAIT` is logged, and the node is still declared ready, since REST and the console work. The container log then shows

```
chr-launch: RouterOS ready
```

and the kind's default [healthcheck](../nodes.md#healthcheck) reports the container `healthy`, so [`stages`](../nodes.md#stages) with `wait-for` can depend on it. A node whose startup config was rejected never becomes healthy. The default healthcheck's start period follows the launcher's own deadlines (`CHR_INTF_WAIT` + 3 × `CHR_BOOT_WAIT` + 90 s, 1050 s with the defaults), so a slow boot never reports `unhealthy`. A `healthcheck` set in the topology replaces the default one.

### Resources

The kind reads the official image's own environment variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `MEM` | 256 MiB + 8 MiB per data interface and vCPU | VM memory (a QEMU `-m` value, e.g. `1024` or `1G`) |
| `CPUS` | `1` | VM vCPUs |
| `DISK` | `128M` | minimum disk size (the disk only grows) |
| `CHR_INTF_WAIT` | `60` | seconds to wait for the data interfaces containerlab attaches before the VM starts |
| `CHR_BOOT_WAIT` | `300` | seconds to wait for RouterOS to answer, for the startup config import, and for SSH before the node is declared ready |

```yaml
r1:
  kind: mikrotik_chr
  image: mikrotik/chr:7.24.5
  env:
    MEM: 1024
    CPUS: 2
```

KVM is used when `/dev/kvm` is available. Without it the image's QEMU runs under TCG emulation: the node works, but boots several times slower.

## Management connectivity

Management is transparent: the IPv4 and IPv6 addresses and gateways containerlab gives the node's `eth0` are configured on RouterOS `ether1`, and the container's `eth0` is bridged to the VM's `ether1`. The router is the node's management address: SSH, the REST API, Winbox, SNMP and every other service answer on it, and the address in the topology table is the one in the RouterOS configuration.

The image ships busybox only (no `tc`), so where vrnetlab's [transparent management](../vrnetlab.md#management-interface) redirects with tc-mirred, this kind uses a Linux bridge. The container itself has no address on the management network; it reaches RouterOS for bootstrap over IPv6 link-local on that bridge.

The management return path lives in its own routing table, `mgmt`, selected by routing rules for the management addresses, and CHR's `ether1` DHCP client is disabled. A default route in the lab configuration, a PPPoE client's default route or a dual-WAN setup in `main` therefore does not cut SSH or REST access to the node. RouterOS's router advertisements on `ether1` carry a zero router lifetime, so no host on the management network takes the router as its IPv6 default gateway; the container joins the management network only once this is in place. The objects carry the comment `clab-mgmt` and are re-applied at boot when the container's addressing changed.

/// warning
Do not remove or change the `clab-mgmt` objects or the `mgmt` routing table: management of the node depends on them.
///

### Neighbor discovery

CHR routers sharing the management bridge can appear in each other's neighbor tables. LLDP, CDP and MNDP are disabled on management `ether1` by default; discovery stays enabled on the other static interfaces.

On first boot, before joining the management network, the launcher creates:

```routeros
/interface list
add name=clab-management comment=clab-mgmt
add name=clab-discovery include=static exclude=clab-management comment=clab-mgmt
/interface list member
add list=clab-management interface=ether1 comment=clab-mgmt
/ip neighbor discovery-settings
set discover-interface-list=clab-discovery
```

These reserved lists belong to the launcher. To change discovery, select another list in your startup configuration or on the running router. For example, to enable discovery on all static interfaces, including management:

```routeros
/ip neighbor discovery-settings set discover-interface-list=static
```

Use `discover-interface-list=none` to disable discovery everywhere, or select your own interface list. Startup configuration is imported after this default is applied. Restarts preserve the selected policy, including on nodes created before this default was introduced. SSH, REST and other management services remain available. This setting does not control IPv6 address resolution or router advertisements under `/ipv6 nd`.

As with RouterOS's normal discovery rules, including a bridge or bonding master enables discovery on its slave ports. If your configuration puts `ether1` into a bridge or bond, select a discovery list that excludes the master as well.

## Restarts

[`containerlab restart`](../../cmd/restart.md) and a `/system reboot` inside RouterOS keep the interface names, MAC addresses, configuration and links: the launcher persists the NIC identity in the container on first boot and does not re-run the bootstrap. A plain `docker restart` keeps the router and its management, but Docker deletes the node's veth links, which stay down until the node is restarted with containerlab or the lab is redeployed.

## File mounts

For each node containerlab creates in its [lab directory](../conf-artifacts.md):

* `ftpboot/` - mounted to `/ftpboot`, holds `config.auto.rsc`
* `chr-launch.sh` - the launcher, mounted read-only to `/chr-launch.sh` and rewritten on every deploy
