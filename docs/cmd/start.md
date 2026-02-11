# start command

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

At least one `--node` value is required.

### Limitations

Node lifecycle operations (`stop`, `start`, `restart`) currently support only a subset of nodes:

- only veth dataplane links are supported
- root-namespace-based nodes are not supported
- nodes with `auto-remove` enabled are not supported
- only single-container nodes are supported
- `network-mode: container:<...>` users/providers are not supported

### Examples

#### Start a single node by topology file

```bash
containerlab start -t mylab.clab.yml --node r1
```

#### Start multiple nodes by lab name

```bash
containerlab start --name mylab --node r1,r2
```
