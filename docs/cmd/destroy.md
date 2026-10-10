# destroy command

### Description

The `destroy` command destroys a lab referenced by its [topology definition file](../manual/topo-def-file.md).

--8<-- "docs/cmd/deploy.md:env-vars-flags"

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

#### keep-links

The local `--keep-links` flag preserves the data-plane interfaces of nodes selected with
`--node-filter`. Containerlab parks the interfaces in persistent network namespaces before it
removes the containers. A subsequent `deploy` or `apply` restores the interfaces when it creates
the missing nodes, preserving the existing veth pairs and their peer interfaces.

This flag is intended for replacing selected nodes and requires `--node-filter`. It cannot be
combined with `--cleanup`.

See [replacing nodes while preserving links](#replacing-nodes-while-preserving-links) for a
complete workflow, interface renaming examples, and recovery guidance.


#### all

Destroy command provided with `--all | -a` flag will perform the deletion of all the labs running on the container host. It will not touch containers launched manually.

#### yes

The `--yes | -y` flag can be used together with `--all` to auto-approve deletion of all labs, skipping the interactive confirmation prompt. This is useful for automation or scripting scenarios where manual confirmation is not desired.

#### node-filter

The local `--node-filter` flag allows users to specify a subset of topology nodes targeted by `destroy` command. The value of this flag is a comma-separated list of node names as they appear in the topology.

When a subset of nodes is specified, containerlab will only destroy those nodes and their links and leave the rest of the topology intact.  
As such, users can destroy a subset of nodes and links in a lab without destroying the entire topology.

Add [`--keep-links`](#keep-links) to preserve the selected nodes' data-plane interfaces for
replacement instead of removing their links.

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

#### Destroy selected nodes while preserving their links for replacement

```bash
containerlab destroy -t mylab.clab.yml --node-filter node1,node2 --keep-links --keep-mgmt-net
containerlab apply -t replacement.clab.yml
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

## Replacing nodes while preserving links

Use `destroy --node-filter ... --keep-links` when you want to remove selected containers and
later reconnect their replacements to the same Linux veth pairs. This is useful when replacing
an image or changing a node kind while keeping the interfaces on neighboring nodes in place.

Preserving a link preserves its kernel interfaces, not service continuity. Traffic through a
removed node is interrupted until its replacement starts and its network OS is ready. The
replacement container has a new lifecycle; this operation does not save the removed node's
running configuration, processes, or protocol sessions. Save any configuration you need before
destroying the node.

### What happens to the links

1. Containerlab discovers the selected nodes' tracked and owned runtime data-plane interfaces.
2. It moves those interfaces into named parking network namespaces before deleting the
   containers. A peer on an unselected node stays in that node's namespace.
3. `deploy` or its alias `apply` discovers the parked interfaces and creates the missing nodes.
4. Containerlab restores the interfaces into the replacements, renames paired veth endpoints
   when required by the new topology, brings them up in the default link mode, and removes
   the parking namespaces after successful restoration.

Both ends of a veth can be parked when both nodes are selected. Restoration can locate a peer
in its parking namespace as well as in a running node. The interface name used by the removed
node does not determine the identity of a preserved pair: containerlab matches the veths by
peer identity. Renames are staged through temporary names so that swaps or overlapping old
and new names do not collide.

### Replace a single node

Keep the lab name and node name unchanged so that the replacement can find the node's parking
namespace. Use the existing topology to remove the old node:

```bash
containerlab destroy -t lab.clab.yml --node-filter node1 --keep-links --keep-mgmt-net
```

Update the image or other supported node properties in the topology, then preview and apply:

```bash
containerlab apply -t lab.clab.yml --dry-run
containerlab apply -t lab.clab.yml
```

`apply` and `deploy` accept the same flags. No restoration flag is needed on either command.
Use ordinary reconciliation for restoration; `--reconfigure` explicitly destroys and recreates
the lab instead.

`--keep-mgmt-net` is separate from `--keep-links`: the former retains the management network,
while the latter parks data-plane interfaces. Include `--keep-mgmt-net` if the management
network must remain available between removal and replacement.

### Change the replacement's interface names

A replacement kind can require different interface names. For example, the original link may
be defined as:

```yaml
name: replacement

topology:
  nodes:
    node1:
      kind: linux
      image: alpine:3
    node2:
      kind: linux
      image: alpine:3
  links:
    - endpoints: ["node1:eth1", "node2:eth1"]
```

After removing `node1` with `--keep-links`, change its endpoint name in the requested topology:

```yaml
  links:
    - endpoints: ["node1:eth4", "node2:eth1"]
```

Apply the updated topology. Containerlab identifies the parked endpoint by its surviving
`node2:eth1` peer and restores it as `node1:eth4`. The veth pair remains the same, and the
interface on `node2` keeps its name. When changing kinds, also update the node's `kind`,
`image`, and configuration to values supported by that kind; this Linux example only
illustrates the endpoint mapping.

You can also park both nodes and rename both ends before applying:

```bash
containerlab destroy -t lab.clab.yml --node-filter node1,node2 --keep-links --keep-mgmt-net
```

Keep the same node identities and the same peer relationships. A matching interface name
alone is insufficient to identify a preserved link, particularly when several interfaces are
renamed or their names are swapped.

### Requirements and limits

- `--keep-links` requires a nonempty `--node-filter`. It cannot be combined with `--cleanup`.
- Retain the lab directory and its state and configuration artifacts during replacement.
- Keep the lab and selected node identities stable. Changing a topology file's path is
  supported, but changing the lab or node name changes how the parking namespace is located.
- The peer matching and rename behavior described here applies to veth pairs. This feature
  does not promise equivalent preservation for every link type supported by deployment.
- Runtime management interfaces and arbitrary unowned interfaces are not a substitute for
  containerlab's tracked or owned data-plane endpoints.
- Existing link parameter or type changes are subject to the normal
  [reconciliation limitations](deploy.md#reconciliation-limitations). Parking is not a way to
  apply unsupported link changes in place.
- Parking namespaces are host resources for the replacement workflow, not a backup that
  survives a host restart.

If the topology path recorded on a container no longer exists, a filtered destroy can fall
back to the topology file supplied with `-t`. Ensure that file still identifies the intended
lab and nodes.

### Troubleshooting restoration

A stale runtime discovery result can still mention a container removed by a filtered destroy.
When the container is absent and its parking namespace exists, apply treats it as a missing,
already parked node and restores its interfaces.

If apply reports `cannot restore preserved link` and `refusing to create a replacement veth`,
the discovered interfaces do not form the pair requested by the topology. Containerlab rejects
that plan rather than silently replacing the preserved pair. The error includes each desired
endpoint, whether its candidates are live or parked, and the discovered interface names,
interface indexes (`idx`), peer indexes (`peer`), and bridge master where available. `none`
means no matching candidate was discovered.

Check the lab and node names, the requested peer relationships, and whether a peer interface
was removed outside containerlab. Restore the intended topology and retry if the preserved
pair still exists. If you intend to change the wiring rather than restore it, use the normal
lab destruction and deployment workflow after saving any configuration you need.

For detailed discovery and restoration logs, use the global debug flag:

```bash
containerlab -d apply -t lab.clab.yml
```

Restoration also resets an interface's dormant link mode and brings it administratively up.
If the NOS still cannot use the link, check its interface configuration and readiness; an
administratively up Linux interface does not establish a routing adjacency by itself.
