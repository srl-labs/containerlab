# api-server status

## Description

The `status` sub-command under the `tools api-server` command displays information about all active Containerlab API server containers. This command provides a comprehensive view of all running API servers, including their configuration, status, and connection details.

This is useful for:
- Identifying all active API server instances
- Checking the configuration of running API servers
- Getting the connection details for API servers
- Monitoring the status of API server containers
- Seeing who created each API server instance

## Usage

```
containerlab tools api-server status [flags]
```

## Flags

### --format | -f

The output format for the status information. Possible values:

- `table` (default) - Displays the information in a formatted table
- `json` - Outputs the information in JSON format for programmatic access

## Examples

List all active API server containers in table format (default):

```bash
❯ containerlab tools api-server status
╭─────────────────┬─────────┬───────────┬──────┬────────────────────────┬─────────┬───────╮
│ NAME            │ STATUS  │ HOST      │ PORT │ LABS DIR               │ RUNTIME │ OWNER │
├─────────────────┼─────────┼───────────┼──────┼────────────────────────┼─────────┼───────┤
│ clab-api-server │ running │ localhost │ 8080 │ /opt/containerlab/labs │ docker  │ alice │
├─────────────────┼─────────┼───────────┼──────┼────────────────────────┼─────────┼───────┤
│ prod-api-server │ running │ localhost │ 9090 │ /home/labs/production  │ docker  │ bob   │
╰─────────────────┴─────────┴───────────┴──────┴────────────────────────┴─────────┴───────╯
```

List all API server containers in JSON format:

```bash
❯ containerlab tools api-server status -f json
[
  {
    "name": "clab-api-server",
    "state": "running",
    "host": "localhost",
    "port": 8080,
    "labs_dir": "/opt/containerlab/labs",
    "runtime": "docker",
    "owner": "alice",
    "environment": {
      "clab-node-kind": "linux",
      "clab-node-name": "clab-api-server",
      "clab-node-type": "tool",
      "clab-owner": "alice"
    }
  },
  {
    "name": "prod-api-server",
    "state": "running",
    "host": "localhost",
    "port": 9090,
    "labs_dir": "/home/labs/production",
    "runtime": "docker",
    "owner": "bob",
    "environment": {
      "clab-node-kind": "linux",
      "clab-node-name": "prod-api-server",
      "clab-node-type": "tool",
      "clab-owner": "bob"
    }
  }
]
```

When no active API server containers exist:

```bash
❯ containerlab tools api-server status
No active API server containers found

# Or in JSON format:
❯ containerlab tools api-server status -f json
[]
```

The status command displays the following information for each API server container:

- **NAME**: The name of the API server container
- **STATUS**: The current status of the container (running, stopped, etc.)
- **HOST**: The host address the API server is configured to use
- **PORT**: The port number the API server is listening on
- **LABS DIR**: The directory path mounted for lab files
- **RUNTIME**: The container runtime being used (docker/podman)
- **OWNER**: The user who created the API server container

In JSON format, additional environment information is included that provides more detailed metadata about the container configuration.