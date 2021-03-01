# vxlan delete

### Description

The `delete` sub-command under the `tools vxlan` command deletes VxLAN interfaces which name matches a user specified prefix. The `delete` command is typically used to remove VxLAN interfaces created with [`create`](create.md) command.

### Usage

`containerlab tools vxlan delete [local-flags]`

### Flags

#### prefix
Set a prefix with `--prefix | -p` flag. The VxLAN interfaces which name is matched by the prefix will be deleted. Default prefix is `vx-` which is matched the default prefix used by [`create`](create.md) command.

### Examples

```bash
# delete all VxLAN interfaces created by containerlab
❯ clab tools vxlan create delete
INFO[0000] Deleting VxLAN link vx-srl_e1-1
```