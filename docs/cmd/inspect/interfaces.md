# Inspect interfaces subcommand

### Description

The `inspect interfaces` subcommand provides information about the network interfaces of deployed nodes of a deployed lab.
The subcommand gathers information directly from deployed containers, and displays information about their operational state, network interface type, and if applicable, the applied network interface alias.

### Usage

`containerlab [global-flags] inspect interfaces [local-flags]`

A shorthand can be used for this subcommand: `int` or `intf`.

### Flags

#### topology | name

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to spin up a lab.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will fail with an error.

#### name

The optional local `--node | -n` flag limits the interface listing output to a specific containerlab node.

#### format

The optional local `--format | -f` flag enables different output stylings. By default the `table` format will be used.

Currently, the only other format option is `json`, which will produce the output in JSON format.

### Examples

#### List all nodes' network interfaces in a lab

```
❯ clab inspect interfaces
╭─────────────────────────┬─────────────┬──────────────┬───────────────────┬───────┬───────┬────────┬─────────╮
│      Container Name     │     Name    │     Alias    │        MAC        │ Index │   MTU │  Type  │  State  │
├─────────────────────────┼─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│ clab-srlceos-ceos       │ eth0        │ N/A          │ 02:42:ac:14:14:03 │   719 │  1500 │ veth   │ up      │
│                         ├─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│                         │ lo          │ N/A          │                   │     1 │ 65536 │ device │ unknown │
├─────────────────────────┼─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│ clab-srlceos-srl        │ dummy-mgmt0 │ N/A          │ 92:19:85:42:c3:11 │     2 │  1500 │ dummy  │ down    │
│                         ├─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│                         │ e1-1        │ ethernet-1/1 │ 1a:c5:01:ff:00:01 │   724 │  9232 │ veth   │ up      │
│                         ├─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│                         │ e1-2        │ ethernet-1/2 │ 1a:c5:01:ff:00:02 │   726 │  9232 │ veth   │ up      │
│                         ├─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│                         │ e1-3        │ ethernet-1/3 │ 1a:c5:01:ff:00:03 │   728 │  9232 │ veth   │ up      │
│                         ├─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│                         │ gway-2800   │ N/A          │ ea:56:5c:31:f2:a4 │     3 │  1500 │ veth   │ up      │
│                         ├─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│                         │ lo          │ N/A          │                   │     1 │ 65536 │ device │ unknown │
│                         ├─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│                         │ mgmt0       │ N/A          │ 02:42:ac:14:14:02 │   717 │  1514 │ veth   │ up      │
│                         ├─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│                         │ mgmt0-0     │ mgmt0.0      │ 2a:2c:2d:8c:12:f1 │     6 │  1500 │ veth   │ up      │
│                         ├─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│                         │ monit_in    │ N/A          │ fa:94:55:db:17:aa │     5 │  9234 │ veth   │ up      │
├─────────────────────────┼─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
```

#### List the network interfaces of a specific node in a lab

```
❯ containerlab inspect interfaces --node clab-srlceos-ceos
╭─────────────────────────┬─────────────┬──────────────┬───────────────────┬───────┬───────┬────────┬─────────╮
│      Container Name     │     Name    │     Alias    │        MAC        │ Index │   MTU │  Type  │  State  │
├─────────────────────────┼─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│ clab-srlceos-ceos       │ eth0        │ N/A          │ 02:42:ac:14:14:03 │   719 │  1500 │ veth   │ up      │
│                         ├─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
│                         │ lo          │ N/A          │                   │     1 │ 65536 │ device │ unknown │
├─────────────────────────┼─────────────┼──────────────┼───────────────────┼───────┼───────┼────────┼─────────┤
```

#### List all nodes' network interfaces in a lab in JSON format

```bash
❯ clab inspect interfaces --format json
[
  {
    "name": "clab-srlvjunos02-ceos",
    "interfaces": [
      {
        "name": "eth0",
        "alias": "",
        "mac": "02:42:ac:14:14:03",
        "ifindex": 719,
        "mtu": 1500,
        "type": "veth",
        "state": "up"
      },
      {
        "name": "lo",
        "alias": "",
        "mac": "",
        "ifindex": 1,
        "mtu": 65536,
        "type": "device",
        "state": "unknown"
      }
    ]
  },
  {
    "name": "clab-srlvjunos02-srl",
    "interfaces": [
      {
        "name": "dummy-mgmt0",
        "alias": "",
        "mac": "92:19:85:42:c3:11",
        "ifindex": 2,
        "mtu": 1500,
        "type": "dummy",
        "state": "down"
      },
      {
        "name": "e1-1",
        "alias": "ethernet-1/1",
        "mac": "1a:c5:01:ff:00:01",
        "ifindex": 724,
        "mtu": 9232,
        "type": "veth",
        "state": "up"
      },
      {
        "name": "e1-2",
        "alias": "ethernet-1/2",
        "mac": "1a:c5:01:ff:00:02",
        "ifindex": 726,
        "mtu": 9232,
        "type": "veth",
        "state": "up"
      },
      {
        "name": "e1-3",
        "alias": "ethernet-1/3",
        "mac": "1a:c5:01:ff:00:03",
        "ifindex": 728,
        "mtu": 9232,
        "type": "veth",
        "state": "up"
      },
      {
        "name": "gway-2800",
        "alias": "",
        "mac": "ea:56:5c:31:f2:a4",
        "ifindex": 3,
        "mtu": 1500,
        "type": "veth",
        "state": "up"
      },
      {
        "name": "lo",
        "alias": "",
        "mac": "",
        "ifindex": 1,
        "mtu": 65536,
        "type": "device",
        "state": "unknown"
      },
      {
        "name": "mgmt0",
        "alias": "",
        "mac": "02:42:ac:14:14:02",
        "ifindex": 717,
        "mtu": 1514,
        "type": "veth",
        "state": "up"
      },
      {
        "name": "mgmt0-0",
        "alias": "mgmt0.0",
        "mac": "2a:2c:2d:8c:12:f1",
        "ifindex": 6,
        "mtu": 1500,
        "type": "veth",
        "state": "up"
      },
      {
        "name": "monit_in",
        "alias": "",
        "mac": "fa:94:55:db:17:aa",
        "ifindex": 5,
        "mtu": 9234,
        "type": "veth",
        "state": "up"
      }
    ]
  }
]
```
