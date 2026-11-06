# logs command

### Description

The `logs` command shows the log of a lab node. With Docker and Podman it is the node container's log, with the [c9s runtime](../manual/clabernetes/runtime.md#logs) the log of the node's primary device container in its pod.

--8<-- "docs/cmd/deploy.md:env-vars-flags"

### Usage

`containerlab [global-flags] logs [local-flags]`

### Flags

#### topology | name

Use the global `--topo | -t` flag to reference the lab topology file, or use the global `--name` flag to reference an already deployed lab by name. Without either, containerlab looks for a `*.clab.yml` file in the current directory, like the other lab commands.

#### node

Use the local `--node | -n` flag to select the node. The flag is required. With the c9s runtime the node name is accepted as the topology file spells it or as it was sanitized for Kubernetes (`R1` or `r1`).

#### tail

The local `--tail` flag limits the output to the last N lines of the log. The default `-1` shows the whole log; `--tail 0` shows no previous lines, which is useful with `--follow`.

#### follow

The local `--follow | -f` flag keeps streaming new log lines until the command is interrupted.

#### timestamps

The local `--timestamps` flag prefixes every line with its timestamp in RFC 3339 format.

### Output

The log goes to stdout. With Docker and Podman, the container's stderr goes to stderr, unless the container has a TTY; Kubernetes returns one combined stream, written to stdout.

### Examples

#### Show the whole log of a node

```bash
containerlab logs -t srl.clab.yml -n srl1
```

#### Follow the last 100 lines with timestamps

```bash
containerlab logs --name srl -n srl1 --tail 100 --follow --timestamps
```

#### Show the device log of a node in a c9s lab

```bash
containerlab --runtime c9s logs --name clos -n srl1 --tail 50
```
