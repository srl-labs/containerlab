# Resetting Link Impairments

With the `containerlab tools netem reset` command users can remove (reset) all network impairments on a specified interface of a containerlab node. This command deletes the netem qdisc associated with the interface, restoring it to its default state. If no impairments are present, the command will complete successfully without error.

## Usage

```bash
containerlab tools netem reset [local-flags]
```

## Flags

### node

The mandatory `--node | -n` flag specifies the name of the containerlab node on which to reset link impairments.

### interface

The mandatory `--interface | -i` flag specifies the interface on which the netem impairments should be reset.

## Examples

### Resetting impairments on an interface

This example resets the impairments on the interface `eth1` of node `clab-netem-r1`:

```bash
containerlab tools netem reset -n clab-netem-r1 -i eth1
```

Output:

```bash
Reset impairments on node "clab-netem-r1", interface "eth1"
```

### Resetting impairments when none are set

If no impairments are configured on the specified interface, the command will complete without error:

```bash
containerlab tools netem reset -n clab-netem-r1 -i eth0
```

Output:

```bash
Reset impairments on node "clab-netem-r1", interface "eth0"
```
