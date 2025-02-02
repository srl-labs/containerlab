# inspect interfaces subcommand

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

```bash
❯ clab inspect interfaces
╭─────────────────────────┬────────────────┬─────────────────┬────────┬─────────╮
│      Container Name     │ Interface Name │ Interface Alias │  Type  │  State  │
├─────────────────────────┼────────────────┼─────────────────┼────────┼─────────┤
│ clab-srlceos-ceos       │ eth0           │ N/A             │ veth   │ up      │
│                         ├────────────────┼─────────────────┼────────┼─────────┤
│                         │ lo             │ N/A             │ device │ unknown │
├─────────────────────────┼────────────────┼─────────────────┼────────┼─────────┤
│ clab-srlceos-srl        │ dummy-mgmt0    │ N/A             │ dummy  │ down    │
│                         ├────────────────┼─────────────────┼────────┼─────────┤
│                         │ e1-1           │ ethernet-1/1    │ veth   │ up      │
│                         ├────────────────┼─────────────────┼────────┼─────────┤
│                         │ e1-2           │ ethernet-1/2    │ veth   │ up      │
│                         ├────────────────┼─────────────────┼────────┼─────────┤
│                         │ e1-3           │ ethernet-1/3    │ veth   │ up      │
│                         ├────────────────┼─────────────────┼────────┼─────────┤
│                         │ gway-2800      │ N/A             │ veth   │ up      │
│                         ├────────────────┼─────────────────┼────────┼─────────┤
│                         │ lo             │ N/A             │ device │ unknown │
│                         ├────────────────┼─────────────────┼────────┼─────────┤
│                         │ mgmt0          │ N/A             │ veth   │ up      │
│                         ├────────────────┼─────────────────┼────────┼─────────┤
│                         │ mgmt0-0        │ mgmt0.0         │ veth   │ up      │
│                         ├────────────────┼─────────────────┼────────┼─────────┤
│                         │ monit_in       │ N/A             │ veth   │ up      │
├─────────────────────────┼────────────────┼─────────────────┼────────┼─────────┤
```

#### List the network interfaces of a specific node in a lab

```bash
❯ containerlab inspect interfaces --node clab-srlceos-ceos
╭───────────────────────┬────────────────┬─────────────────┬────────┬─────────╮
│     Container Name    │ Interface Name │ Interface Alias │  Type  │  State  │
├───────────────────────┼────────────────┼─────────────────┼────────┼─────────┤
│ clab-srlceos-ceos     │ eth0           │ N/A             │ veth   │ up      │
│                       ├────────────────┼─────────────────┼────────┼─────────┤
│                       │ lo             │ N/A             │ device │ unknown │
╰───────────────────────┴────────────────┴─────────────────┴────────┴─────────╯
```

#### List all nodes' network interfaces in a lab in JSON format

```bash
❯ clab inspect interfaces --format json
[
  {
    "name": "clab-srlceos-ceos",
    "interfaces": [
      {
        "name": "eth0",
        "type": "veth",
        "state": "up"
      },
      {
        "name": "lo",
        "type": "device",
        "state": "unknown"
      }
    ]
  },
  {
    "name": "clab-srlceos-srl",
    "interfaces": [
      {
        "name": "dummy-mgmt0",
        "type": "dummy",
        "state": "down"
      },
      {
        "name": "e1-1",
        "alias": "ethernet-1/1",
        "type": "veth",
        "state": "up"
      },
      {
        "name": "e1-2",
        "alias": "ethernet-1/2",
        "type": "veth",
        "state": "up"
      },
      {
        "name": "e1-3",
        "alias": "ethernet-1/3",
        "type": "veth",
        "state": "up"
      },
      {
        "name": "gway-2800",
        "type": "veth",
        "state": "up"
      },
      {
        "name": "lo",
        "type": "device",
        "state": "unknown"
      },
      {
        "name": "mgmt0",
        "type": "veth",
        "state": "up"
      },
      {
        "name": "mgmt0-0",
        "alias": "mgmt0.0",
        "type": "veth",
        "state": "up"
      },
      {
        "name": "monit_in",
        "type": "veth",
        "state": "up"
      }
    ]
  }
]
```
