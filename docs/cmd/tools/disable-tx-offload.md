# disable-tx-offload command

### Description

The `disable-tx-offload` command under the `tools` command disables tx checksum offload for `eth0` interface of a container referenced by its name.

The need for `disable-tx-offload` might arise when you launch a container outside of containerlab or restart a container. Some nodes, like SR Linux, will require correct checksums in TCP packets; thus, it is needed to disable checksum offload on those containers to do checksum calculations instead of offloading it.

### Usage

`containerlab tools disable-tx-offload [local-flags]`

### Flags

#### container
With the local mandatory `--container | -c` flag, a user specifies which container to remove tx offload.

### Examples

```bash
# disable tx checksum offload on gnmic container
❯ clab tools disable-tx-offload -c clab-st-gnmic
INFO[0000] getting container 'clab-st-gnmic' information 
INFO[0000] Tx checksum offload disabled for eth0 interface of clab-st-gnmic container 
```