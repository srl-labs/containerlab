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

### format
The optional --format | -f flag can be used to choose the output format. The default value is table, which displays the output in a formatted table. Specifying json returns the link impairment details in JSON format.

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

### Showing link impairments for a node

```bash
containerlab tools netem show -n clab-netem-r1 --format json
[
  {
    "Interface": "lo",
    "NodeName": "clab-netem-r1",
    "Delay": "N/A",
    "Jitter": "N/A",
    "Packet Loss": "N/A",
    "Rate (Kbit)": "N/A",
    "Corruption": "N/A"
  },
  {
    "Interface": "eth0",
    "NodeName": "clab-netem-r1",
    "Delay": "N/A",
    "Jitter": "N/A",
    "Packet Loss": "N/A",
    "Rate (Kbit)": "N/A",
    "Corruption": "N/A"
  },
  {
    "Interface": "eth1",
    "NodeName": "clab-netem-r1",
    "Delay": "15ms",
    "Jitter": "2ms",
    "Packet Loss": "0.00%",
    "Rate (Kbit)": "0",
    "Corruption": "N/A"
  }
]

```