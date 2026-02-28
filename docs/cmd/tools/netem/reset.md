# Resetting Link Impairments

With the `containerlab tools netem reset` command users can remove (reset) all network impairments on a specified interface of a containerlab node. This command deletes the netem qdisc associated with the interface, restoring it to its default state. If no impairments are present, the command will complete successfully without error.

## Usage

```bash
containerlab tools netem reset [local-flags]
```

## Flags

### node

With the `--node | -n` flag a user specifies the node name as defined in the topology file. This flag requires a topology file to be provided via `--topo | -t` (or a lab name via `--name`).

### container

With the `--container | -c` flag a user specifies the container name directly. This mode does not require a topology file.

One of `--node` or `--container` must be specified. These flags are mutually exclusive.

### interface

The mandatory `--interface | -i` flag specifies the interface on which the netem impairments should be reset.

## Examples

### Resetting impairments using node name

This example resets the impairments on the interface `eth1` of node `r1` defined in the topology:

```bash
containerlab tools netem reset -n r1 -t netem.clab.yml -i eth1
```

Output:

```bash
Reset impairments on node "r1", interface "eth1"
```

### Resetting impairments using container name

This example resets the impairments on the interface `eth1` of container `clab-netem-r1`:

```bash
containerlab tools netem reset -c clab-netem-r1 -i eth1
```

Output:

```bash
Reset impairments on node "clab-netem-r1", interface "eth1"
```

### Resetting impairments when none are set

If no impairments are configured on the specified interface, the command will complete without error:

```bash
containerlab tools netem reset -c clab-netem-r1 -i eth0
```

Output:

```bash
Reset impairments on node "clab-netem-r1", interface "eth0"
```
