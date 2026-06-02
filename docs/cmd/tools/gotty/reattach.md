# gotty reattach

## Description

The `reattach` sub-command under the `tools gotty` command removes any existing GoTTY container from a lab and then creates a new one with the same parameters. This is useful for refreshing the web terminal session.

## Usage

```
containerlab tools gotty reattach [flags]
```

## Flags

Same as the `attach` command (`--lab`, `--name`, `--port`, `--username`, `--password`, `--shell`, `--image`, `--owner`).

## Examples

```bash
❯ containerlab tools gotty reattach -l mylab
11:40:03 INFO Removing existing GoTTY container clab-mylab-gotty if present...
11:40:03 INFO Pulling image ghcr.io/srl-labs/network-multitool...
11:40:04 INFO Creating new GoTTY container clab-mylab-gotty on network 'clab-mylab'
11:40:05 INFO GoTTY container clab-mylab-gotty started. Waiting for GoTTY service to initialize...
11:40:10 INFO GoTTY web terminal successfully reattached url=http://HOST_IP:8080 username=admin password=admin
```
