# gotty detach

## Description

The `detach` sub-command under the `tools gotty` command removes a GoTTY container from a lab network, terminating the web terminal session.

## Usage

```
containerlab tools gotty detach [flags]
```

## Flags

### `--lab | -l`

Name of the lab where the GoTTY container is attached.

### `--topology | -t`

Path to the topology file (`*.clab.yml`) to derive the lab name if `--lab` is not provided.

## Examples

```bash
❯ containerlab tools gotty detach -l mylab
11:40:03 INFO Removing GoTTY container clab-mylab-gotty
11:40:03 INFO GoTTY container clab-mylab-gotty removed successfully
```
