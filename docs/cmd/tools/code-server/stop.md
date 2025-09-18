# code-server stop

## Description

The `stop` sub-command under the `tools code-server` command removes a running code-server container. Use it to tear down the VS Code web terminal once you are done editing lab files. The command deletes the container but leaves the persistent configuration, extension, and user-data directories on disk so that the next start is nearly instant.

## Usage

```
containerlab tools code-server stop [flags]
```

## Flags

### --name | -n

Name of the code-server container to remove. Defaults to `clab-code-server`.

## Examples

Stop the default helper container:

```bash
❯ containerlab tools code-server stop
16:42:13 INFO Removing code-server container name=clab-code-server
16:42:13 INFO Removed container name=clab-code-server
16:42:13 INFO code server container removed name=clab-code-server
```

Target a specific container name:

```bash
❯ containerlab tools code-server stop --name dev-code-server
16:45:01 INFO Removing code-server container name=dev-code-server
16:45:01 INFO Removed container name=dev-code-server
16:45:01 INFO code server container removed name=dev-code-server
```

If the named container is not found the command returns an error from the underlying runtime (for example `container "dev-code-server" not found`).
