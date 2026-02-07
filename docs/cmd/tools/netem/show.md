# Showing link impairments

With the `containerlab tools netem show` command users can list all link impairments for a given containerlab node.

For links with no associated qdisc the output will contain `N/A` values.

## Usage

```bash
containerlab tools netem show [local-flags]
```

## Flags

### node

With the `--node | -n` flag a user specifies the node name as defined in the topology file. This flag requires a topology file to be provided via `--topo | -t` (or a lab name via `--name`).

### container

With the `--container | -c` flag a user specifies the container name directly. This mode does not require a topology file.

One of `--node` or `--container` must be specified to show impairments for a single node. These flags are mutually exclusive.

When neither `--node` nor `--container` is specified, but a topology is provided via `--topo | -t`, the command shows impairments for all nodes in the topology.

### format

The optional `--format | -f` flag can be used to choose the output format. The default value is `table`, which displays the output in a formatted table. Specifying `json` returns the link impairment details in JSON format.

## Examples

### Showing link impairments for a node using node name

```bash
containerlab tools netem show -n r1 -t netem.clab.yml
+-----------+-------+--------+-------------+-------------+
| Interface | Delay | Jitter | Packet Loss | Rate (kbit) |
+-----------+-------+--------+-------------+-------------+
| lo        | N/A   | N/A    | N/A         | N/A         |
| eth0      | N/A   | N/A    | N/A         | N/A         |
| eth1      | 15ms  | 2ms    |        0.00 |           0 |
+-----------+-------+--------+-------------+-------------+
```

### Showing link impairments for a node using container name

```bash
containerlab tools netem show -c clab-netem-r1
```

### Showing link impairments for a node in json format

When displaying the netem details in json format, the fields have the following types:

* delay - string with the time suffix (ms, s, etc)
* jitter - string with the time suffix (ms, s, etc)
* packet_loss - a value with a floating point and 2 decimal places
* rate - an integer value expressed in kbit/s
* corruption - a value with a floating point and 2 decimal places

```bash
containerlab tools netem show -n srl -t netem.clab.yml --format json
```

<div class="embed-result">
```json
{
  "srl": [
    {
      "interface": "lo",
      "delay": "",
      "jitter": "",
      "packet_loss": 0,
      "rate": 0,
      "corruption": 0
    },
    {
      "interface": "mgmt0",
      "delay": "1s",
      "jitter": "5ms",
      "packet_loss": 0.1,
      "rate": 0,
      "corruption": 0.2
    },
    {
      "interface": "gway-2800",
      "delay": "",
      "jitter": "",
      "packet_loss": 0,
      "rate": 0,
      "corruption": 0
    },
    {
      "interface": "monit_in",
      "delay": "",
      "jitter": "",
      "packet_loss": 0,
      "rate": 0,
      "corruption": 0
    },
    {
      "interface": "mgmt0-0 (mgmt0.0)",
      "delay": "",
      "jitter": "",
      "packet_loss": 0,
      "rate": 0,
      "corruption": 0
    }
  ]
}
```
</div>

### Showing impairments for all nodes in a topology

When neither `--node` nor `--container` is specified, but `--topo` is provided, the command displays impairments for all nodes in the topology:

```bash
containerlab tools netem show -t netem.clab.yml
```

In table mode, each node's impairments are displayed under a `=== Node: <name> ===` header.

In JSON mode, the output is a single JSON object keyed by node name:

```bash
containerlab tools netem show -t netem.clab.yml --format json
```

<div class="embed-result">
```json
{
  "r1": [
    {
      "interface": "eth1",
      "delay": "15ms",
      "jitter": "2ms",
      "packet_loss": 0,
      "rate": 0,
      "corruption": 0
    }
  ],
  "r2": [
    {
      "interface": "eth1",
      "delay": "",
      "jitter": "",
      "packet_loss": 0,
      "rate": 0,
      "corruption": 0
    }
  ]
}
```
</div>
