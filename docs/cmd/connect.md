# connect command

## Description

The `connect` command opens an interactive shell or an SSH session on a **single** lab node.

If more than one node matches the given filters, a beautiful TUI picker appears:

* Type to **fuzzy-search** on node names.
* Navigate with `↑/↓` (or `j/k`), press `enter` to select, `esc` to quit.
* The right-hand panel shows details of the highlighted node (kind, lab, image, state, IPv4/IPv6, ports).
* After picking a node, a second picker lets you choose **how** to connect — the node's default shell, a common fallback shell, or plain `ssh` via the per-lab SSH config.

When only one node matches, the node picker is skipped and the method picker is shown directly.

The topology file is auto-detected from the current working directory when `--topo` / `--name` are not given.

## Usage

`containerlab [global-flags] connect [local-flags] [node]`

## Flags

### node (positional)

An optional substring matched against node names **after** the topology/label filters have been applied. When it still matches more than one node, the TUI picker is shown with just those nodes.

### shell

`--shell | -s` forces a specific shell command and **skips the method picker**. The value is split on whitespace and passed to the runtime's `exec -it`, e.g. `--shell '/bin/bash -l'`.

### ssh

`--ssh` connects via plain SSH instead of a container shell and **skips the method picker**. This relies on the per-lab SSH client config that `clab deploy` writes to `/etc/ssh/ssh_config.d/clab-<lab>.conf`, so `ssh <node>` works without extra flags.

### label

Using `--label` it is possible to filter the nodes before the picker appears. The label is provided as a string in the form `key=value`.

### topology

With the global `--topo | -t` flag a user can set a path to the topology file that will be used to filter the nodes targeted for connection. The nodes can further be filtered with the `--label` flag.

## Method resolution

When the method picker is skipped or a default is accepted, the shell is resolved in the following priority order:

1. **`--shell` flag** — explicit CLI override, highest priority.
2. **`CLAB_EXEC_INTERACTIVE_SHELL` env var** — set on the node via the topology `env:` block (per-node, per-group, per-kind, or under `defaults:`). Useful when a specific node needs a different shell from the kind default, for example:
   ```yaml
   nodes:
     web:
       kind: linux
       image: example.com/myimage:latest
       env:
         CLAB_EXEC_INTERACTIVE_SHELL: /bin/bash -l
   ```
3. **Kind default** — each node kind ships a built-in default shell (e.g. `nokia_srlinux` uses `/opt/srlinux/bin/sr_cli`, `arista_ceos` uses `/usr/bin/Cli -p 15`, `fdio_vpp` uses `/usr/bin/nsenter --net=/run/netns/dataplane /bin/bash`).
4. **`/bin/sh`** — final fallback.

## Examples

### Connect to a node by name

```bash
❯ containerlab connect srl1
```

### Connect with a forced shell

```bash
❯ containerlab connect srl1 --shell '/bin/bash -l'
```

### Connect via SSH

```bash
❯ containerlab connect srl1 --ssh
```

### Fuzzy search across all nodes of a lab

```bash
❯ containerlab connect -t topo.clab.yml
```

Typing narrows the list live; `↑/↓` moves the cursor, `enter` selects the highlighted node and opens the method picker.

### Filter by label first

```bash
❯ containerlab connect --label clab-node-kind=nokia_srlinux
```

## See also

* [`exec` command](exec.md) — run a command non-interactively on many nodes.
