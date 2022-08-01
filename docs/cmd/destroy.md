# destroy command

### Description

The `destroy` command destroys a lab referenced by its [topology definition file](../manual/topo-def-file.md).

### Usage

`containerlab [global-flags] destroy [local-flags]`

**aliases:** `des`

### Flags

#### topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be used get the elements of a lab that will be destroyed.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory. If a single file is found, it will be used.

#### cleanup

The local `--cleanup | -c` flag instructs containerlab to remove the lab directory and all its content.

Without this flag present, containerlab will keep the lab directory and all files inside of it.

Refer to the [configuration artifacts](../manual/conf-artifacts.md) page to get more information on the lab directory contents.

#### graceful
To make containerlab attempt a graceful shutdown of the running containers, add the `--graceful` flag to destroy cmd. Without it, containers will be removed forcefully without even attempting to stop them.

#### keep-mgmt-net
Do not try to remove the management network. Usually the management docker network (in case of docker) and the underlaying bridge are being removed. If you have attached additional resources outside of containerlab and you want the bridge to remain intact just add the `--keep-mgmt-net` flag.

#### all
Destroy command provided with `--all | -a` flag will perform the deletion of all the labs running on the container host. It will not touch containers launched manually.

### Examples

```bash
# destroy a lab based on mylab.clab.yml topology file located in the same dir
containerlab destroy -t mylab.clab.yml

# destroy a lab and also remove the Lab Directory
containerlab destroy -t mylab.clab.yml --cleanup

# destroy all labs deployed with containerlab
# using shortcut names
clab des -a
```