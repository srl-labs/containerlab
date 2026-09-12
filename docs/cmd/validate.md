# validate command

### Description

The `validate` command parses a topology definition file and runs the same schema, node, and link checks that the `deploy` or `destroy` commands perform but without deploying or destroying the lab. 

It is useful for catching errors in a topology file before deploying it, for example as a linting step in a CI pipeline.

If the topology is valid, containerlab reports the lab name along with the number of nodes and links and exits with a zero exit code. If the topology is invalid, the offending error is printed and containerlab exits with a non-zero exit code.

### Usage

`containerlab [global-flags] validate [local-flags]`

**aliases:** `val`

### Flags

--8<-- "docs/cmd/deploy.md:env-vars-flags"

#### Topology

With the global `--topo | -t` flag a user sets the path to the topology definition file that will be validated.

When the topology path refers to a directory, containerlab will look for a file with `.clab.yml` or `.clab.yaml` extension in that directory and use it as a topology definition file.

When the topology file flag is omitted, containerlab will try to find the matching file name by looking at the current working directory.

If more than one file is found for directory-based path or when the flag is omitted entirely, containerlab will open an interactive selector to let you pick the topology file from the discovered `clab.yml` or `clab.yaml` files.

### Examples

#### Validate a lab using the given topology file

```bash
containerlab validate -t mylab.clab.yml
```

#### Validate a lab without specifying topology file

Given that a single topology file is present in the current directory.

```bash
containerlab validate
```

#### Validate a lab using short flag names

```bash
clab val -t mylab.clab.yml
```
