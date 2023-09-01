# deploy command

### Description

The `deploy` command spins up a lab using the topology expressed via [topology definition file](../manual/topo-def-file.md).

### Usage

`containerlab [global-flags] deploy [local-flags]`

**aliases:** `dep`

### Flags

#### topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to spin up a lab.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will fail with an error.

#### name

With the global `--name | -n` flag a user sets a lab name. This value will override the lab name value passed in the topology definition file.

#### vars

Global `--vars` option for using specified json or yaml file to load template variables from for generating topology file.

Default is to lookup files with "_vars" suffix and common json/yaml file extensions next to topology file.
For example, for `mylab.clab.gotmpl` template of topology definition file, variables from `mylab.clab_vars.yaml` file will be used by default, if it exists, or one with `.json` or `.yml` extension.

See documentation on [Generated topologies](../manual/topo-def-file.md#generated-topologies) for more information and examples on how to use these variables.

#### reconfigure

The local `--reconfigure | -c` flag instructs containerlab to first **destroy** the lab and all its directories and then start the deployment process. That will result in a clean (re)deployment where every configuration artefact will be generated (TLS, node config) from scratch.

Without this flag present, containerlab will reuse the available configuration artifacts found in the lab directory.

Refer to the [configuration artifacts](../manual/conf-artifacts.md) page to get more information on the lab directory contents.

#### max-workers

With `--max-workers` flag, it is possible to limit the number of concurrent workers that create containers or wire virtual links. By default, the number of workers equals the number of nodes/links to create.

#### runtime

Containerlab nodes can be started by different runtimes, with `docker` being the default one. Besides that, containerlab has experimental support for `podman`, and `ignite` runtimes.

A global runtime can be selected with a global `--runtime | -r` flag that will select a runtime to use. The possible value are:

* `docker` - default
* `podman` - experimental support
* `ignite`

#### timeout

A global `--timeout` flag drives the timeout of API requests that containerlab send toward external resources. Currently the only external resource is the container runtime (i.e. docker).

In a busy compute the runtime may respond longer than anticipated, in that case increasing the timeout may help.

The default timeout is set to 2 minutes and can be changed to values like `30s, 10m`.

#### export-template

The local `--export-template` flag allows a user to specify a custom Go template that will be used for exporting topology data into `topology-data.json` file under the lab directory. If not set, the default template path is `/etc/containerlab/templates/export/auto.tmpl`.

To export full topology data instead of a subset of fields exported by default, use `--export-template /etc/containerlab/templates/export/full.tmpl`. Note, some fields exported via `full.tmpl` might contain sensitive information like TLS private keys. To customize export data, it is recommended to start with a copy of `auto.tmpl` and change it according to your needs.

#### log-level

Global `--log-level` parameter can be used to configure logging verbosity of all containerlab operations.
`--debug | -d` option is a shorthand for `--log-level debug` and takes priority over `--log-level` if specified.

Following values are accepted, ordered from most verbose to least: `trace`, `debug`, `info`, `warning`, `error`, `fatal`. Default level is `info`.

It should be useful to enable more verbose logging when something doesn't work as expected, to better understand what's going on, and to provide more useful output logs when reporting containerlab issues, while making it more terse in production environments.

#### node-filter

The local `--node-filter` flag allows users to specify a subset of topology nodes targeted by `deploy` command. The value of this flag is a comma-separated list of node names as they appear in the topology.

When a subset of nodes is specified, containerlab will only deploy those nodes and links belonging to all selected nodes and ignore the rest. This can be useful e.g. in CI/CD test case scenarios, where resource constraints may prohibit the deployment of a full topology.

Read more about [node filtering](../manual/node-filtering.md) in the documentation.

### Environment variables

#### CLAB_RUNTIME

Default value of "runtime" key for nodes, same as global `--runtime | -r` flag described above.
Affects all containerlab commands in the same way, not just `deploy`.

Intended to be set in environments where non-default container runtime should be used, to avoid needing to specify it for every command invocation or in every configuration file.

Example command-line usage: `CLAB_RUNTIME=podman containerlab deploy`

#### CLAB_VERSION_CHECK

Can be set to "disable" value to prevent deploy command making a network request to check new version to report if one is available.

Useful when running in an automated environments with restricted network access.

Example command-line usage: `CLAB_VERSION_CHECK=disable containerlab deploy`

#### CLAB_LABDIR_BASE

To change the [lab directory](../manual/conf-artifacts.md#identifying-a-lab-directory) location, set `CLAB_LABDIR_BASE` environment variable accordingly. It denotes the base directory in which the lab directory will be created.

The default behavior is to create the lab directory in the current working dir.

### Examples

#### Deploy a lab using the given topology file

```bash
containerlab deploy -t mylab.clab.yml
```

#### Deploy a lab and regenerate all configuration artifacts

```bash
containerlab deploy -t mylab.clab.yml --reconfigure
```

#### Deploy a lab without specifying topology file

Given that a single topology file is present in the current directory.

```bash
containerlab deploy
```

#### Deploy a lab using short flag names

```bash
clab dep -t mylab.clab.yml
```
