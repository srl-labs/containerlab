# redeploy command

### Description

The `redeploy` command redeploys a lab referenced by a provided topology definition file. It effectively combines the `destroy` and `deploy` commands into a single operation.

The two most common applications of this command are:

1. Redeploying a lab while keeping the lab directory intact:

    ```bash
    sudo containerlab redeploy -t mylab.clab.yml
    ```

    This command will destroy the lab and redeploy it using the same topology file and the same lab directory. This should keep intact any saved configurations for the nodes.

2. Redeploying a lab while removing the lab directory at the destroy stage:

    ```bash
    sudo containerlab redeploy --cleanup -t mylab.clab.yml
    ```

    or using the shorthands:

    ```bash
    sudo clab rdep -c -t mylab.clab.yml
    ```

    This command will destroy the lab and remove the lab directory before deploying the lab again. This ensures a clean redeployment as if you were deploying a lab for the first time discarding any previous lab state.

### Usage

`containerlab [global-flags] redeploy [local-flags]`

**aliases:** `rdep`

### Flags

#### topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used to redeploy a lab.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` or `.clab.yaml` extension in that directory and use it as a topology definition file.

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

Do not try to remove the management network during destroy phase. Usually the management docker network (in case of docker) and the underlying bridge are being removed. If you have attached additional resources outside of containerlab and you want the bridge to remain intact just add the `--keep-mgmt-net` flag.

#### skip-post-deploy

The `--skip-post-deploy` flag can be used to skip the post-deploy phase of the lab deployment. This is a global flag that affects all nodes in the lab.

#### export-template

The local `--export-template` flag allows a user to specify a custom Go template that will be used for exporting topology data into `topology-data.json` file under the lab directory.

#### skip-labdir-acl

The `--skip-labdir-acl` flag can be used to skip the lab directory access control list (ACL) provisioning during the deploy phase.

### Examples

#### Redeploy a lab using the given topology file

```bash
containerlab redeploy -t mylab.clab.yml
```

#### Redeploy a lab with removing the Lab directory at destroy stage

```bash
containerlab redeploy --cleanup -t mylab.clab.yml
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
