---
search:
  boost: 4
kind_code_name: frr
kind_display_name: FRRouting
---
# -{{ kind_display_name }}-

[-{{ kind_display_name }}-](https://frrouting.org) is an open source internet routing protocol suite for Linux. It is identified with the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). The `frrouting` kind name is accepted as an alias.

## FRR via the `linux` kind

Earlier containerlab labs ran FRR containers with the [`linux`](linux.md) kind, supplying `frr.conf` and `daemons` through explicit bind mounts. The native `frr` kind builds on that work and makes FRR a first-class citizen in containerlab: it manages configuration files, daemon selection, forwarding, SSH public keys, and saving the running configuration.

To adapt an existing lab, change `kind: linux` to `kind: frr`, replace the bind mount for `/etc/frr/frr.conf` with `startup-config`, and replace the `/etc/frr/daemons` bind mount with `extras.frr.daemons`. Remove any bind for `/etc/frr/vtysh.conf`, since the kind generates it. Use the containerlab image below if you need SSH access. Existing labs can continue using the `linux` kind.

## Getting -{{ kind_display_name }}- image

FRR publishes a containerlab image alongside its plain release image, tagged `containerlab-<version>` in the same [`quay.io/frrouting/frr`](https://quay.io/repository/frrouting/frr) repository. This image was introduced with **FRR 10.7.1** and is published for 10.7.1 and later releases. **Releases older than 10.7.1 will not receive containerlab-native images.**

```bash
docker pull quay.io/frrouting/frr:containerlab-10.7.1
```

The containerlab image uses the corresponding FRR release image as its base and adds:

- An OpenSSH server, started alongside `watchfrr`, with SSH host keys generated on first start so each container has its own keys.
- An `admin` user whose login shell is `vtysh`, so SSH opens the routing CLI. Its `frr` and `frrvty` group membership lets it configure the router and save configurations. The existing `root` user retains a Linux shell.
- An FRR-specific `/etc/motd` describing the two SSH entry points and how to view FRR logs.
- A startup script that removes the management network's IPv4 and IPv6 default routes to keep them out of the lab's routing protocols. The connected management routes remain available.

FRR itself and the other base image components are unchanged. The image's build files and README are maintained upstream in [`docker/containerlab`](https://github.com/FRRouting/frr/tree/master/docker/containerlab).

The plain `<version>` tags ship no SSH server. Use `docker exec` to access those containers, as shown in the `vtysh` and `bash` tabs below.

## Managing -{{ kind_display_name }}- nodes

/// tab | SSH
The public keys detected on your host are added to both the `admin` and the `root` user, so no password is needed either way.

`admin` is the default user for these nodes, and its login shell is `vtysh`, so a bare `ssh` reaches the routing CLI:

```bash
ssh <node-name>
```

`root` gets a shell:

```bash
ssh root@<node-name>
```

`admin` can also log in with a password, which is `admin` unless the node's [`credentials`](../nodes.md#credentials) set another. The image ships no password for it; containerlab sets this one when it deploys the node, so changing `credentials` changes the password the node accepts. `root` has no password and accepts keys only.

These SSH commands require the containerlab image described above. For plain release images, use the `vtysh` or `bash` tabs below to access them with `docker exec`.
///
/// tab | vtysh
FRR's integrated shell is available in the container:

```bash
docker exec -it <node-name> vtysh
```

///
/// tab | bash
```bash
docker exec -it <node-name> bash
```

///

## Interfaces naming

-{{ kind_display_name }}- nodes use the `eth` prefix for their data interfaces, so `eth1`, `eth2` and so on. `eth0` is reserved for the management interface.

## Node configuration

-{{ kind_display_name }}- nodes are configured through three files, which containerlab writes into the node's lab directory under `config/`. That directory is bind mounted over the container's `/etc/frr`:

| File | Contents |
| --- | --- |
| `frr.conf` | the node's running configuration |
| `daemons` | which routing daemons to start |
| `vtysh.conf` | `service integrated-vtysh-config`, so `frr.conf` is the only config file |

The official image ships none of `frr.conf` and `vtysh.conf`, and `vtysh` refuses to start without them, which is why all three are always written.

Saving a configuration also leaves a `frr.conf.sav` next to them, which is the previous configuration: FRR keeps one backup by renaming the old file before writing the new one.

Files saved with `write memory` or `copy running-config startup-config` belong to the user who deployed the lab and remain readable on the host with `0644` permissions.

### Startup configuration

Point `startup-config` at an FRR configuration file to have it used as the node's `frr.conf`:

```yaml
topology:
  nodes:
    router1:
      kind: -{{ kind_code_name }}-
      image: quay.io/frrouting/frr:containerlab-10.7.1
      startup-config: router1/frr.conf
```

Without a `startup-config` the node comes up with a minimal configuration that only sets `frr defaults traditional` and logging.

The hostname is not set in the generated config on purpose. FRR picks up the container's hostname, which containerlab already sets to the node name.

### Daemons

By default all daemons supported by this kind are enabled. To run only the daemons a lab actually needs, list them under `extras`:

```yaml
topology:
  nodes:
    router1:
      kind: -{{ kind_code_name }}-
      image: quay.io/frrouting/frr:containerlab-10.7.1
      extras:
        frr:
          daemons:
            - ospfd
            - bfdd
```

Naming any daemon switches off all the ones you did not name. `zebra`, `staticd`, `mgmtd` and `watchfrr` are always started by FRR and may be listed or left out; either way they run.

The list accepts `bgpd`, `ospfd`, `ospf6d`, `ripd`, `ripngd`, `isisd`, `pimd`, `pim6d`, `ldpd`, `nhrpd`, `eigrpd`, `babeld`, `sharpd`, `pbrd`, `bfdd`, `fabricd`, `vrrpd` and `pathd`. Any other name is an error naming the offending entry.

/// admonition | Daemons and topology size
    type: subtle-note
Starting all daemons increases the number of processes and memory usage per node. For larger topologies, list only the daemons the lab needs.
///

`extras` can be set on a group or a kind as well as on a single node, so a whole class of routers can share one daemon list. See the [frr01 lab](../../lab-examples/frr01.md) for that.

### Saving configuration

`containerlab save` writes each node's running configuration back to `config/frr.conf` in its lab directory:

```bash
containerlab save -t <topology-file>
```

The saved file is reused on subsequent deployments while the lab directory exists. To replace it with `startup-config`, set [`enforce-startup-config`](../nodes.md#enforce-startup-config). Destroying the lab with `--cleanup` removes the saved configuration along with the lab directory; copy configurations you want to keep elsewhere first.

## Host requirements

-{{ kind_display_name }}- nodes enable IPv4 and IPv6 forwarding in the container's network namespace, so no host-level sysctl changes are needed.

## Lab examples

- [FRR OSPF lab](../../lab-examples/frr01.md)
