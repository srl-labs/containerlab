# restart command

### Description

The `restart` command restarts one or more nodes in a deployed lab by performing a lifecycle-aware
stop+start operation.

For running nodes, containerlab parks dataplane interfaces before stop and restores them on start.
For already stopped nodes, containerlab performs the start/restore phase.

--8<-- "docs/cmd/deploy.md:env-vars-flags"

### Usage

`containerlab [global-flags] restart [local-flags]`

### Flags

#### topology | name

Use the global `--topo | -t` flag to reference the lab topology file, or use the global `--name`
flag to reference an already deployed lab by name.

One of `--topo` or `--name` is required.

#### node

Use local `--node | -n` to select nodes to restart.

The flag is repeatable and also supports comma-separated values:

```bash
containerlab restart -t mylab.clab.yml --node r1 --node r2
containerlab restart -t mylab.clab.yml --node r1,r2
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

#### Restart a single node by topology file

```bash
containerlab restart -t mylab.clab.yml --node r1
```

#### Restart multiple nodes by lab name

```bash
containerlab restart --name mylab --node r1,r2
```
