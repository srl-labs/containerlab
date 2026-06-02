# sshx reattach

## Description

The `reattach` sub-command under the `tools sshx` command combines the functionality of `detach` and `attach` in a single operation. It removes any existing SSHX container from a lab network and then creates a new one, effectively restarting the terminal sharing session.

This command is particularly useful when:
- You need to refresh the SSHX session with a new link
- The current SSHX container has become unresponsive
- You want to change the container's configuration (such as enabling read-only access)
- You need to quickly restart the sharing session without running two separate commands

## Usage

```
containerlab tools sshx reattach [flags]
```

## Flags

### --lab | -l

Name of the lab to reattach the SSHX container to. This directly specifies the lab name to use.

### --topology | -t

Path to the topology file (`*.clab.yml`) that defines the lab. This global flag can be provided instead of the lab name provided with the `--lab | -l` flag. When lab name is not provided and the topology is provided, containerlab will extract the lab name from the topology file.

### --name

Name of the SSHX container. If not provided, the name will be automatically generated as `clab-<labname>-sshx`.

### --enable-readers | -w

Enable read-only access links. When enabled, the command will generate an additional link that can be shared with users who should have read-only access to the terminal session.

### --image | -i

The container image to use for SSHX client. Defaults to [`ghcr.io/srl-labs/network-multitool`](https://github.com/srl-labs/network-multitool).

### --owner | -o

The owner name to associate with the SSHX container. If not provided, it will be determined from environment variables (SUDO_USER or USER).

### --expose-ssh | -s

Mount the host's SSH directory (~/.ssh) into the container. This allows the SSHX session to use your existing SSH keys to connect to lab nodes.

## Examples

Reattach an SSHX container to a lab specified by lab name:

```bash
❯ containerlab tools sshx reattach -l mylab
11:40:03 INFO Removing existing SSHX container clab-mylab-sshx if present...
11:40:03 INFO Successfully removed existing SSHX container
11:40:03 INFO Pulling image ghcr.io/srl-labs/network-multitool...
11:40:04 INFO Creating new SSHX container clab-mylab-sshx on network 'clab-mylab'
11:40:04 INFO Creating container name=clab-mylab-sshx
11:40:04 INFO SSHX container clab-mylab-sshx started. Waiting for SSHX link...
11:40:09 INFO SSHX successfully reattached link=https://sshx.io/s#sessionid,accesskey note=
  │ Inside the shared terminal, you can connect to lab nodes using SSH:
  │ ssh admin@clab-mylab-<node-name>
```

Reattach with read-only access enabled:

```bash
❯ containerlab tools sshx reattach -l mylab --enable-readers
11:40:03 INFO Removing existing SSHX container clab-mylab-sshx if present...
11:40:03 INFO Successfully removed existing SSHX container
11:40:03 INFO Pulling image ghcr.io/srl-labs/network-multitool...
11:40:04 INFO Creating new SSHX container clab-mylab-sshx on network 'clab-mylab'
11:40:04 INFO Creating container name=clab-mylab-sshx
11:40:04 INFO SSHX container clab-mylab-sshx started. Waiting for SSHX link...
11:40:09 INFO SSHX successfully reattached link=https://sshx.io/s#sessionid,accesskey note=
  │ Inside the shared terminal, you can connect to lab nodes using SSH:
  │ ssh admin@clab-mylab-<node-name>

Read-only access link:
https://sshx.io/s#readonlyid
```

Reattach with exposed SSH keys and custom container name:

```bash
❯ containerlab tools sshx reattach -l mylab --name my-terminal --expose-ssh
11:40:03 INFO Removing existing SSHX container my-terminal if present...
11:40:03 INFO Successfully removed existing SSHX container
11:40:03 INFO Pulling image ghcr.io/srl-labs/network-multitool...
11:40:04 INFO Creating new SSHX container my-terminal on network 'clab-mylab'
11:40:04 INFO Creating container name=my-terminal
11:40:04 INFO SSHX container my-terminal started. Waiting for SSHX link...
11:40:09 INFO SSHX successfully reattached link=https://sshx.io/s#sessionid,accesskey note=
  │ Inside the shared terminal, you can connect to lab nodes using SSH:
  │ ssh admin@clab-mylab-<node-name>

Your SSH keys and configuration have been mounted to allow direct authentication.
```

The `reattach` command provides a convenient way to restart your terminal sharing session without having to run separate detach and attach commands. This is particularly useful when troubleshooting or when you need to refresh your session with new configuration settings.