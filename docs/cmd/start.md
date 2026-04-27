# start command
<!-- --8<-- [start:lifecycle-commands] -->
The node's lifecycle command set consists of [start](start.md), [stop](stop.md), and [restart](restart.md) commands that allow users to individually control the lifecycle of the nodes in a deployed lab. The most common use case is to restart or stop+start the nodes that are stuck in some bad shape or require a reboot while keeping their dataplane interfaces intact.

With these commands you don't need to respin the whole topology when all you need is to restart a few nodes.

<!-- --8<-- [end:lifecycle-commands] -->

### Description

The `start` command starts one or more stopped nodes in a deployed lab and restores dataplane
interfaces parked by `containerlab stop`.

This command is intended to be used with nodes previously stopped by containerlab lifecycle
operations.

--8<-- "docs/cmd/deploy.md:env-vars-flags"

### Usage

`containerlab [global-flags] start [local-flags]`

### Flags

#### topology | name

Use the global `--topo | -t` flag to reference the lab topology file, or use the global `--name`
flag to reference an already deployed lab by name.

One of `--topo` or `--name` is required.

#### node

Use local `--node | -n` to select nodes to start.

The flag is repeatable and also supports comma-separated values:

```bash
containerlab start -t mylab.clab.yml --node r1 --node r2
containerlab start -t mylab.clab.yml --node r1,r2
```

If `--node` is omitted, all nodes in the selected lab are started.

### Limitations

<!-- --8<-- [start:limitations] -->
Node lifecycle operations (`stop`, `start`, `restart`) currently support only a subset of nodes:

- only veth dataplane links are supported
- root-namespace-based nodes are not supported
- nodes with `auto-remove` enabled are not supported
- only single-container nodes are supported
- `network-mode: container:<...>` users/providers are not supported
<!-- --8<-- [end:limitations] -->

### Examples

#### Start a single node by topology file

```bash
containerlab start -t mylab.clab.yml --node r1
```

#### Start multiple nodes by lab name

```bash
containerlab start --name mylab --node r1,r2
```

#### Start all nodes in a lab

```bash
containerlab start -t mylab.clab.yml
containerlab start --name mylab
```
