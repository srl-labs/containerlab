# deploy command

### Description

The `deploy` command spins up a lab using the topology expressed via [topology definition file](../manual/topo-def-file.md).

### Usage

`containerlab [global-flags] deploy [local-flags]`

**aliases:** `dep`

### Flags

#### topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to spin up a lab.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` or `.clab.yaml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will fail with an error.

It is possible to read the topology file from stdin by passing `-` as a value to the `--topo` flag. See [examples](#deploy-a-lab-from-a-remote-url-with-curl) for more details.

##### Remote topology files

###### Git

To simplify the deployment of labs that are stored in remote version control systems, containerlab supports the use of remote topology files for GitHub.com and GitLab.com hosted projects.

By specifying a URL to a repository or a `.clab.yml` file in a repository, containerlab will automatically clone[^1] the repository in your current directory and deploy it. If the URL points to a `.clab.yml` file, containerlab will clone the repository and deploy the lab defined in the file.

The following URL formats are supported:

| Type                                  | Example                                                             | Which topology file is used                                                        |
| ------------------------------------- | ------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Link to github repository             | https://github.com/hellt/clab-test-repo/                            | An auto-find procedure will find a `clab.yml` in the repository root and deploy it |
| Link to a file in a github repository | https://github.com/hellt/clab-test-repo/blob/main/lab1.clab.yml     | A file specified in the URL will be deployed                                       |
| Link to a repo's branch               | https://github.com/hellt/clab-test-repo/tree/branch1                | A branch of a repo is cloned and auto-find procedure kicks in                      |
| Link to a file in a branch of a repo  | https://github.com/hellt/clab-test-repo/blob/branch1/lab2.clab.yml  | A branch is cloned and a file specified in the URL is used for deployment          |
| Link to a file in a subdir of a repo  | https://github.com/hellt/clab-test-repo/blob/main/dir/lab3.clab.yml | A file specified in the subdir of the branch will be deployed                      |
| Shortcut of a github project          | hellt/clab-test-repo                                                | An auto-find procedure will find a `clab.yml` in the repository root and deploy it |

When the lab is deployed using the URL, the repository is cloned in the current working directory. If the repository is already cloned it will be used and not cloned again; containerlab will try to fetch the latest changes from the remote repository.

Subsequent lab operations (such as destroy) must use the filesystem path to the topology file and not the URL.

???note "Remote labs workflow in action"
    <div class="iframe-container">
        <iframe width="100%" src="https://www.youtube.com/embed/0QlUZsJGQDo" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>
    </div>

###### HTTP(S)

Labs can be deployed from remote HTTP(S) URLs as well. These labs should be self-contained and not reference any external resources, like startup-config files, licenses, binds, etc.

The following URL formats are supported:

| Type                           | Example                                                             | Description                                           |
| ------------------------------ | ------------------------------------------------------------------- | ----------------------------------------------------- |
| Link to raw github gist        | https://gist.githubusercontent.com/hellt/abc/raw/def/linux.clab.yml | A file is downloaded to a temp directory and launched |
| Link to a short schemaless URL | srlinux.dev/clab-srl                                                | A file is downloaded to a temp directory and launched |

Containerlab distinct HTTP URLs from GitHub/GitLab by checking if github.com or gitlab.com is present in the URL. If not, it will treat the URL as a plain HTTP(S) URL.

###### S3

Containerlab supports using S3 URLs to retrieve topology files and startup configurations for network devices. Check out the documentation on [S3 usage](../manual/s3-usage-example.md) for more details.

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

The local `--export-template` flag allows a user to specify a custom Go template that will be used for exporting topology data into `topology-data.json` file under the lab directory. If not set, the [default template](https://github.com/srl-labs/containerlab/blob/main/clab/export_templates/auto.tmpl) is used.

To export the full topology data instead of a subset of fields exported by default, use `--export-template __full` which is a special value that instructs containerlab to use the [full.tmpl](https://github.com/srl-labs/containerlab/blob/main/clab/export_templates/full.tmpl) template file. Note, some fields exported via `full.tmpl` might contain sensitive information like TLS private keys. To customize export data, it is recommended to start with a copy of `auto.tmpl` and change it according to your needs.

#### log-level

Global `--log-level` parameter can be used to configure logging verbosity of all containerlab operations.
`--debug | -d` option is a shorthand for `--log-level debug` and takes priority over `--log-level` if specified.

Following values are accepted, ordered from most verbose to least: `trace`, `debug`, `info`, `warning`, `error`, `fatal`. Default level is `info`.

It should be useful to enable more verbose logging when something doesn't work as expected, to better understand what's going on, and to provide more useful output logs when reporting containerlab issues, while making it more terse in production environments.

#### node-filter

The local `--node-filter` flag allows users to specify a subset of topology nodes targeted by `deploy` command. The value of this flag is a comma-separated list of node names as they appear in the topology.

When a subset of nodes is specified, containerlab will only deploy those nodes and links belonging to all selected nodes and ignore the rest. This can be useful e.g. in CI/CD test case scenarios, where resource constraints may prohibit the deployment of a full topology.

Read more about [node filtering](../manual/node-filtering.md) in the documentation.

#### skip-post-deploy

The `--skip-post-deploy` flag can be used to skip the post-deploy phase of the lab deployment. This is a global flag that affects all nodes in the lab.

#### skip-labdir-acl

The `--skip-labdir-acl` flag can be used to skip the lab directory access control list (ACL) provisioning.

The extended File ACLs are provisioned for the lab directory by default, unless this flag is set. Extended File ACLs allow a sudo user to access the files in the lab directory that might be created by the `root` user from within the container node.

While this is useful in most cases, sometimes extended File ACLs might prevent your lab from working, especially when your lab directory end up being mounted from the network filesystem (NFS, CIFS, etc.). In such cases, you can use this flag to skip the ACL provisioning.

#### owner

The local `--owner` flag allows you to specify a custom owner for the lab. This value will be applied as the owner label for all nodes in the lab.

This flag is designed for multi-user environments where you need to track ownership of lab resources. Only users who are members of the `clab_admins` group can set a custom owner. If a non-admin user attempts to set an owner, the flag will be ignored with a warning, and the current user will be used as the owner instead.

Example:

```bash
containerlab deploy -t mylab.clab.yml --owner alice
```

### Environment variables

#### `CLAB_RUNTIME`

Default value of "runtime" key for nodes, same as global `--runtime | -r` flag described above.
Affects all containerlab commands in the same way, not just `deploy`.

Intended to be set in environments where non-default container runtime should be used, to avoid needing to specify it for every command invocation or in every configuration file.

Example command-line usage: `CLAB_RUNTIME=podman containerlab deploy`

#### `CLAB_VERSION_CHECK`

Can be set to "disable" value to prevent deploy command making a network request to check new version to report if one is available.

Useful when running in an automated environments with restricted network access.

Example command-line usage: `CLAB_VERSION_CHECK=disable containerlab deploy`

#### `CLAB_LABDIR_BASE`

To change the [lab directory](../manual/conf-artifacts.md#identifying-a-lab-directory) location, set `CLAB_LABDIR_BASE` environment variable accordingly. It denotes the base directory in which the lab directory will be created.

The default behavior is to create the lab directory in the same directory as the topology file (`clab.yml` file).

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

#### Deploy a lab from a remote URL with curl

```bash
curl -s https://gist.githubusercontent.com/hellt/9baa28d7e3cb8290ade1e1be38a8d12b/raw/03067e242d44c9bbe38afa81131e46bab1fa0c42/test.clab.yml | \
    sudo containerlab deploy -t -
```

[^1]: The repository is cloned with `--depth 1` parameter.
