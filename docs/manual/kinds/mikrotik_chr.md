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

=== "CLI"
    ```bash
    ssh admin@<node-name>
    ```
=== "REST API"
    ```bash
    curl -u admin:admin http://<node-name>/rest/system/resource
    ```
=== "Winbox / API"
    Winbox (TCP 8291) and the RouterOS API (TCP 8728, 8729 for TLS) are reachable on the node's management address.
=== "Console"
    The serial console is exposed on TCP port 5000:
    ```bash
    telnet <node-name> 5000
    ```
=== "bash"
    ```bash
    docker exec -it <node-name> sh
    ```

SSH (22), Telnet (23), FTP (21), HTTP/HTTPS (80/443), Winbox (8291), API (8728/8729) and SNMP (UDP 161) on the node's management address are forwarded to RouterOS.

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

[`containerlab save`](../../cmd/save.md) writes each node's `/export` to `ftpboot/config.auto.rsc` in its lab directory. The objects the launcher creates at every boot (the `ether1` DHCP client and the `mgmt` routing table, route and rule) are left out, so the saved file can be used as the startup config of the next deployment.

### Node readiness

The node is ready once RouterOS accepted the credentials and the startup config and, unless the configuration disables SSH, its SSH server answers. The container log then shows

```
chr-launch: RouterOS ready
```

and the kind's default [healthcheck](../nodes.md#healthcheck) reports the container `healthy`, so [`stages`](../nodes.md#stages) with `wait-for` can depend on it. A node whose startup config was rejected never becomes healthy. A `healthcheck` set in the topology replaces the default one.

### Resources

The kind reads the official image's own environment variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `MEM` | 256 MiB + 8 MiB per data interface | VM memory (a QEMU `-m` value, e.g. `1024` or `1G`) |
| `CPUS` | `1` | VM vCPUs |
| `DISK` | `128M` | minimum disk size (the disk only grows) |
| `CHR_BOOT_WAIT` | `300` | seconds to wait for RouterOS to answer, and for the startup config import |

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

The VM's `ether1` is a QEMU user-mode (slirp) interface: CHR's default DHCP client gets `198.18.255.15/24` (from the RFC 2544 benchmarking range, so it does not collide with lab addressing), and the service ports listed above are forwarded from the container's management address.

The management return path lives in its own routing table, `mgmt`, selected by a routing rule for `198.18.255.15`, and the DHCP client adds no default route to `main`. A default route in the lab configuration, a PPPoE client's default route or a dual-WAN setup in `main` therefore does not cut SSH or REST access to the node.

/// warning
Do not configure `198.18.255.0/24` on data interfaces, or change the `ether1` DHCP client or the `mgmt` table: management of the node depends on them.
///

## Restarts

[`containerlab restart`](../../cmd/restart.md) and a `/system reboot` inside RouterOS keep the interface names, MAC addresses, configuration and links: the launcher persists the NIC identity in the container on first boot and does not re-run the bootstrap. A plain `docker restart` keeps the router and its management, but Docker deletes the node's veth links, which stay down until the node is restarted with containerlab or the lab is redeployed.

## File mounts

For each node containerlab creates in its [lab directory](../conf-artifacts.md):

* `ftpboot/` - mounted to `/ftpboot`, holds `config.auto.rsc`
* `chr-launch.sh` - the launcher, mounted read-only to `/chr-launch.sh` and rewritten on every deploy
