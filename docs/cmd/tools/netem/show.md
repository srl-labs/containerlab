# Showing link impairments

With the `containerlab tools netem show` command users can list all link impairments for a given containerlab node.

For links with no associated qdisc the output will contain `N/A` values.

## Usage

```bash
containerlab tools netem show [local-flags]
```

## Flags

### node

With the mandatory `--node | -n` flag a user specifies the name of the containerlab node to show link impairments on.

## Examples

### Showing link impairments for a node

```bash
containerlab tools netem show -n clab-netem-r1
+-----------+-------+--------+-------------+-------------+
| Interface | Delay | Jitter | Packet Loss | Rate (kbit) |
+-----------+-------+--------+-------------+-------------+
| lo        | N/A   | N/A    | N/A         | N/A         |
| eth0      | N/A   | N/A    | N/A         | N/A         |
| eth1      | 15ms  | 2ms    |        0.00 |           0 |
+-----------+-------+--------+-------------+-------------+
```
