# api-server stop

## Description

The `stop` sub-command under the `tools api-server` command removes an API server container, terminating the API service. Once stopped, the API endpoints will no longer be available and any active client connections will be terminated.

Use this command when you want to shut down the API server and remove its container from the system.

## Usage

```
containerlab tools api-server stop [flags]
```

## Flags

### --name | -n

Name of the API server container to stop. Defaults to `clab-api-server`.

This should match the name used when starting the container with the `start` command.

## Examples

Stop the default API server container:

```bash
❯ containerlab tools api-server stop
10:28:28 INFO Removing API server container clab-api-server
10:28:28 INFO Removed container name=clab-api-server
10:28:28 INFO API server container clab-api-server removed successfully
```

Stop a specific named API server container:

```bash
❯ containerlab tools api-server stop --name prod-api-server
11:40:03 INFO Removing API server container prod-api-server
11:40:03 INFO API server container prod-api-server removed successfully
```

Attempt to stop a non-existent container:

```bash
❯ containerlab tools api-server stop --name missing-api-server
Error: failed to remove API server container: container "missing-api-server" not found
```

When stopping an API server container, the command will:
1. Find the container by name
2. Stop any running processes
3. Remove the container completely

After stopping the API server, all active API connections will be terminated and the service will no longer be available. This is a clean shutdown that ensures all resources are properly released.