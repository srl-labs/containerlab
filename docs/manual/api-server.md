# Containerlab API Server

The `clab-api-server` exposes Containerlab operations over an HTTP API. It is meant for cases where the lab host needs to be controlled by another program, a browser UI, or a remote user session instead of by someone typing `containerlab` commands directly on the host.

Typical uses are:

* running the [Containerlab Desktop GUI](containerlab-gui/desktop.md) or [Containerlab Web GUI](containerlab-gui/web.md)
* integrating Containerlab with automation systems
* giving a UI or service account a controlled way to deploy, inspect, destroy, and operate labs
* managing lab files on a shared lab server

The API server runs on the Linux system that owns the container runtime and the lab files. Clients connect to that API endpoint from the same machine, a browser, a desktop app, or another workstation.

/// note
The API server embeds Containerlab as a Go library. When you install and run `clab-api-server` directly, you do not need to install a separate `containerlab` binary for the API server itself.
///

## Installation

There are several ways to run the API server. For a persistent lab host, the systemd installation is the recommended path.

### Systemd service

Install the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/srl-labs/clab-api-server/main/install.sh | sudo bash -s -- install
```

The installer:

* downloads the API server binary to `/usr/local/bin/clab-api-server`
* creates `/etc/clab-api-server/clab-api-server.env`
* creates the `clab-api-server.service` systemd unit
* creates the default `clab_api` and `clab_admins` groups if they do not exist
* generates a random `JWT_SECRET` for new installations

Edit the environment file before starting the service:

```bash
sudoedit /etc/clab-api-server/clab-api-server.env
```

At minimum, set:

* `API_SERVER_HOST` to the hostname or IP address clients should use
* `CORS_ALLOWED_ORIGINS` if a browser client contacts the API server directly

Add users to the API group, then start the service:

```bash
sudo usermod -aG clab_api <username>
sudo systemctl enable --now clab-api-server
sudo systemctl status clab-api-server
```

For an immediate start with the generated defaults, use `install --start`.

The systemd service runs as `root` because it controls host container runtime resources, network namespaces, Linux users, and lab files.

Upgrade to the latest release:

```bash
sudo clab-api-server version upgrade
```

Install a specific release, including an older release for downgrade:

```bash
curl -fsSL https://raw.githubusercontent.com/srl-labs/clab-api-server/main/install.sh | sudo bash -s -- upgrade --version clab-0.73.0-api-0.2.1
```

Uninstall removes the service and binary while keeping configuration:

```bash
curl -fsSL https://raw.githubusercontent.com/srl-labs/clab-api-server/main/install.sh | sudo bash -s -- uninstall
```

Use `uninstall --purge` only when you also want to remove `/etc/clab-api-server/clab-api-server.env`.

### Containerlab tools command

For quick trials, demos, or temporary API access, Containerlab can start the API server as a container:

```bash
sudo containerlab tools api-server start \
  --labs-dir /opt/containerlab/labs
```

Use the matching commands to inspect or remove the API server container:

```bash
sudo containerlab tools api-server status
sudo containerlab tools api-server stop
```

See the command reference for the complete flag list:

* [`tools api-server start`](../cmd/tools/api-server/start.md)
* [`tools api-server status`](../cmd/tools/api-server/status.md)
* [`tools api-server stop`](../cmd/tools/api-server/stop.md)

/// warning
The standalone API server and the `containerlab tools api-server start` helper both default to port `8090` and HTTPS.
///

### Direct binary

Download and run a release binary directly when you want to test without installing the service:

```bash
curl -fsSL https://raw.githubusercontent.com/srl-labs/clab-api-server/main/install.sh | sudo bash -s -- pull-only
sudo clab-api-server -env-file /path/to/clab-api-server.env
```

Configuration is read from environment variables or a `.env` file in the current directory.

### Container image

The API server can also run as a container. This is useful when you want to manage it with your own container tooling:

```bash
docker run -d \
  --name clab-api-server \
  --privileged \
  --network host \
  --pid host \
  -e LOG_LEVEL=debug \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/run/netns:/var/run/netns \
  -v /var/lib/docker/containers:/var/lib/docker/containers \
  -v /etc/passwd:/etc/passwd:ro \
  -v /etc/shadow:/etc/shadow:ro \
  -v /etc/group:/etc/group:ro \
  -v /etc/gshadow:/etc/gshadow:ro \
  -v /home:/home \
  ghcr.io/srl-labs/clab-api-server/clab-api-server:latest
```

The bind mounts give the API server access to the host container runtime, network namespaces, Linux user database, and user lab files.

## Users and access

The API server authenticates Linux users from the host it runs on. A user must:

* exist on the API server host
* belong to `API_USER_GROUP`, `clab_api` by default, or `SUPERUSER_GROUP`, `clab_admins` by default
* log in with the Linux username and password

Create the default groups and add users if they do not exist yet:

```bash
sudo groupadd -f clab_api
sudo groupadd -f clab_admins
sudo usermod -aG clab_api <username>
```

Successful login returns a JWT token that clients use for API requests.

Superusers can see and operate all labs. Regular API users operate within their user context and lab ownership.

## Configuration

These settings are the ones most users need to look at first:

| Variable | Default | Purpose |
| -------- | ------- | ------- |
| `API_PORT` | `8090` | API server listen port |
| `API_SERVER_HOST` | `localhost` | Hostname or IP address shown to clients for API and SSH access |
| `JWT_SECRET` | generated by installer | Secret used to sign login tokens |
| `JWT_EXPIRATION` | `24h` | Token lifetime, for example `24h` or `7d` |
| `API_USER_GROUP` | `clab_api` | Linux group allowed to use the API |
| `SUPERUSER_GROUP` | `clab_admins` | Linux group with elevated API access |
| `CLAB_RUNTIME` | `docker` | Container runtime used for labs |
| `CORS_ALLOWED_ORIGINS` | unset | Browser origins allowed to call the API directly |
| `TLS_ENABLE` | `true` | Serve the API over HTTPS |
| `TLS_AUTO_CERT` | `true` | Generate a local self-signed certificate if cert files are unset |

The full configuration reference is maintained in the [`clab-api-server` repository](https://github.com/srl-labs/clab-api-server).

## API docs

When the server is running, the interactive API documentation is available from the API server itself:

```text
https://<server>:<port>/swagger/index.html
https://<server>:<port>/redoc
```

The published API reference is available at:

```text
https://srl-labs.github.io/clab-api-server/
```

## API example

Log in with a Linux user accepted by the API server:

```bash
TOKEN=$(curl -sk -X POST https://localhost:8090/login \
  -H "Content-Type: application/json" \
  -d '{"username":"your_linux_username","password":"your_linux_password"}' \
  | jq -r '.token')
```

List labs:

```bash
curl -k -H "Authorization: Bearer $TOKEN" https://localhost:8090/api/v1/labs
```

## Using it with Containerlab GUI

The [Containerlab Desktop GUI](containerlab-gui/desktop.md) and [Containerlab Web GUI](containerlab-gui/web.md) connect to a reachable API server URL and log in with a Linux user accepted by the API server. For regular use, run `clab-api-server` as a service on the lab host, then point the GUI at that URL.

On macOS and Windows, the API server still needs to run in the Linux environment that owns Docker and the lab files, such as a remote lab host, a Linux VM, a devcontainer, Docker Desktop VM, or OrbStack VM. The GUI can run on the workstation and connect to that API endpoint.
