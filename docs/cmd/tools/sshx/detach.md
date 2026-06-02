# sshx detach

## Description

The `detach` sub-command under the `tools sshx` command removes an SSHX container from a lab network, terminating the terminal sharing session. Once detached, the sharing link will no longer be functional and any active browser sessions will be disconnected.

Use this command when you've completed your collaboration session and want to remove the sharing capability.

## Usage

```
containerlab tools sshx detach [flags]
```

## Flags

### --lab | -l

Name of the lab where the SSHX container is attached. This directly specifies the lab name to use.

### --topology | -t

Path to the topology file (*.clab.yml) that defines the lab. This flag is defined at the global level. If provided without specifying a lab name via `-l`, containerlab will extract the lab name from this file.

## Examples

```bash
# Detach the SSHX container from a lab specified by lab name
❯ containerlab tools sshx detach -l mylab
11:40:03 INFO Removing SSHX container clab-mylab-sshx
11:40:03 INFO SSHX container clab-mylab-sshx removed successfully

# Detach using a specific topology file
❯ containerlab tools sshx detach -t mylab.clab.yml
11:40:03 INFO Parsing & checking topology file=mylab.clab.yml
11:40:03 INFO Removing SSHX container clab-mylab-sshx
11:40:03 INFO SSHX container clab-mylab-sshx removed successfully

# Using auto-discovered topology file in current directory
❯ containerlab tools sshx detach
11:40:03 INFO Parsing & checking topology file=mylab.clab.yml
11:40:03 INFO Removing SSHX container clab-mylab-sshx
11:40:03 INFO SSHX container clab-mylab-sshx removed successfully
```

After detaching the SSHX container, the terminal sharing session is terminated, and the shareable link will no longer work. This effectively ends the collaboration session and removes the container from your system.
