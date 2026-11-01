# Alcatel-Lucent Enterprise OmniSwitch vAOS

[ALE OmniSwitch](https://www.al-enterprise.com/en/products/switches) virtual AOS (vAOS) 8.x is identified with the `ale_vaos` kind in the [topology file](../topo-def-file.md). It is built using the [vrnetlab](../vrnetlab.md) project and essentially is a QEMU VM packaged in a docker container format.

## Getting the image

To get the vAOS image, contact your ALE representative or ask for it on the [Spacewalkers community](https://www.spacewalkers.com/).

## Managing ale_vaos nodes

!!!note
    Containers with vAOS inside take about 3 minutes to boot and apply their configuration.
    You can monitor the progress with `docker logs -f <container-name>`. The node becomes healthy once the bootstrap is done.

=== "SSH"
    `ssh admin@<container-name>`
    Password: `Switch@123`
=== "Serial console"
    `telnet <container-name> 5000`

## Interface naming

| Container interface | vAOS port |
| ------------------- | --------- |
| `eth1` to `eth10`   | `1/1/1` to `1/1/10` |
| `eth11`             | `1/1/29A` |
| `eth12`             | `1/1/30A` |

In the topology, ports `1/1/1` to `1/1/10` can be used directly, for example `os1:1/1/1`. Ports 1/1/29A and 1/1/30A are addressed as `eth11` and `eth12`.

The management interface is the EMP port. With `CLAB_MGMT_PASSTHROUGH: "true"` the EMP port gets the node management address and is bridged to the containerlab management network.

## Features and options

### Node configuration

Once the switch has booted, the vrnetlab launcher logs in on the serial console, sets the system name and prompt to the node name, configures the EMP address and saves the configuration.

### Startup configuration

The [`startup-config`](../nodes.md#startup-config) file is pushed line by line on the serial console after the EMP address is configured, then saved with `write memory flash-synchro`. Use the AOS CLI format of `vcboot.cfg` or `show configuration snapshot`: one command per line, lines starting with `!` are skipped.

```yaml
topology:
  nodes:
    os1:
      kind: ale_vaos
      image: vrnetlab/ale_vaos:8.11.162.R01
      startup-config: os1.cfg
```

### Environment variables

| Variable          | Description |
| ----------------- | ----------- |
| `EMP_ROUTES`      | Static routes on EMP, `prefix=gateway` separated by commas. A default route (0.0.0.0/0) is not allowed on EMP and is skipped. |
| `VAOS_EXTRA_CMDS` | Extra CLI commands, separated by `;` |
| `VAOS_SAVE`       | `false` skips `write memory flash-synchro` |
| `VAOS_BOOTSTRAP`  | `false` skips the console bootstrap |

### Saving the configuration

`containerlab save` runs `write memory flash-synchro` on each vAOS node and stores the output of `show configuration snapshot` in `clab-<lab_name>/<node_name>/config/startup-config.cfg`.

## Known issues and limitations

- The node credentials must match an account of the vAOS image (default `admin` / `Switch@123`): they are used by the launcher to log in on the console.
- `containerlab save` uses the serial console: close any open console session on the node before saving.
