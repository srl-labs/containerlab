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
{
  "clab-vlan-srl1": [
    {
      "interface": "lo",
      "delay": "N/A",
      "jitter": "N/A",
      "packet_loss": "N/A",
      "rate": "N/A",
      "corruption": "N/A"
    },
    {
      "interface": "mgmt0",
      "delay": "N/A",
      "jitter": "N/A",
      "packet_loss": "N/A",
      "rate": "N/A",
      "corruption": "N/A"
    },
    {
      "interface": "e1-1",
      "delay": "N/A",
      "jitter": "N/A",
      "packet_loss": "N/A",
      "rate": "N/A",
      "corruption": "N/A"
    },
    {
      "interface": "e1-10",
      "delay": "N/A",
      "jitter": "N/A",
      "packet_loss": "N/A",
      "rate": "N/A",
      "corruption": "N/A"
    },
    {
      "interface": "gway-2801",
      "delay": "N/A",
      "jitter": "N/A",
      "packet_loss": "N/A",
      "rate": "N/A",
      "corruption": "N/A"
    },
    {
      "interface": "monit_in",
      "delay": "N/A",
      "jitter": "N/A",
      "packet_loss": "N/A",
      "rate": "N/A",
      "corruption": "N/A"
    },
    {
      "interface": "mgmt0-0 (mgmt0.0)",
      "delay": "N/A",
      "jitter": "N/A",
      "packet_loss": "N/A",
      "rate": "N/A",
      "corruption": "N/A"
    }
  ]
}

```