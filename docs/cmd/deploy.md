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

The local `--reconfigure` flag instructs containerlab to remove the lab directory and all its content (if such directory exists) and start the deploy process after that. That will result in a clean deployment where every configuration artefact will be generated (TLS, node config) from scratch.

Without this flag present, containerlab will reuse the available configuration artifacts found in the lab directory.

Refer to the [configuration artifacts](../manual/conf-artifacts.md) page to get more information on the lab directory contents.

### Examples

```bash
# deploy a lab from mylab.yml file located in the same dir
containerlab deploy -t mylab.yml

# deploy a lab from mylab.yml file and regenerate all configuration artifacts
containerlab deploy -t mylab.yml --reconfigure
```