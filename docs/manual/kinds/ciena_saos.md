---
search:
  boost: 4
kind_code_name: ciena_saos
kind_display_name: SAOS
---
# Ciena SAOS

Ciena SAOS virtualized switch is identified with the `ciena_saos` kind in the
[topology file](../topo-def-file.md). It is built using the [vrnetlab](../vrnetlab.md)
project and runs as a Qemu VM packaged in a container.

## Managing ciena_saos nodes

Ciena SAOS nodes launched with containerlab can be managed via the following interfaces:

=== "bash"
    to connect to a `bash` shell of a running ciena_saos container:
    ```bash
    docker exec -it <container-name/id> bash
    ```
=== "CLI via SSH"
    to connect to the SAOS CLI (password `ciena123`)
    ```bash
    ssh diag@<container-name/id>
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
3948, 3984, 3985, 5130, 5131, 5132, 5134, 5144, 5162, 5164, 5166, 5168,
5169, 5170, 5171, 5184, 5186, 8110, 8112, 8114, 8140, 8190, 8192
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

Provide a `vrnetlab/ciena_saos:<tag>` image explicitly:

```yaml
topology:
  nodes:
    saos-1:
      kind: ciena_saos
      image: vrnetlab/ciena_saos:10-11-01-0248
      type: 5132
```

### Startup configuration

Startup configuration is not supported yet.
