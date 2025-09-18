# code-server start

## Description

The `start` sub-command under the `tools code-server` command launches a dedicated [code-server](https://github.com/coder/code-server) container that is pre-configured with the VSCode Containerlab extension. The container exposes a VS Code compatible web UI in your browser and mounts both the host lab directory and user home so you can browse, edit, and run labs.

On first start the command creates persistent directories under `~/.clab/code-server/<name>/` for configuration, extensions, and user data. It also seeds the extensions directory with the pre-baked extensions that ship in the container image and writes a default configuration (`password: clab`).

## Usage

```
containerlab tools code-server start [flags]
```

## Flags

### --image | -i

Container image to use for the code-server instance. Defaults to `ghcr.io/kaelemc/clab-code-server:main`.

### --name | -n

Container name to create. Defaults to `clab-code-server`.

### --labs-dir | -l

Host directory that will be mounted inside the container at `/labs`. Defaults to `~/.clab` when not provided.

### --port | -p

Host TCP port that will be forwarded to the container's port `8080`. Defaults to `0`, which lets the container runtime pick a random available port.

### --owner | -o

Label value stored on the container to record the creator/owner. If omitted, Containerlab derives the value from `SUDO_USER` or `USER`.

## Examples

Start with default settings and let Docker assign a random host port:

```bash
❯ containerlab tools code-server start
16:41:50 INFO Pulling image ghcr.io/kaelemc/clab-code-server:main...
16:41:50 INFO Pulling image image=ghcr.io/kaelemc/clab-code-server:main
main: Pulling from kaelemc/clab-code-server
Digest: sha256:5d3b80127db6f74b556f1df1ad8c339f8bbd9694616e8325ea7e9b9fe6065fe9
Status: Image is up to date for ghcr.io/kaelemc/clab-code-server:main
16:41:50 INFO Done pulling image image=ghcr.io/kaelemc/clab-code-server:main
16:41:50 INFO Creating code server container name=clab-code-server
16:41:50 INFO Creating container name=clab-code-server
16:41:50 INFO code-server container clab-code-server started successfully.
16:41:50 INFO code-server available at: http://0.0.0.0:32779
```

Expose the service on a specific host port with a custom labs directory:

```bash
❯ containerlab tools code-server start --port 10080 --labs-dir /srv/containerlab/labs
...[snip]...
INFO code-server container clab-code-server started successfully.
INFO code-server available at: http://0.0.0.0:10080
```

After the container starts you can browse to the reported URL and log in with username `clab` / password `clab`.
