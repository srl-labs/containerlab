# sshx attach

## Description

The `attach` sub-command under the `tools sshx` command creates and starts a container that runs the [SSHX](https://sshx.io/) client. SSHX client establishes a terminal sharing session and provides a shareable web link that allows others to access the container's shell via their browser.

SSHX terminal sharing is particularly useful for:

- Remote collaboration on lab environments
- Sharing terminal access with team members
- Providing troubleshooting assistance
- Conducting demonstrations without giving direct access to your system

## Usage

```
containerlab tools sshx attach [flags]
```

## Flags

### --lab | -l

Name of the lab to attach the SSHX container to. This directly specifies the lab name to use.

### --topology | -t

Path to the topology file (`*.clab.yml`) that defines the lab. This global flag can be provided instead of the lab name provided with the `--lab | -l` flag. When lab name is not provided and the topology is provided, containerlab will extract the lab name from the topology file.

### --name

Name of the SSHX container. If not provided, the name will be automatically generated as `clab-<labname>-sshx`.

### --enable-readers | -w

Enable read-only access links. When enabled, the command will generate an additional link that can be shared with users who should have read-only access to the terminal session.

<!-- TODO: ssh mounting removed for now, as it needs to work with non root users -->
<!-- ### --expose-ssh | -s

Mount the host's SSH directory (~/.ssh) into the container. This allows the SSHX session to use your existing SSH keys to connect to lab nodes. Enabled by default. `--expose-ssh=true` -->

### --image | -i

The container image to use for SSHX client. Defaults to [`ghcr.io/srl-labs/network-multitool`](https://github.com/srl-labs/network-multitool).

### --owner | -o

The owner name to associate with the SSHX container. If not provided, it will be determined from environment variables (SUDO_USER or USER).

### --format | -f

Output format for the command. Defined at the parent command level and applies to the `list` command. Values: `table` (default) or `json`.

## Examples

Attach an SSHX container to a lab specified by lab name

```bash
❯ containerlab tools sshx attach -l mylab
11:40:03 INFO Pulling image ghcr.io/srl-labs/network-multitool...
11:40:03 INFO Creating SSHX container clab-mylab-sshx on network 'clab-mylab'
11:40:03 INFO Creating container name=clab-mylab-sshx
11:40:03 INFO SSHX container clab-mylab-sshx started. Waiting for SSHX link...
SSHX link for collaborative terminal access:
https://sshx.io/s#sessionid,accesskey

Inside the shared terminal, you can connect to lab nodes using SSH:
ssh admin@clab-mylab-node1
```

Attach SSHX without exposing SSH keys

Attach with a specific topology file

```bash
❯ containerlab tools sshx attach -t mylab.clab.yml
11:40:03 INFO Parsing & checking topology file=mylab.clab.yml
11:40:03 INFO Pulling image ghcr.io/srl-labs/network-multitool...
11:40:03 INFO Creating SSHX container clab-mylab-sshx on network 'clab-mylab'
11:40:03 INFO Creating container name=clab-mylab-sshx
11:40:03 INFO SSHX container clab-mylab-sshx started. Waiting for SSHX link...
SSHX link for collaborative terminal access:
https://sshx.io/s#sessionid,accesskey

Inside the shared terminal, you can connect to lab nodes using SSH:
ssh admin@clab-mylab-node1
```

Attach with a custom container name

```
❯ containerlab tools sshx attach -l mylab --name my-shared-terminal
11:40:03 INFO Pulling image ghcr.io/srl-labs/network-multitool...
11:40:03 INFO Creating SSHX container my-shared-terminal on network 'clab-mylab'
11:40:03 INFO Creating container name=my-shared-terminal
11:40:03 INFO SSHX container my-shared-terminal started. Waiting for SSHX link...
SSHX link for collaborative terminal access:
https://sshx.io/s#sessionid,accesskey

Inside the shared terminal, you can connect to lab nodes using SSH:
ssh admin@clab-mylab-node1

```

Attach with read-only access enabled

```bash
❯ containerlab tools sshx attach -l mylab --enable-readers
11:40:03 INFO Pulling image ghcr.io/srl-labs/network-multitool...
11:40:03 INFO Creating SSHX container clab-mylab-sshx on network 'clab-mylab'
11:40:03 INFO Creating container name=clab-mylab-sshx
11:40:03 INFO SSHX container clab-mylab-sshx started. Waiting for SSHX link...
SSHX link for collaborative terminal access:
https://sshx.io/s#sessionid,accesskey

Read-only access link:
https://sshx.io/s#sessionid,accesskey

Inside the shared terminal, you can connect to lab nodes using SSH:
ssh admin@clab-mylab-node1
```

When the SSHX container is attached, anyone with the sharing link can access your terminal session through their web browser. From this shared terminal, they can connect to any node in the lab using SSH.

The container is automatically connected to the lab's management network, which provides DNS resolution for all lab nodes. This allows you to use node names like `clab-mylab-node1` directly in SSH commands without needing to know specific IP addresses.

When the session is no longer needed, use the `detach` command to remove the container and terminate the session.
