# gotty attach

## Description

The `attach` sub-command under the `tools gotty` command creates and starts a container that runs the [GoTTY](https://github.com/srl-labs/gotty-service) web terminal. GoTTY provides a browser based terminal which can be used to access your lab nodes via SSH.

## Usage

```
containerlab tools gotty attach [flags]
```

## Flags

### --lab | -l

Name of the lab to attach the GoTTY container to.

### --topology | -t

Path to the topology file (`*.clab.yml`) that defines the lab. This flag can be used instead of `--lab`.

### --name

Name of the GoTTY container. If omitted it defaults to `clab-<labname>-gotty`.

### --port | -p

Port for the GoTTY web interface. Default is `8080`.

### --username | -u

Username used to authenticate to the GoTTY web terminal. Defaults to `admin`.

### --password | -P

Password used to authenticate to the GoTTY web terminal. Defaults to `admin`.

### --shell | -s

Shell to start inside the container. Defaults to `bash`.

### --image | -i

Container image used to run GoTTY. Defaults to `ghcr.io/srl-labs/network-multitool`.

### --owner | -o

Owner name to associate with the GoTTY container. If not provided it will be discovered automatically from environment variables.

## Examples

Attach a GoTTY container to a running lab:

```bash
❯ containerlab tools gotty attach -l mylab
11:40:03 INFO Pulling image ghcr.io/srl-labs/network-multitool...
11:40:03 INFO Creating GoTTY container clab-mylab-gotty on network 'clab-mylab'
11:40:04 INFO GoTTY container clab-mylab-gotty started. Waiting for GoTTY service to initialize...
11:40:09 INFO GoTTY web terminal successfully started url=http://HOST_IP:8080 username=admin password=admin
```

Once started, open the printed URL in a browser to access the terminal. From there you can connect to lab nodes using SSH for example:

```
ssh admin@clab-mylab-node1
```

