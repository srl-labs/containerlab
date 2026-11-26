# schema command

### Description

The `schema` command prints the [JSON schema](../manual/topo-def-file.md) of the topology definition file to stdout. The schema is the one of the installed containerlab version, so it describes exactly the topology options this version accepts.

Code editors and other tools can use it to validate topology files and to complete their keys, without fetching the schema of another version from the repository.

Go programs that use containerlab as a library get the same schema with `schemas.Topology()` from the `github.com/srl-labs/containerlab/schemas` package.

### Usage

`containerlab [global-flags] schema`

### Examples

#### Save the schema to a file

```bash
containerlab schema > clab.schema.json
```

#### Use the saved schema in a topology file

With the [YAML language server](https://github.com/redhat-developer/yaml-language-server), a comment at the top of the topology file points the editor to the schema:

```yaml
# yaml-language-server: $schema=./clab.schema.json
name: mylab
```
