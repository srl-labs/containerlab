# deploy command

### Description

The `deploy` command spins up a lab using the topology expressed via [topology definition file](../manual/topo-def-file.md).

### Usage

`containerlab [global-flags] deploy [local-flags]`

**aliases:** `dep`

### Flags

#### topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to spin up a lab.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory. If a single file is found, it will be used.

#### name

With the global `--name | -n` flag a user sets a lab name. This value will override the lab name value passed in the topology definition file.

#### reconfigure

The local `--reconfigure | -c` flag instructs containerlab to first **destroy** the lab and all its directories and then start the deployment process. That will result in a clean (re)deployment where every configuration artefact will be generated (TLS, node config) from scratch.

Without this flag present, containerlab will reuse the available configuration artifacts found in the lab directory.

Refer to the [configuration artifacts](../manual/conf-artifacts.md) page to get more information on the lab directory contents.

#### max-workers
With `--max-workers` flag, it is possible to limit the number of concurrent workers that create containers or wire virtual links. By default, the number of workers equals the number of nodes/links to create.

#### runtime
Containerlab nodes can be started by different runtimes, with `docker` being the default one. Besides that, containerlab has experimental support for `podman`, `containerd`, and `ignite` runtimes.

A global runtime can be selected with a global `--runtime | -r` flag that will select a runtime to use. The possible value are:

* `docker` - default
* `podman` - beta support
* `containerd` - experimental support
* `ignite`

#### timeout
A global `--timeout` flag drives the timeout of API requests that containerlab send toward external resources. Currently the only external resource is the container runtime (i.e. docker).

In a busy compute the runtime may respond longer than anticipated, in that case increasing the timeout may help.

The default timeout is set to 2 minutes and can be changed to values like `30s, 10m`.

#### export-template
The local `--export-template` flag allows a user to specify a custom Go template that will be used for exporting topology data into `topology-data.json` file under the lab directory. If not set, the default template path is `/etc/containerlab/templates/export/auto.tmpl`.

To export full topology data instead of a subset of fields exported by default, use `--export-template /etc/containerlab/templates/export/full.tmpl`. Note, some fields exported via `full.tmpl` might contain sensitive information like TLS private keys. To customize export data, it is recommended to start with a copy of `auto.tmpl` and change it according to your needs.

### Environment variables

#### CLAB_VERSION_CHECK

Can be set to "disable" value to prevent deploy command making a network request to check new version to report if one is available.

Useful when running in an automated environments with restricted network access.

Example command-line usage: `CLAB_VERSION_CHECK=disable containerlab deploy`

### Examples

#### Deploy a lab using the given topology file

```bash
❯ containerlab deploy -t mylab.clab.yml
```

#### Deploy a lab and regenerate all configuration artifacts

```bash
❯ containerlab deploy -t mylab.clab.yml --reconfigure
```

#### Deploy a lab without specifying topology file

Given that a single topology file is present in the current directory.

```bash
❯ containerlab deploy
```

#### Deploy a lab using shortcut names
```bash
❯ clab dep -t mylab.clab.yml
```

