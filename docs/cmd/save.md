# save command

### Description

The `save` command perform configuration save for all the containers running in a lab.

The exact command that performs configuration save depends on a given kind. The below table explains the method used for each kind:

| Kind               | Command                                     | Notes                                                   |
| ------------------ | ------------------------------------------- | ------------------------------------------------------- |
| **Nokia SR Linux** | `sr_cli -d tools system configuration save` |                                                         |
| **Nokia SR OS**    |                                             | delivered via netconf RPC `copy-config running startup` |
| **Arista cEOS**    | `Cli -p 15 -c wr`                           |                                                         |


### Usage

`containerlab [global-flags] save [local-flags]`

### Flags

#### topology | name

With the global `--topo | -t` or `--name | -n` flag a user specifies from which lab to take the containers and perform the save configuration task.

### Examples

```bash
# save the configuration of the containers running in lab named srl02
❯ containerlab save -n srl02
INFO[0001] clab-srl02-srl1: stdout: /system:
    Generated checkpoint '/etc/opt/srlinux/checkpoint/checkpoint-0.json' with name 'checkpoint-2020-11-18T09:00:54.998Z' and comment ''

INFO[0002] clab-srl02-srl2: stdout: /system:
    Generated checkpoint '/etc/opt/srlinux/checkpoint/checkpoint-0.json' with name 'checkpoint-2020-11-18T09:00:56.444Z' and comment ''
```