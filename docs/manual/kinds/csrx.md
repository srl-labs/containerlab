---
search:
  boost: 4
kind_code_name: juniper_csrx
kind_display_name: Juniper cSRX
---
# -{{ kind_display_name }}-
[-{{ kind_display_name }}-](https://www.juniper.net/documentation/us/en/software/csrx/csrx-getting-started/index.html) is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).
A kind defines a supported feature set and a startup procedure of a `csrx` node.

cSRX nodes launched with containerlab come up with SSH enabled, `root` login allowed, NETCONF enabled, and the `root` password pre-provisioned.

cSRX is distributed as a Docker image that must be loaded manually from a Juniper-provided tarball:

```bash
# load the cSRX container image
sudo docker load -i junos-csrx-docker-24.4R1.9.tgz
```

## Managing cSRX nodes

Juniper cSRX nodes launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running cSRX container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI"
    to connect to the cSRX CLI
    ```bash
    docker exec -it <container-name/id> cli
    ```
=== "SSH"
    direct SSH to the management interface:
    ```bash
    ssh root@<container-name>
    ```
=== "NETCONF"
    NETCONF server is running over port 830
    ```bash
    ssh root@<container-name> -p 830 -s netconf
    ```

!!!info
    Default user credentials: `root:clab123`

## Interfaces mapping

cSRX container uses the following mapping between Linux interfaces and Junos interfaces:

* `eth0` - management interface, corresponds to `fxp0` from the CLI perspective and receives the container management address from the `docker0`/management network.
* `eth1` - first data interface, mapped to Junos `ge-0/0/0`.
* `eth2` - mapped to Junos `ge-0/0/1`.
* `eth3` - mapped to Junos `ge-0/0/2`.

When defining topology links, use the Linux names (`ethN`) on the `endpoints`. The corresponding `ge-0/0/X` interfaces can then be configured from Junos as usual:

```yaml
topology:
  nodes:
    csrx1: { kind: -{{ kind_code_name }}-, image: csrx:24.4R1.9 }
    csrx2: { kind: -{{ kind_code_name }}-, image: csrx:24.4R1.9 }
  links:
    - endpoints: ["csrx1:eth1", "csrx2:eth1"]
```

```
root@csrx1> show interfaces ge-0/0/0
Physical interface: ge-0/0/0, Enabled, Physical link is Up
  ...
  Logical interface ge-0/0/0.0 (Index 100)
    Protocol inet
        Destination: 10.0.0.0/30, Local: 10.0.0.1
```

!!!note
    cSRX is a firewall: by default, traffic between zones requires an explicit security policy, and traffic destined to the device (ping, SSH, NETCONF on data interfaces) requires `host-inbound-traffic` to be enabled on the zone.

## Features and options

### Node configuration

cSRX nodes have a dedicated [`config`](../conf-artifacts.md#identifying-a-lab-directory) directory used to persist the Junos configuration. `-{{ kind_code_name }}-` nodes can boot with a built-in default config or with a user-provided one.

#### Default node configuration

When a node is defined without the `startup-config` statement, containerlab generates a minimal Junos config from [this template](https://github.com/srl-labs/containerlab/blob/main/nodes/csrx/csrx.cfg) and copies it to the node's config directory.

```yaml
# example of a topo file that does not define a custom config
# as a result, the config will be generated from a template
# and used by this node
name: csrx
topology:
  nodes:
    csrx:
      kind: -{{ kind_code_name }}-
      image: csrx:24.4R1.9
```

The generated config will be saved at `clab-<lab_name>/<node-name>/config/juniper.conf`. Using the example topology above, the exact path is `clab-csrx/csrx/config/juniper.conf`.

#### User-defined config

It is possible to boot cSRX with a user-defined config instead of the built-in one via the [`startup-config`](../nodes.md#startup-config) property:

```yaml
name: csrx_lab
topology:
  nodes:
    csrx:
      kind: -{{ kind_code_name }}-
      image: csrx:24.4R1.9
      startup-config: myconfig.conf
```

containerlab copies `myconfig.conf` into the node's lab directory under `config/juniper.conf` and mounts it into the container at `/config/juniper.conf`, so it is applied on boot.

!!!warning
    cSRX is a firewall and does not support Junos routing stanzas such as `policy-options` or `routing-options { protocols ... }`. If the startup config contains unsupported stanzas, `mgd` rejects the whole file and falls back to the factory configuration. Inspect `/var/log/csrx_start.log` inside the container to see syntax errors.

#### Saving configuration

With the [`containerlab save`](../../cmd/save.md) command it is possible to save the running cSRX configuration. The output of `cli show configuration` is written to `config/juniper.conf` in the node directory, overwriting the previous content.

### License

cSRX requires a license for full functionality. With the [`license`](../nodes.md#license) directive you can provide a path to a license file; containerlab copies it into the node directory at `config/license/license.lic` and mounts it into the container. On boot, containerlab runs `cli request system license add /config/license/license.lic` in the `PostDeploy` phase.

## Container configuration

### File mounts

When a user starts a lab, containerlab creates a node directory for storing [configuration artifacts](../conf-artifacts.md). For `-{{ kind_code_name }}-` the following files and directories are created in the node directory and mounted into the container:

| Host path (under `clab-<lab>/<node>/`) | Container path | Purpose |
| --- | --- | --- |
| `config/` | `/config` | Junos config and license |
| `log/` | `/var/log` | Junos/Linux log files |
| `config/sshd_config` | `/etc/ssh/sshd_config` | SSH daemon configuration (for Junos <23 images) |
| `csrx_password_config_file` | `/var/local/csrx_password_config_file` | Sentinel file; its presence makes the cSRX entrypoint skip the default `root-authentication *disabled*` step, so the `encrypted-password` from `juniper.conf` is honored |

The `CSRX_JUNOS_CONFIG` environment variable is set to `/config/juniper.conf` on 22.x images so the entrypoint loads our config (24.x images load it unconditionally).

```
❯ tree clab-csrx/csrx
clab-csrx/csrx
├── config
│   ├── juniper.conf
│   ├── license
│   │   └── license.lic
│   └── sshd_config
├── csrx_password_config_file
└── log
```
