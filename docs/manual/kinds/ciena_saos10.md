---
search:
  boost: 4
kind_code_name: ciena_saos10
kind_display_name: Ciena SAOS 10
---
# Ciena SAOS 10

The Ciena SAOS 10.x virtualized switch is identified with the `ciena_saos10` kind in the
[topology file](../topo-def-file.md). It is distributed by Ciena as a
[vrnetlab](../vrnetlab.md)-packaged container image and runs as a Qemu VM inside
the container.

## Host requirements

Because SAOS 10 runs as a Qemu VM inside the container, the host must support
hardware virtualization (KVM). When containerlab itself runs inside a VM, nested
virtualization must be enabled on that VM.

By default the kind uses the `tc` dataplane connection mode. When
`connection-mode` is set to `macvtap`, the launcher additionally bind-mounts the
host's `/dev` into the container.

## Managing ciena_saos10 nodes

Ciena SAOS 10 nodes launched with containerlab can be managed via the following interfaces:

/// tab | CLI via SSH
to connect to the SAOS CLI (password `ciena123`)

```bash
ssh diag@<container-name/id>
```
///
/// tab | NETCONF
NETCONF is exposed over SSH on port 830:

```bash
ssh diag@<container-name/id> -p 830 -s netconf
```
///
/// tab | Console via telnet
to connect to the serial console

```bash
telnet <container-name/id> 5000
```
///

## Credentials

Default user credentials are `diag` / `ciena123`. They are used for both SSH
(port 22) and NETCONF (port 830) access. The defaults can be overridden per node
via the `credentials` property in the topology file.

## Variants

You must specify the SAOS variant using the `type` field in the topology:

```
3948, 3949, 3984, 3985, 5130, 5131, 5131-910, 5132, 5134, 5144, 5162, 5164,
5164-902, 5166, 5166-903, 5168, 5169, 5170, 5171, 5171-920, 5184, 5186, 8110,
8112, 8114, 8140, 8190, 8192
```

## Interface naming

SAOS ports are addressed by simple numeric names in the topology file:

* `1` - first data port available
* `2` - second data port, and so on...

The ports above are mapped to the following Linux interfaces inside the container:

* `eth1` - first data interface
* `eth2+` - second and subsequent data interfaces

## Features and options

### Image

The `ciena_saos10` image is **provided by Ciena**. Unlike most vrnetlab-based
kinds, it cannot currently be built locally from a qcow2 disk — obtain the
prebuilt image from your Ciena representative and load it onto the host
(for example with `docker load`) before deploying.

Reference the provided image tag explicitly in the topology:

```yaml
topology:
  nodes:
    saos-1:
      kind: ciena_saos10
      image: vrnetlab/ciena_saos10:10-12-00-0228
      type: 5132
```

### Node configuration

Out of the box a `ciena_saos10` node boots with the default `diag` user and its
management plane reachable over SSH (port 22) and NETCONF (port 830). No
data-plane configuration is present until a startup config is applied.

When a `startup-config` is provided, containerlab mounts it into the node's
`/config` directory and points the launcher at it via the
`SAOS_STARTUP_CONFIG_PATH` environment variable. Only partial overlays are
supported — see below.

### Startup configuration

SAOS 10 does **not** support full startup-config replacement. The startup config
is treated as a **partial overlay** that is applied once, after the node finishes
booting and its management plane becomes reachable.

For the `ciena_saos10` kind, set the `startup-config` property to a partial config
file whose name contains `.partial`.

The config format is auto-detected from the file **contents** (not the file
extension):

* a file whose first non-whitespace character is `<` is treated as a
  **NETCONF/XML** partial and applied over NETCONF;
* anything else is treated as a **CLI** partial — each non-empty, non-comment
  line is sent as a config command (lines that are just `config`/`configure` or
  start with `#`/`!` are ignored).

In all cases the partial is applied over the **management plane** (NETCONF or
SSH) once the device reaches SSH readiness — it is **never** applied over the
serial console. Explicit config/RPC errors (for example `Config Mode Error`,
`rpc-error`, `access-denied`) fail the apply instead of being treated as success.
The apply result is recorded in the container's `/state.json` — see
[Boot state and health](#boot-state-and-health).

```yaml
topology:
  nodes:
    saos-1:
      kind: ciena_saos10
      image: vrnetlab/ciena_saos10:10-12-00-0228
      type: 5132
      # CLI partials (e.g. .cfg.partial / .txt.partial) work the same way
      startup-config: configuration.xml.partial
```

/// tip | Apply timeout tuning
Older SAOS 10.11 images may need longer apply timeouts. These can be set per
node via the `env` map: `SAOS_STARTUP_PARTIAL_CLI_CMD_TIMEOUT_S` (default
`240`), `SAOS_CONFIG_ENTER_CMD_TIMEOUT_S` (default `90`), and
`SAOS_BASE_CONFIG_CMD_TIMEOUT_S` (default `180`).
///

## Boot state and health

The launcher records boot and config-apply progress in `/state.json` inside the
container. It is the primary way to see where a node is — or why it is stuck:

```bash
docker exec <container-name/id> cat /state.json
```

Useful fields:

* `current` - the node's current state
* `states` - a timestamped log of each state transition
* `timeouts_soft_s` / `timeouts_hard_s` - per-state time budgets (tunable via the
  `SAOS_*_TIMEOUT_S` env vars)
* `meta` - extra detail, including the partial-config apply result

A node advances through these states in order:

`waiting_for_login` → `password_revert` → `bootstrap_done` → `config_ready` →
`config_base_applied` → `ssh_ready` → `startup_partial_applied` → `healthy`

The container reports Docker health `healthy` only once it reaches the `healthy`
state. A cold boot typically takes several minutes, dominated by `config_ready`.

The partial-config apply outcome is recorded under `meta`:

| Outcome | How it appears in `/state.json` |
| --- | --- |
| Applied | node reaches the `startup_partial_applied` state (then `healthy`) |
| Skipped | `startup_partial_apply_status: "skipped"` with a `startup_partial_apply_warning` |
| Failed | `startup_partial_apply_failed: true` and `startup_partial_apply_error: "<reason>"` |

## Known issues and limitations

* Only **partial** startup-config overlays are supported. A full-config
  replacement (a `startup-config` file whose name does not contain `.partial`)
  is rejected at deploy time.
* Partial-config apply is **not supported when management passthrough is
  enabled**. In that mode the launcher does not apply the partial config — it
  records `startup_partial_apply_status=skipped` in `/state.json` (see
  [Boot state and health](#boot-state-and-health)) and continues booting. Any
  configuration must then be applied out-of-band against the node's management IP.
* Older SAOS 10.11 images may boot and apply config slowly; if the partial apply
  times out, tune the `SAOS_*_TIMEOUT_S` values described in the
  [Startup configuration](#startup-configuration) section.

## Lab examples

The following minimal lab connects two `ciena_saos10` nodes back to back — see
`lab-examples/ciena_saos10/ciena_saos10.clab.yml`:

```yaml
name: ciena_saos10

topology:
  nodes:
    saos1:
      kind: ciena_saos10
      image: vrnetlab/ciena_saos10:10-12-00-0228
      type: 5132
    saos2:
      kind: ciena_saos10
      image: vrnetlab/ciena_saos10:10-12-00-0228
      type: 5132

  links:
    - endpoints: ["saos1:1", "saos2:1"]
```
