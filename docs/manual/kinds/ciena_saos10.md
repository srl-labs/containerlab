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

## Managing ciena_saos10 nodes

Ciena SAOS 10 nodes launched with containerlab can be managed via the following interfaces:

=== "CLI via SSH"
    to connect to the SAOS CLI (password `ciena123`)
    ```bash
    ssh diag@<container-name/id>
    ```
=== "NETCONF"
    NETCONF is exposed over SSH on port 830:
    ```bash
    ssh diag@<container-name/id> -p 830 -s netconf
    ```
=== "Console via telnet"
    to connect to the serial console
    ```bash
    telnet <container-name/id> 5000
    ```

!!!info
    Default user credentials: `diag:ciena123`

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

### Startup configuration

SAOS 10 does **not** support full startup-config replacement. The startup config
is treated as a **partial overlay** that is applied once, after the node finishes
booting and its management plane becomes reachable.

For the `ciena_saos10` kind, set the `startup-config` property to a partial config
file whose name contains `.partial`. containerlab mounts the file into the node's
`/config` directory and points the launcher at it via the
`SAOS_STARTUP_CONFIG_PATH` environment variable.

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
The apply result is recorded in the container's `/state.json`.

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

!!!warning "Not supported with management passthrough"
    Startup partial config apply is **not supported when management passthrough
    is enabled**. In that mode the launcher does not apply the partial config —
    it records `startup_partial_apply_status=skipped` in `/state.json` and
    continues booting. Any configuration must be applied out-of-band against the
    node's management IP.

!!!tip "Apply timeout tuning"
    Older SAOS 10.11 images may need longer apply timeouts. These can be set per
    node via the `env` map: `SAOS_STARTUP_PARTIAL_CLI_CMD_TIMEOUT_S` (default
    `240`), `SAOS_CONFIG_ENTER_CMD_TIMEOUT_S` (default `90`), and
    `SAOS_BASE_CONFIG_CMD_TIMEOUT_S` (default `180`).
