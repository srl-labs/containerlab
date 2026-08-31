---
search:
  boost: 4
kind_code_name: frr
kind_display_name: FRRouting
---
# -{{ kind_display_name }}-

[-{{ kind_display_name }}-](https://frrouting.org) is an open source internet routing protocol suite for Linux. It is identified with the `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md). The `frrouting` kind name is accepted as an alias.

## Getting -{{ kind_display_name }}- image

FRR publishes a containerlab flavour of its release image, tagged `containerlab-<version>` in the same [`quay.io/frrouting/frr`](https://quay.io/repository/frrouting/frr) repository:

```bash
docker pull quay.io/frrouting/frr:containerlab-10.7.1
```

It is the release image plus `openssh` and a start script that runs `sshd` alongside `watchfrr`. FRR itself is unchanged, and the image is built from [`docker/containerlab`](https://github.com/FRRouting/frr/tree/master/docker/containerlab) in the FRR repository on every release.

The plain `<version>` tags ship no SSH server, so their containers cannot be reached with `ssh`. They work with this kind otherwise, and everything else on this page behaves the same way with them.

## Managing -{{ kind_display_name }}- nodes

/// tab | SSH
The public keys detected on your host are added to the `root` user, so no password is needed:

```bash
ssh root@<node-name>
```

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

-{{ kind_display_name }}- nodes are configured through three files, which containerlab writes into the node's lab directory under `config/` and bind mounts over the container's `/etc/frr`:

| File | Contents |
| --- | --- |
| `frr.conf` | the node's running configuration |
| `daemons` | which routing daemons to start |
| `vtysh.conf` | `service integrated-vtysh-config`, so `frr.conf` is the only config file |

The official image ships none of `frr.conf` and `vtysh.conf`, and `vtysh` refuses to start without them, which is why all three are always written.

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

By default every -{{ kind_display_name }}- daemon is started, so any configuration you write works without further thought. To run only the daemons a lab actually needs, list them under `extras`:

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
Starting all daemons means roughly twenty processes per node. That is not worth thinking about for a handful of routers, but on a large fabric it adds up — name the daemons you need.
///

`extras` can be set on a group or a kind as well as on a single node, so a whole class of routers can share one daemon list. See the [frr01 lab](../../lab-examples/frr01.md) for that.

### Saving configuration

`containerlab save` writes each node's running configuration back to `config/frr.conf` in its lab directory:

```bash
containerlab save -t <topology-file>
```

## Host requirements

-{{ kind_display_name }}- nodes enable IPv4 and IPv6 forwarding in the container's network namespace, so no host-level sysctl changes are needed.

## Lab examples

- [FRR OSPF lab](../../lab-examples/frr01.md)
