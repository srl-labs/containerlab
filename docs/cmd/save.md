# save command

### Description

The `save` command perform configuration save for all the containers running in a lab.

The exact command that performs configuration save depends on a given kind. The below table explains the method used for each kind:

| Kind               | Command                                     | Notes                                                   |
| ------------------ | ------------------------------------------- | ------------------------------------------------------- |
| **Nokia SR Linux** | `sr_cli -d tools system configuration save` |                                                         |
| **Nokia SR OS**    |                                             | delivered via netconf RPC `copy-config running startup` |
| **Arista cEOS**    | `Cli -p 15 -c wr`                           |                                                         |
| **Cisco IOL**      | `write memory`                              |                                                         |

### Usage

`containerlab [global-flags] save [local-flags]`

### Flags

#### topology | name

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to spin up a lab.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will fail with an error.

#### node-filter

The local `--node-filter` flag allows users to specify a subset of topology nodes targeted by `save` command. The value of this flag is a comma-separated list of node names as they appear in the topology.

When a subset of nodes is specified, containerlab will only attempt to save configuration on the selected nodes.

### Examples

#### Save the configuration of the containers in a specific lab

Save the configuration of the containers running in lab named srl02

```bash
❯ containerlab save -n srl02
INFO[0001] clab-srl02-srl1: stdout: /system:
    Generated checkpoint '/etc/opt/srlinux/checkpoint/checkpoint-0.json' with name 'checkpoint-2020-11-18T09:00:54.998Z' and comment ''

INFO[0002] clab-srl02-srl2: stdout: /system:
    Generated checkpoint '/etc/opt/srlinux/checkpoint/checkpoint-0.json' with name 'checkpoint-2020-11-18T09:00:56.444Z' and comment ''
```
