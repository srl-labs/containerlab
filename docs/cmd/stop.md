# stop command

### Description

The `stop` command stops one or more nodes in a deployed lab while keeping dataplane links intact.
For each selected node, containerlab parks veth interfaces in a dedicated namespace before stopping
the container.

--8<-- "docs/cmd/deploy.md:env-vars-flags"

### Usage

`containerlab [global-flags] stop [local-flags]`

### Flags

#### topology | name

Use the global `--topo | -t` flag to reference the lab topology file, or use the global `--name`
flag to reference an already deployed lab by name.

One of `--topo` or `--name` is required.

#### node

Use local `--node | -n` to select nodes to stop.

The flag is repeatable and also supports comma-separated values:

```bash
containerlab stop -t mylab.clab.yml --node r1 --node r2
containerlab stop -t mylab.clab.yml --node r1,r2
```

If `--node` is omitted, all nodes in the selected lab are stopped.

### Limitations

Node lifecycle operations (`stop`, `start`, `restart`) currently support only a subset of nodes:

- only veth dataplane links are supported
- root-namespace-based nodes are not supported
- nodes with `auto-remove` enabled are not supported
- only single-container nodes are supported
- `network-mode: container:<...>` users/providers are not supported

### Examples

#### Stop a single node by topology file

```bash
containerlab stop -t mylab.clab.yml --node r1
```

#### Stop multiple nodes by lab name

```bash
containerlab stop --name mylab --node r1,r2
```

#### Stop all nodes in a lab

```bash
containerlab stop -t mylab.clab.yml
containerlab stop --name mylab
```
