# redeploy command

### Description

The `redeploy` command destroys and redeploys a lab based on the topology definition file. It effectively combines the `destroy` and `deploy` commands into a single operation.

### Usage

`containerlab [global-flags] redeploy [local-flags]`

**aliases:** `rdep`

### Flags

#### topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to redeploy a lab.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will fail with an error.

#### cleanup

The local `--cleanup | -c` flag instructs containerlab to remove the lab directory and all its content during the destroy phase.

Without this flag present, containerlab will keep the lab directory and all files inside of it.

#### graceful

To make containerlab attempt a graceful shutdown of the running containers during destroy phase, add the `--graceful` flag. Without it, containers will be removed forcefully without attempting to stop them.

#### graph

The local `--graph | -g` flag instructs containerlab to generate a topology graph after deploying the lab.

#### network

With `--network` flag users can specify a custom name for the management network that containerlab creates for the lab.

#### ipv4-subnet

Using `--ipv4-subnet | -4` flag users can define a custom IPv4 subnet that containerlab will use to assign management IPv4 addresses.

#### ipv6-subnet

Using `--ipv6-subnet | -6` flag users can define a custom IPv6 subnet that containerlab will use to assign management IPv6 addresses.

#### max-workers

With `--max-workers` flag, it is possible to limit the number of concurrent workers that create/delete containers or wire virtual links. By default, the number of workers equals the number of nodes/links to process.

#### keep-mgmt-net

Do not try to remove the management network during destroy phase. Usually the management docker network (in case of docker) and the underlaying bridge are being removed. If you have attached additional resources outside of containerlab and you want the bridge to remain intact just add the `--keep-mgmt-net` flag.

#### reconfigure

The local `--reconfigure` flag instructs containerlab to regenerate configuration artifacts during the deploy phase and overwrite previous ones if any.

#### skip-post-deploy

The `--skip-post-deploy` flag can be used to skip the post-deploy phase of the lab deployment. This is a global flag that affects all nodes in the lab.

#### export-template

The local `--export-template` flag allows a user to specify a custom Go template that will be used for exporting topology data into `topology-data.json` file under the lab directory.

#### node-filter

The local `--node-filter` flag allows users to specify a subset of topology nodes targeted by `redeploy` command. The value of this flag is a comma-separated list of node names as they appear in the topology.

When a subset of nodes is specified, containerlab will only redeploy those nodes and their links and ignore the rest.

#### skip-labdir-acl

The `--skip-labdir-acl` flag can be used to skip the lab directory access control list (ACL) provisioning during the deploy phase.

### Examples

#### Redeploy a lab using the given topology file

```bash
containerlab redeploy -t mylab.clab.yml
```

#### Redeploy a lab and remove the Lab directory

```bash
containerlab redeploy -t mylab.clab.yml --cleanup
```

#### Redeploy a lab with regenerating configuration artifacts

```bash
containerlab redeploy -t mylab.clab.yml --reconfigure
```

#### Redeploy a lab without specifying topology file

Given that a single topology file is present in the current directory.

```bash
containerlab redeploy
```

#### Redeploy a lab using short flag names

```bash
clab rdep -t mylab.clab.yml
```

#### Redeploy specific nodes in a lab

```bash
containerlab redeploy -t mylab.clab.yml --node-filter "node1,node2"
```
