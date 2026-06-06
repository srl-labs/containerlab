# apply command

### Description

The `apply` command makes the runtime match a topology definition file. If the lab is not
deployed yet, `apply` deploys it. If the lab is already deployed, `apply` discovers the current
state from the container runtime and applies supported topology deltas without destroying and
redeploying the whole lab.

The first implementation focuses on topology shape changes:

- add nodes
- delete nodes
- add links
- delete links

Apply also tracks a small set of existing node definition changes from the last saved apply state.
Some changes, such as `exec`, restart the existing node; changes that affect the container object,
such as image, type, environment, binds, ports, resources, runtime, or components, recreate the node.
Use `redeploy` or `deploy --reconfigure` when unsupported node properties, startup configuration,
or generated configuration artifacts need to change.

When existing nodes need their dataplane adjusted, apply uses the same endpoint parking
mechanism as `stop`, `start`, and `restart`: affected nodes are stopped, their dataplane
interfaces are parked in a temporary network namespace, and the interfaces are restored after the
node starts again.

--8<-- "docs/cmd/deploy.md:env-vars-flags"

### Usage

`containerlab [global-flags] apply [local-flags]`

### Flags

#### topology | name

Use the global `--topo | -t` flag to reference the desired topology file, or use the global
`--name` flag to reference an already deployed lab by name.

When the lab does not exist yet, `--topo` is required because there is no runtime state from which
containerlab can derive the original topology path.

When `--name` is used, containerlab tries to derive the topology file from the labels on the
deployed containers. If the original topology file is no longer available, provide `--topo`
explicitly.

#### dry-run

The local `--dry-run` flag prints the planned apply actions without applying them.

#### max-workers

With `--max-workers` flag, it is possible to limit the number of concurrent workers that create
new nodes.

#### skip-post-deploy

The `--skip-post-deploy` flag skips the post-deploy phase for nodes added by apply.

#### export-template

The local `--export-template` flag allows a user to specify a custom Go template that will be used
for exporting topology data into `topology-data.json` file under the lab directory after apply
finishes.

### Limitations

Apply currently supports only a subset of topology changes:

- supported link types are `veth`, brief links, `host`, `mgmt-net`, `macvlan`, `vxlan`,
  `vxlan-stitch`, `dummy`, and `bridge`
- distributed nodes, such as SR-SIM with components, are supported for node and link add/delete
- root-namespace-based nodes are not supported
- nodes with `auto-remove` enabled are not supported
- `ext-container` and other pre-existing container nodes are not supported
- `network-mode: container:<...>` users/providers are not supported
- existing node definition reconciliation is limited to fields captured in the apply state file
- existing link parameter/type changes with the same runtime interface names are not applied in
  place; use `redeploy` for those changes

Apply discovers existing links from live interfaces that carry containerlab's ownership marker and
persists a `.state.clab.yaml` file under the lab directory after `deploy` and `apply`. The state file
stores the resolved topology used as the baseline for limited node definition reconciliation. Older
labs without this state file can still apply supported shape changes, but existing node definition
changes are not inferred until a state file has been written. Older or manually created interfaces
without containerlab's ownership marker are left untouched. If such an interface blocks a requested
link change, apply fails instead of deleting it. Removed `vxlan-stitch` host-side interfaces are
cleaned up on a best-effort basis when their default runtime names can be derived from the stale node
endpoint.

Deleted nodes are removed directly from the runtime. Node lab directories are kept.

### Examples

#### Preview topology changes

```bash
containerlab apply -t mylab.clab.yml --dry-run
```

#### Deploy or apply topology changes

```bash
containerlab apply -t mylab.clab.yml
```

If `mylab` is not running, the command deploys it. If it is running, the command applies supported
topology changes in place.

#### Apply by lab name

```bash
containerlab apply --name mylab
```
