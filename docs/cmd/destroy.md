# destroy command

### Description

The `destroy` command destroys a lab referenced by its [topology definition file](../manual/topo-def-file.md).

### Usage

`containerlab [global-flags] destroy [local-flags]`

**aliases:** `des`

### Flags

#### topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to identify the lab to destroy.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will fail with an error.

#### cleanup

The local `--cleanup | -c` flag instructs containerlab to remove the lab directory and all its content.

Without this flag present, containerlab will keep the lab directory and all files inside of it.

Refer to the [configuration artifacts](../manual/conf-artifacts.md) page to get more information on the lab directory contents.

#### graceful

To make containerlab attempt a graceful shutdown of the running containers, add the `--graceful` flag to destroy cmd. Without it, containers will be removed forcefully without even attempting to stop them.

#### keep-mgmt-net

Do not try to remove the management network. Usually the management docker network (in case of docker) and the underlying bridge are being removed. If you have attached additional resources outside of containerlab and you want the bridge to remain intact just add the `--keep-mgmt-net` flag.

#### all

Destroy command provided with `--all | -a` flag will perform the deletion of all the labs running on the container host. It will not touch containers launched manually.

#### yes

The `--yes | -y` flag can be used together with `--all` to auto-approve deletion of all labs, skipping the interactive confirmation prompt. This is useful for automation or scripting scenarios where manual confirmation is not desired.

#### node-filter

The local `--node-filter` flag allows users to specify a subset of topology nodes targeted by `destroy` command. The value of this flag is a comma-separated list of node names as they appear in the topology.

When a subset of nodes is specified, containerlab will only destroy those nodes and their links and leave the rest of the topology intact.  
As such, users can destroy a subset of nodes and links in a lab without destroying the entire topology.

Read more about [node filtering](../manual/node-filtering.md) in the documentation.

### Examples

#### Destroy a lab described in the given topology file

```bash
containerlab destroy -t mylab.clab.yml
```

#### Destroy a lab and remove the Lab directory

```bash
containerlab destroy -t mylab.clab.yml --cleanup
```

#### Destroy a lab without specifying topology file

Given that a single topology file is present in the current directory.

```bash
containerlab destroy
```

#### Destroy all labs on the container host

```bash
containerlab destroy -a
```

#### Destroy all labs on the container host without confirmation prompt

```bash
containerlab destroy -a -y
```

#### Destroy a lab using short flag names

```bash
clab des
```
