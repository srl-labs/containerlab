# vxlan delete

### Description

The `delete` sub-command under the `tools vxlan` command deletes VxLAN interfaces which name matches a user specified prefix or an exact name. The `delete` command is typically used to remove VxLAN interfaces created with [`create`](create.md) command.

Either `--prefix` or `--name` must be provided. The two flags are mutually exclusive.

### Usage

`containerlab tools vxlan delete [local-flags]`

### Flags

#### prefix
Set a prefix with `--prefix | -p` flag. The VxLAN interfaces which name is matched by the prefix will be deleted. Default prefix is `vx-` which is matched the default prefix used by [`create`](create.md) command.

#### name
Set an exact interface name with `--name | -n` flag. Exactly one VxLAN interface with the given name will be deleted. If no interface with that name exists, or if an interface with that name exists but is not a VxLAN, the command returns an error.

### Examples

```bash
# delete all VxLAN interfaces created by containerlab
❯ clab tools vxlan create delete
INFO[0000] Deleting VxLAN link vx-srl_e1-1

# delete a single VxLAN interface by its exact name
❯ clab tools vxlan delete --name vx-srl-e1-1
INFO[0000] Deleting VxLAN link vx-srl-e1-1
```