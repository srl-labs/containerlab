# api-server start

## Description

The `start` sub-command under the `tools api-server` command creates and starts a container that runs the Containerlab API server. The API server provides a RESTful HTTP interface for managing Containerlab operations programmatically, including lab deployment, node management, and configuration tasks.

The API server is particularly useful for:

- Automating lab deployments and management
- Integrating Containerlab with other systems and workflows
- Providing a web-based interface for lab management
- Managing multiple labs programmatically
- Implementing custom lab automation solutions

## Usage

```
containerlab tools api-server start [flags]
```

## Flags

### --port | -p

Port to expose the API server on. Defaults to `8080`.

### --host

Host address for the API server. Defaults to `localhost`.

### --image | -i

Container image to use for the API server. Defaults to `ghcr.io/srl-labs/clab-api-server/clab-api-server:latest`.

### --log-level

Log level for the API server. Options: `debug` (default), `info`, `warn`, `error`.

### --name | -n

Name of the API server container. Defaults to `clab-api-server`.

### --tls-enable

Enable TLS for the API server. Defaults to `false`.

### --tls-cert

Path to TLS certificate file (required if TLS is enabled).

### --tls-key

Path to TLS key file (required if TLS is enabled).

### --ssh-base-port

SSH proxy base port. Defaults to `2223`.

### --ssh-max-port

SSH proxy maximum port. Defaults to `2322`.

### --runtime | -r

Runtime to use for Containerlab operations inside the API server. Options are `docker` (default) or `podman` (WIP).

### --jwt-secret

JWT secret key for authentication. If not provided, a random secret will be generated automatically.

### --jwt-expiration

JWT token expiration time. Defaults to `60m`.

### --user-group

User group for API access. Defaults to `clab_api`.

### --superuser-group

Superuser group name for administrative access. Defaults to `clab_admins`.

### --gin-mode

Gin framework mode. Options: `debug`, `release` (default), `test`.

### --trusted-proxies

Comma-separated list of trusted proxies for the API server.


### --owner | -o

Owner name for the API server container. If not provided, it will be determined from environment variables (SUDO_USER or USER).

### --labs-dir | -l

Directory to mount as shared labs directory where lab files will be stored.

## Examples

Start an API server with default settings:

```bash
❯ containerlab tools api-server start
10:28:35 INFO Generated random JWT secret for API server
10:28:35 INFO Pulling image ghcr.io/srl-labs/clab-api-server/clab-api-server:latest...
10:28:35 INFO Creating API server container clab-api-server
10:28:35 INFO Creating container name=clab-api-server
10:28:36 INFO API server container clab-api-server started successfully.
10:28:36 INFO API Server available at: http://localhost:8080
```

Start with custom port and labs directory:

```bash
❯ containerlab tools api-server start -p 9090 -l /home/user/containerlab/labs
11:40:03 INFO Pulling image ghcr.io/srl-labs/clab-api-server/clab-api-server:latest...
11:40:03 INFO Generated random JWT secret for API server
11:40:03 INFO Creating API server container clab-api-server
11:40:03 INFO API server container clab-api-server started successfully.
11:40:03 INFO API Server available at: http://localhost:9090
```

Start with TLS enabled:

```bash
❯ containerlab tools api-server start --tls-enable --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem
11:40:03 INFO Pulling image ghcr.io/srl-labs/clab-api-server/clab-api-server:latest...
11:40:03 INFO Generated random JWT secret for API server
11:40:03 INFO Creating API server container clab-api-server
11:40:03 INFO API server container clab-api-server started successfully.
11:40:03 INFO API Server available at: http://localhost:8080
11:40:03 INFO API Server TLS enabled at: https://localhost:8080
```

Start with custom name and owner:

```bash
❯ containerlab tools api-server start --name prod-api-server --owner alice
11:40:03 INFO Pulling image ghcr.io/srl-labs/clab-api-server/clab-api-server:latest...
11:40:03 INFO Generated random JWT secret for API server
11:40:03 INFO Creating API server container prod-api-server
11:40:03 INFO API server container prod-api-server started successfully.
11:40:03 INFO API Server available at: http://localhost:8080
```

Start with custom JWT settings and user groups:

```bash
❯ containerlab tools api-server start --jwt-secret my-secret-key --jwt-expiration 120m --user-group developers --superuser-group admins
11:40:03 INFO Pulling image ghcr.io/srl-labs/clab-api-server/clab-api-server:latest...
11:40:03 INFO Creating API server container clab-api-server
11:40:03 INFO API server container clab-api-server started successfully.
11:40:03 INFO API Server available at: http://localhost:8080
```

The API server container is started with host network mode and various bind mounts that provide access to:
- Docker socket for container management
- Network namespaces for network operations
- User and group information for authentication
- Containerlab binary for lab operations

When started successfully, the API server will be available at the specified host and port, ready to handle Containerlab API requests.