# dc command

## Description

The `dc` (docker-connect) command under the `tools` command exec's into a running containerlab
container using the shell appropriate for that node's image. The shell is selected automatically
based on the container image, but can be overriden with the `-s` flag, see below.

When run from a directory that contains a single `*.clab.yml` topology file, the container list is
automatically scoped to that lab. On machines with many running labs this avoids ambiguous
partial-name matches. The scope can also be set explicitly with `--topo` or `--name`.

!!!note "Docker group membership"
    This command runs `docker exec` directly and does **not** require root privileges. The calling user must be a member of the **`docker`** group (or equivalent) for `docker exec` to succeed.

## Usage

```
containerlab tools dc [containername] [flags]
```

When called without a container name the command lists all containers in scope.

## Flags

### --shell | -s

Override the auto-detected shell. The value is split on whitespace and passed directly as the
command to `docker exec`, so multi-word commands are supported (e.g. `--shell "/bin/bash -l"`).

### --topo | -t

Path to a topology file. Restricts the container list to nodes defined in that topology.

### --name

Lab name. Restricts the container list to containers belonging to the named lab.

## Examples

List containers in the current lab (topology auto-discovered from cwd):

```bash
❯ clab tools dc
available containers:
  clab-mylab-router1 (ceos:latest)
  clab-mylab-router2 (git.ipng.ch/ipng/vpp-containerlab:stable)
  clab-mylab-host1 (ghcr.io/srl-labs/network-multitool)
```

Connect to a container by partial name:

```bash
❯ clab tools dc router1
# drops into the cEOS CLI
```

Connect with a shell override:

```bash
❯ clab tools dc --shell /bin/bash router1
# drops into a bash shell
```

Connect scoped to a specific topology file:

```bash
❯ clab tools dc -t mylab.clab.yml router2
# drops into a bash shell in the VPP dataplane
```
