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

--8<-- "docs/cmd/deploy.md:env-vars-flags"

### Usage

`containerlab [global-flags] save [local-flags]`

### Flags

#### topology | name

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to spin up a lab.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will fail with an error.

Alternatively, use the global `--name` flag to derive the topology from a running lab. This requires the lab to be running and its containers to have the `topo-file` label; otherwise the save command will fail.

#### node-filter

The local `--node-filter` flag allows users to specify a subset of topology nodes targeted by `save` command. The value of this flag is a comma-separated list of node names as they appear in the topology.

When a subset of nodes is specified, containerlab will only attempt to save configuration on the selected nodes.

#### copy

The local `--copy` flag allows users to copy saved configuration files to a dedicated directory. The path is resolved relative to the current working directory unless an absolute path is provided.

When `--copy` is specified, containerlab copies the saved configuration file from its original location in the lab directory to the destination with a UTC timestamp embedded in the filename. A symlink with the original filename is created (or updated) to always point to the latest timestamped copy.

```
<copy-path>/clab-<labname>/<node-name>/<config>-<YYMMDD_HHMMSS>.<ext>   # timestamped copy
<copy-path>/clab-<labname>/<node-name>/<config>.<ext>                    # symlink → latest
```

The destination directory is created automatically if it does not exist. Running `save --copy` multiple times to the same directory preserves all previous saves, allowing easy rollback to an earlier configuration.

The exact file that is copied depends on the node kind and corresponds to the configuration file produced by the save operation:

| Kind               | Copied file                                  |
| ------------------ | -------------------------------------------- |
| **Nokia SR Linux** | `config/config.json`                         |
| **Nokia SR OS**    | `<slot>/config/cf3/config.cfg`               |
| **Arista cEOS**    | `flash/startup-config`                       |

Node kinds that do not report a saved config path are silently skipped.

### Examples

#### Save the configuration of the containers in a specific lab

Save the configuration of the containers running in lab named srl02

```bash
❯ containerlab save --name srl02
INFO[0001] clab-srl02-srl1: stdout: /system:
    Generated checkpoint '/etc/opt/srlinux/checkpoint/checkpoint-0.json' with name 'checkpoint-2020-11-18T09:00:54.998Z' and comment ''

INFO[0002] clab-srl02-srl2: stdout: /system:
    Generated checkpoint '/etc/opt/srlinux/checkpoint/checkpoint-0.json' with name 'checkpoint-2020-11-18T09:00:56.444Z' and comment ''
```

#### Save and copy configs to a reusable directory

```bash
❯ containerlab save -t srl02.clab.yml --copy ./startup-configs
```

This creates timestamped copies with symlinks pointing to the latest save:

```
./startup-configs/clab-srl02/srl1/config-260207_091500.json   # timestamped copy
./startup-configs/clab-srl02/srl1/config.json                 # symlink → config-260207_091500.json
./startup-configs/clab-srl02/srl2/config-260207_091500.json
./startup-configs/clab-srl02/srl2/config.json
```

Running the same command again creates new timestamped files and updates the symlinks, while the previous saves remain in the directory.
