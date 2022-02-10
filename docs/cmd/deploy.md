# deploy command

### Description

The `deploy` command spins up a lab using the topology expressed via [topology definition file](../manual/topo-def-file.md).

### Usage

`containerlab [global-flags] deploy [local-flags]`

**aliases:** `dep`

### Flags

#### topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to spin up a lab.

#### name

With the global `--name | -n` flag a user sets a lab name. This value will override the lab name value passed in the topology definition file.

#### reconfigure

The local `--reconfigure` flag instructs containerlab to first **destroy** the lab and all its directories and then start the deployment process. That will result in a clean (re)deployment where every configuration artefact will be generated (TLS, node config) from scratch.

Without this flag present, containerlab will reuse the available configuration artifacts found in the lab directory.

Refer to the [configuration artifacts](../manual/conf-artifacts.md) page to get more information on the lab directory contents.

#### max-workers
With `--max-workers` flag it is possible to limit the amout of concurrent workers that create containers or wire virtual links. By default the number of workers equals the number of nodes/links to create.

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

#### skip-post-deploy
With `--skip-post-deploy` flag it is possible to skip post deployment actions.

### Examples

```bash
# deploy a lab from mylab.clab.yml file located in the same dir
containerlab deploy -t mylab.clab.yml

# deploy a lab from mylab.clab.yml file and regenerate all configuration artifacts
containerlab deploy -t mylab.clab.yml --reconfigure
```
