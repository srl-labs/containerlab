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

### Link apply modes

When apply adds or removes a link on a node that keeps running, the node kind decides how
disruptive that change is. Three modes exist:

- `live` - the link change is applied in place; the node is neither restarted nor recreated.
  This requires the NOS to detect interfaces that appear or disappear at runtime (hotplug).
- `restart` - the link change is applied first and the existing container is then restarted so
  the NOS picks up the new interface inventory.
- `recreate` - the node container is deleted and created again. Generated runtime metadata such
  as the `CLAB_INTFS` environment variable and startup files are rebuilt. This is the
  conservative default for kinds that have not been validated for anything better.

The currently declared modes per kind:

| Kind                          | Mode       | Notes                                                            |
| ----------------------------- | ---------- | ---------------------------------------------------------------- |
| `nokia_srlinux` / `srl`       | `live`     | SR Linux detects hot-plugged interfaces                          |
| `nokia_srsim`                 | `live`     | SR-SIM detects hot-plugged interfaces                            |
| `linux`                       | `live`     | plain Linux containers see new interfaces immediately            |
| `ceos` / `arista_ceos`        | `restart`  | cEOS requires a restart to enumerate new interfaces              |
| vrnetlab-based VM kinds       | `recreate` | VM NIC wiring is fixed at VM boot; live changes cannot work      |
| images built with Boxen       | `live`     | detected via the `org.opencontainers.image.vendor=Boxen` label   |
| all other kinds               | `recreate` | conservative default for kinds not yet validated                 |

#### Overriding the mode

If you know that a node's NOS handles hot-plugged interfaces (or at least survives a plain
restart), you can override the kind default with the `link-apply-mode` property on a node, a
group, a kind, or the topology defaults:

```yaml
topology:
  nodes:
    r1:
      kind: juniper_crpd
      image: crpd:24.4R1.9
      link-apply-mode: live # apply link changes without recreating the node
```

The override takes precedence over the kind's declaration. When the override is more permissive
than the kind default, containerlab logs a warning: it is then your responsibility to verify the
NOS actually uses interfaces added this way.

#### Validating that a kind supports live link changes

The kernel will always show a hot-plugged interface inside the container namespace - the real
question is whether the NOS picks it up. To validate a kind:

1. Deploy a small lab with two nodes of that kind and one link.
2. Add a second link between the nodes in the topology file and run
   `containerlab apply -t <topo>` with `link-apply-mode: live` set on the nodes.
3. Confirm the node was not recreated (`docker inspect` start time is unchanged and the apply
   summary lists the change under `added links` without recreating the nodes).
4. Confirm the NOS sees and can use the new interface: it shows up in the NOS CLI, can be
   configured, and passes traffic.
5. Remove the link again with apply and confirm the NOS handles the removal gracefully.

If a kind passes this validation, please open an issue or pull request so the kind's default can
be changed for everyone - the change is a one-line declaration in the kind's node implementation.

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
