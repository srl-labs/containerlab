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

### Showing link impairments for a node in json format

When displaying the netem details in json format, the fields have the following types:

* delay - string with the time suffix (ms, s, etc)
* jitter - string with the time suffix (ms, s, etc)
* packet_loss - a value with a floating point and 2 decimal places
* rate - an integer value expressed in kbit/s
* corruption - a value with a floating point and 2 decimal places

```bash
containerlab tools netem show -n srl --format json
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
