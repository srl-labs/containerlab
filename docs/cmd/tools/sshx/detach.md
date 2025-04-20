# sshx detach

## Description

The `detach` sub-command under the `tools sshx` command removes an SSHX container from a lab network, terminating the terminal sharing session. Once detached, the sharing link will no longer be functional and any active browser sessions will be disconnected.

Use this command when you've completed your collaboration session and want to remove the sharing capability.

## Usage

`containerlab tools sshx detach [local-flags]`

## Flags

### network

The network where the SSHX container is attached, specified with `--network | -n` flag. Defaults to `clab`.

If a topology file (*.clab.yml) is present in the current directory, the network name will be automatically detected from it.
Or if it is given via -t --topology

### name

The name of the SSHX container to detach, specified with `--name` flag.

If not provided, the name will be automatically determined as `sshx-<network>` where `<network>` is the network name with any `clab-` prefix removed.

## Examples

```bash
# Detach the SSHX container from the default network
❯ containerlab tools sshx detach
INFO[0000] Removing SSHX container sshx-default
INFO[0001] SSHX container sshx-default removed successfully

# Detach from a specific network
❯ containerlab tools sshx detach -n clab-mylab
INFO[0000] Removing SSHX container sshx-mylab
INFO[0001] SSHX container sshx-mylab removed successfully

# Detach a container with a custom name
❯ containerlab tools sshx detach --name my-shared-terminal
INFO[0000] Removing SSHX container my-shared-terminal
INFO[0001] SSHX container my-shared-terminal removed successfully

# When the container doesn't exist
❯ containerlab tools sshx detach -n clab-nonexistent
INFO[0000] SSHX container sshx-nonexistent does not exist, nothing to detach
```

After detaching the SSHX container, the terminal sharing session is terminated, and the shareable link will no longer work. This effectively ends the collaboration session and removes the container from your system.