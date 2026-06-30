# Containerlab Best Practices

**Version 2.0.0**
Containerlab agent guidance
June 2026

> This document is the compiled guide for agents maintaining, reviewing, or refactoring Containerlab. It is generated from `rules/` in the order defined by `rules/_sections.md`. Edit the individual rule files, not this file.

## Abstract

Containerlab is a CLI that manages real labs, containers, namespaces, links, generated files, and user topology definitions. Good changes preserve user contracts, keep operational flows recoverable, and place behavior at the abstraction that owns it. The link endpoint type-switch problem is one concrete example of the broader rule: generic code orchestrates through contracts, it does not rediscover implementation details. That rule applies across every subsystem below, and the largest of them is node kinds, not links.

## Containerlab Subsystems

Links and apply are a small slice. Know the whole surface before deciding where a change belongs:

- **Node kinds** — `nodes/` holds 60+ kinds (`srl`, `ceos`, `linux`, `bridge`, `host`, `ext_container`, `ovs`, `k8s_kind`, the `vr_*` VM family via vrnetlab, etc.). The dominant extension surface. Each kind implements `nodes.Node` (a ~40-method contract) and self-registers with `Register(r *nodes.NodeRegistry)`.
- **Runtimes** — `runtime/` with `docker` and `podman` providers behind `runtime.ContainerRuntime`, wired via `runtime/all`.
- **Links and endpoints** — `links/` (raw vs resolved links, endpoints, namespace moves, apply).
- **Topology and config inheritance** — `types/` and `core/config.go`. Values resolve through **defaults → kinds → groups → node** (`types/topology.go`).
- **Lifecycle and deploy ordering** — `core/` (`deploy`, `destroy`, `apply`, `topology_reconcile`, `restart`, `save`) plus `core/dependency_manager/`.
- **CLI and tools** — `cmd/` cobra commands and the `tools_*` family (`cert`, `vxlan`, `veth`, `netem`, `api`, `sshx`, `gotty`).
- **Generated artifacts (user-facing contracts)** — TLS/PKI (`cert/`, `core/cert.go`), inventory (`core/inventory.go`), `/etc/hosts` (`core/hostsfile.go`), SSH config (`core/sshconfig.go`), export (`core/export.go`), graphs (`core/graph.go`).
- **Supporting packages** — `exec/`, `netem/`, `netconf/`, `git/`, `virt/`, `cert/`, `constants/`, `utils/`, `internal/`.

---

# 1. User Compatibility (`cli`) — CRITICAL

CLI flags, topology syntax, labels, state files, and generated artifacts are user-facing contracts. Breaking them silently breaks real labs and automation.

## CLI Flags and Topology Syntax Are Contracts

Command names, flags, defaults, exit codes, topology YAML fields, and shorthand syntax are public API. Renaming, removing, or repurposing them breaks users' scripts and pipelines. Add new flags/fields; do not change the meaning of existing ones.

Incorrect — repurpose an existing flag:

```go
// --node-filter used to take names; now it silently takes a regex
cmd.Flags().StringVar(&nodeFilter, "node-filter", "", "regex of nodes")
```

Correct — add a new flag, keep the old behavior:

```go
cmd.Flags().StringVar(&nodeFilter, "node-filter", "", "comma-separated node names")
cmd.Flags().StringVar(&nodeFilterRegex, "node-filter-regex", "", "regex of nodes")
```

## Prefer Additive Behavior Over Changed Defaults

Changing a default changes behavior for every existing topology that did not set the value. Prefer an opt-in. If a default genuinely must change, make the migration explicit with validation, docs, schema, and tests — never silently.

Incorrect — flip a default in place:

```go
const DefaultVethLinkMTU = 1500 // was 9500; every existing lab's MTU silently changes
```

Correct — keep the default; let users opt in:

```go
const DefaultVethLinkMTU = 9500 // users set `mtu:` per link/default to choose another value
```

## Labels, State Files, and Paths Are Stable

Container labels, the lab directory layout (`clab-<lab>/`), generated file names, and inspect/output formats are consumed by users and tooling. Renaming a label or moving a generated file is a breaking change even though no flag changed.

Incorrect — rename an established label:

```go
labels["clab-nodename"] = node.ShortName // breaks `docker ps --filter label=clab-node-name`
```

Correct — use the established constant; add new keys, don't rename:

```go
labels[constants.NodeName] = node.ShortName // "clab-node-name"
```

---

# 2. Operational Lifecycle Safety (`lifecycle`) — CRITICAL

Deploy, destroy, apply, and reconcile act on real host and container state. They must be idempotent, context-aware, ordering-aware, and recoverable under partial failure.

## Make Cleanup Idempotent

Destroy, rollback, and cleanup run against labs that may be partially deployed, already gone, or left over from a crashed run. Treat "already absent" as success, not an error, so cleanup always converges.

Incorrect — a missing container discovered from topology aborts cleanup:

```go
containers, err := c.ListNodesContainers(ctx)
if err != nil {
	return err
}
```

Correct — tolerate already-gone topology containers during destroy discovery:

```go
containers, err := c.ListNodesContainersIgnoreNotFound(ctx)
if err != nil {
	return err
}
```

## Thread Context Through Lifecycle Operations

Deploy, destroy, apply, exec, and every runtime/namespace call can block on real I/O. Pass the caller's `context.Context` through so cancellation and timeouts propagate. Do not start a fresh `context.Background()` deep in a lifecycle path.

Incorrect — drops the caller's context:

```go
func (n *node) Deploy() error { return rt.CreateContainer(context.Background(), n.cfg) }
```

Correct — propagate it:

```go
func (n *node) Deploy(ctx context.Context) error { return rt.CreateContainer(ctx, n.cfg) }
```

## Dry-Run Must Match Real Execution

Apply computes a plan and can run it in dry-run mode. The planning decision (create / delete / restart / recreate / live-update a node or link) must be derived from the same logic execution uses, so the preview matches what actually happens. Don't fork dry-run and execution into two code paths.

Incorrect — planning re-decides differently from execution:

```go
if dryRun { plan.recreatedNodeSet[nodeName] = struct{}{} } // hard-coded; execution may choose otherwise
```

Correct — one decision feeds both:

```go
if err := c.planNodeReconciliation(ctx, plan); err != nil { return err }
```

## Respect Deploy Ordering Through the Dependency Manager

Node start order, `wait-for` dependencies, and health gating are coordinated by `core/dependency_manager`. Express ordering by registering nodes and letting the manager build and validate wait-for stages, not by hand-rolling sleeps or a fixed kind-based sequence.

Incorrect — guess the order with a sleep:

```go
deploy(srlNodes)
time.Sleep(5 * time.Second) // hope the fabric is up before clients
deploy(linuxNodes)
```

Correct — register nodes; let the manager schedule:

```go
for _, n := range nodes { c.dependencyManager.AddNode(n) }
if err := c.createWaitForDependency(); err != nil { return err }
if err := c.dependencyManager.CheckAcyclicity(); err != nil { return err }
```

---

# 3. Architecture and Extension Boundaries (`architecture`) — CRITICAL

Generic code orchestrates; concrete types own their behavior. Never dispatch on a link type, node kind, or runtime name — delegate to an interface and register new types.

## Call the Interface, Don't Type-Switch

When generic code needs behavior an interface already exposes, call the method. Listing concrete types and handling each one means every new type must edit this site and every other one like it.

Incorrect — generic code re-lists concrete link types:

```go
switch link := l.(type) {
case *LinkVEth:  return link.Endpoints
case *LinkDummy: return link.Endpoints
}
```

Correct — the contract already exposes it:

```go
return l.GetEndpoints()
```

## Promote Missing Behavior to the Owning Interface

When generic code needs behavior that is not on the interface it already receives, add the method to the owning interface and implement it for every concrete type. A type assertion moves missed implementations from compile time to runtime, which defeats the point of the interface. Apply needs runtime-owned endpoints, a subset of `GetEndpoints()` (macvlan excludes its host endpoint, vxlan excludes the remote endpoint), so the subset belongs on `Link`.

Incorrect — optional provider hides missing implementations until runtime:

```go
type runtimeEndpointProvider interface{ runtimeEndpoints() []Endpoint }

func ApplyRuntimeEndpoints(l Link) []Endpoint {
	if link, ok := l.(runtimeEndpointProvider); ok {
		return materialEndpoints(link.runtimeEndpoints())
	}
	return materialEndpoints(l.GetEndpoints()) // fallback: runtime set == all endpoints
}
```

Correct — the owning contract exposes the behavior:

```go
type Link interface {
	GetEndpoints() []Endpoint
	GetRuntimeEndpoints() []Endpoint
}

func ApplyRuntimeEndpoints(l Link) []Endpoint {
	return materialEndpoints(l.GetRuntimeEndpoints())
}
```

## Never Branch on node.Config().Kind

Node kinds are the largest extension surface (60+ kinds). Generic code must never switch on the kind string. Let the node answer, and switch on the returned value. Apply's "live-update vs restart vs recreate" decision is the model: kind-specific, but the node owns the policy via `LinkApplyMode`, read through `nodes.LinkApplyModeForNode`.

Incorrect — a 61st kind means editing this and N other sites:

```go
switch node.Config().Kind {
case "vr_vmx", "vr_xrv9k": mode = recreate
case "ceos":              mode = restart
default:                  mode = live
}
```

Correct — the node owns the policy; switch on the mode enum:

```go
switch nodes.LinkApplyModeForNode(ctx, node) {
case nodes.LinkApplyModeRecreate: // ...
}
```

## Don't Check Runtime Names; Call a Runtime Method

Generic code that special-cases `"docker"` or `"podman"` by name leaks provider differences out of the runtime layer. Put the difference behind a `runtime.ContainerRuntime` method; each provider implements its own behavior.

Incorrect — provider behavior leaks into generic code:

```go
if rt.GetName() == "podman" { socket = "/run/podman/podman.sock" } else { socket = "/var/run/docker.sock" }
```

Correct — ask the runtime:

```go
socket, err := rt.GetRuntimeSocket()
if err != nil { return err }
```

## Register New Kinds and Runtimes; Don't Add a Central Case

Node kinds and runtimes are wired through registries. A new kind is a new `nodes/<kind>/` package implementing `nodes.Node` plus a `Register(r *nodes.NodeRegistry)` call; a new runtime registers with `runtime.Register`. If adding a type forces a `switch`/`if` edit in `core`, `cmd`, or `links`, the behavior is in the wrong place.

Incorrect — central registry of concrete types in generic code:

```go
func newNode(kind string) nodes.Node {
	switch kind {
	case "srl":  return &srl{}
	case "ceos": return &ceos{} // every new kind edits here
	}
}
```

Correct — each kind self-registers; generic code asks the registry:

```go
func Register(r *nodes.NodeRegistry) { // in nodes/<kind>/<kind>.go
	r.Register(kindNames, func() nodes.Node { return new(myKind) }, nil)
}
node, err := reg.NewNodeOfKind(kind) // generic code
```

## Type Assertions Belong Only at Boundaries

Concrete type switches and type assertions are legitimate only at narrow boundaries: parser/factory routing after YAML or shorthand decoding, third-party adapters that hand back `any`, and compatibility shims while migrating an old API to a unified one. Everywhere else, a type assertion or kind check is a smell — first add the missing behavior to the link, endpoint, node, runtime, or topology resolver interface.

Acceptable — serialization boundary (the wire format is type-specific):

```go
func (r *LinkDefinition) MarshalYAML() (any, error) {
	switch r.Link.GetType() {
	case LinkTypeVEth: // emit veth shorthand
	case LinkTypeHost: // emit host shorthand
	}
}
```

Smell — behavior selection in generic flow (add the method instead):

```go
if stitcher, ok := link.(interface{ Stitch() error }); ok {
	return stitcher.Stitch()
}
```

---

# 4. Link, Endpoint, Node, and Runtime Contracts (`contracts`) — HIGH

Behavior belongs to the abstraction that owns it.

## Links Own Endpoint Sets and Link Semantics

A link owns its endpoint collection, deploy/remove behavior, MTU, vars, and any apply-specific subset. Read these through the `links.Link` contract, including dedicated methods such as `GetRuntimeEndpoints()` when the generic endpoint set has the wrong semantics. Do not bolt on optional provider assertions in callers.

Incorrect — reach into the concrete struct:

```go
veth := l.(*LinkVEth)
for _, ep := range veth.Endpoints { /* ... */ }
```

Correct — use the contract:

```go
for _, ep := range l.GetEndpoints() { /* ... */ }
```

## Endpoints Own Namespace Moves and Activation

An endpoint owns its interface identity, runtime-discovered state, link back-reference (`GetLink`), namespace movement (`MoveTo`), and activation (`Activate`). Drive these through the `links.Endpoint` contract rather than reaching into concrete endpoint structs or re-deriving the node's namespace.

Incorrect — generic code performs the move itself:

```go
ns, _ := ns.GetNSFromPath(ep.GetNode().nsPath)
netlink.LinkSetNsFd(link, int(ns.Fd()))
```

Correct — the endpoint moves itself:

```go
if err := ep.MoveTo(ctx, ep.GetNode()); err != nil { return err }
```

## Nodes Own Kind-Specific Behavior

Endpoint normalization, interface-name validation, interface indexing, config generation, deploy hooks, health, and link-apply policy are node responsibilities. Call the `nodes.Node` methods, don't re-implement kind logic.

Incorrect — generic code validates an interface name per kind:

```go
if node.Config().Kind == "srl" && !strings.HasPrefix(name, "e1-") { return fmt.Errorf("invalid interface %q for srl", name) }
```

Correct — the node validates its own interface name:

```go
if err := node.CheckInterfaceName(); err != nil { return err }
```

Methods to prefer: `AddEndpoint`, `CheckInterfaceName`, `CalculateInterfaceIndex`, `DeployEndpoints`, `PostDeploy`, `LinkApplyMode`.

## Runtimes Own Provider Behavior

Container, network, label, and namespace operations, plus every docker/podman API difference, belong behind `runtime.ContainerRuntime`. Generic code calls the interface; it never reaches for a provider-specific client or branches on the provider name.

Incorrect — generic code talks to a provider client directly:

```go
cli, _ := dockerC.NewClientWithOpts(dockerC.FromEnv, dockerC.WithAPIVersionNegotiation())
cli.ContainerStart(ctx, id, container.StartOptions{})
```

Correct — go through the runtime contract:

```go
if err := rt.StartContainer(ctx, id, node); err != nil { return err }
```

---

# 5. Topology, Schema, and Docs (`topology`) — HIGH

Topology syntax is a public API. Keep parsing, resolution, validation, inheritance, schema, docs, examples, and generated artifacts aligned.

## Keep Parse, Resolve, Validate, and Deploy Separate

Raw structs describe input shape, resolve converts raw input to domain objects, domain objects validate and act, and deploy/apply drives the domain contracts. Deploy code must not parse YAML strings or infer topology meaning from raw fields.

Incorrect — deploy re-parses raw input:

```go
parts := strings.Split(rawLink.Endpoints[0], ":") // node:iface, parsed at deploy time
node := topo.Nodes[parts[0]]
```

Correct — resolve once, then use the domain object:

```go
link, err := rawLink.Resolve(resolveParams)
for _, ep := range link.GetEndpoints() { /* ... */ }
```

## Read Effective Config Through the Inheritance Chain

Per-node values resolve through **defaults → kinds → groups → node**. Read the effective value through the topology helpers (`GetNode*`), which apply that precedence. Reading a raw field directly, or re-implementing the merge, silently ignores `kinds`/`groups`/`defaults` settings.

Incorrect — reads only the node-level field:

```go
img := topo.Nodes[name].Image // ignores defaults/kinds/groups image
```

Correct — resolve through the chain:

```go
img := topo.GetNodeImage(name) // applies defaults → kinds → groups → node
```

## Keep Schema, Docs, and Examples Aligned With Topology

A new or changed topology field is not done until the JSON schema, docs, and examples match. Users author topologies against the schema (editor validation/completion) and the docs; an unschematized field fails validation or goes unnoticed.

Incorrect — add a hypothetical topology field to the struct only:

```go
type NodeDefinition struct {
	FooBar string `yaml:"foo-bar,omitempty"` // new field, but not in schema or docs
}
```

Correct — struct + `types/types.go` when applicable + `schemas/clab.schema.json` + `docs/` + `lab-examples/` + tests, together.

## Generated Artifacts Are User-Facing Contracts

Containerlab generates files users depend on: TLS/PKI, inventory, `/etc/hosts`, SSH config, export, and graphs. A change to a name, label, path, or default can silently break automation that consumes these even when no flag or YAML field changed. Update the generator and its tests together.

Incorrect — change inventory grouping silently:

```go
group := node.Config().Kind // playbooks keyed on the old grouping break
```

Correct — preserve the established key; extend additively:

```go
group := ansibleInventoryGroup(node.Config())
```

Generators: `core/inventory.go`, `core/cert.go`, `core/hostsfile.go`, `core/sshconfig.go`, `core/export.go`, `core/graph.go`.

---

# 6. Go Context, Errors, and Logging (`go`) — MEDIUM-HIGH

Thread context through blocking work, return and wrap errors instead of only logging, and match existing package patterns.

## Thread context.Context Through Blocking Work

Any function that talks to a runtime, runs a command, touches a namespace, or otherwise blocks should take and forward `ctx`. Don't store a context in a struct and don't synthesize a new background context partway down the stack.

Incorrect:

```go
func runCmd(n nodes.Node, cmd *clabexec.ExecCmd) error {
	_, err := n.RunExec(context.TODO(), cmd)
	return err
}
```

Correct:

```go
func runCmd(ctx context.Context, n nodes.Node, cmd *clabexec.ExecCmd) error {
	_, err := n.RunExec(ctx, cmd)
	return err
}
```

## Return and Wrap Errors; Don't Only Log Them

Logging an error and continuing hides failure from the caller. Return it, and wrap it with the operation plus the lab/node/link/interface/file that failed.

Incorrect — swallow and continue:

```go
if err := node.Deploy(ctx, deployParams); err != nil { log.Errorf("deploy failed: %v", err) }
```

Correct — wrap with context and return:

```go
if err := node.Deploy(ctx, deployParams); err != nil {
	return fmt.Errorf("deploying node %q: %w", node.Config().ShortName, err)
}
```

## Match Local Patterns; Don't Add Speculative Abstractions

Don't introduce a new interface, generic, or framework for a single caller. Match the surrounding package. Add an abstraction when a real second implementation or caller exists — the right time to add a narrow interface is when behavior actually diverges, not before.

Incorrect — a hypothetical framework for one helper:

```go
type NodeBehaviorStrategyFactoryProvider interface{ Provide() Strategy } // one impl, used once
```

Correct — a plain function next to its caller:

```go
func runtimeContainerNodeName(ctr clabruntime.GenericContainer) string {
	if name := ctr.Labels[clabconstants.NodeName]; name != "" {
		return name
	}
	if len(ctr.Names) > 0 {
		return ctr.Names[0]
	}
	return ""
}
```

---

# 7. Tests and Validation (`tests`) — MEDIUM-HIGH

Scale test coverage with blast radius.

## Match Tests to Blast Radius

Pure logic gets a focused unit test; topology/schema changes get valid+invalid parse tests plus schema/docs updates; contract changes get interface-level tests; deploy/apply/runtime changes get package tests plus a Robot Framework integration test when feasible. State which tests you ran and which you skipped and why.

Incorrect — assert against one concrete implementation only:

```go
func TestEndpoints(t *testing.T) { got := (&LinkVEth{}).GetEndpoints() /* ... */ }
```

Correct — exercise the contract every type must satisfy:

```go
func TestApplyRuntimeEndpoints(t *testing.T) {
	for _, l := range []Link{vethLink, macvlanLink, vxlanLink} { /* assert subset */ }
}
```

## Test Contracts Through Interfaces and Fakes

When the behavior under test is a contract (link, endpoint, node, runtime), test against the interface with a fake rather than wiring a real container. This proves the contract and keeps the test fast and deterministic.

Incorrect — needs a real runtime to test apply logic:

```go
rt, _ := docker.NewDockerRuntime() // requires docker in CI for a pure-logic test
```

Correct — a fake satisfying the interface:

```go
type applyRuntimeFakeLink struct{ eps []Endpoint }
func (l *applyRuntimeFakeLink) GetEndpoints() []Endpoint { return l.eps }
```

## Use Robot Framework Tests for Real Lifecycle Behavior

Changes that touch real container lifecycle, networking, filesystem state, or CLI workflows need an integration test, not only unit tests. The suite is Robot Framework under `tests/`, run via `tests/rf-run.sh`.

```bash
go test ./links ./nodes ./core ./runtime/...   # fast, scoped to packages you touched
make test                                       # whole module with -race + coverage (slower)
CLAB_BIN=$(pwd)/bin/containerlab ./tests/rf-run.sh docker tests/01-smoke/29-apply.robot
```

Useful architecture-review search (treat hits as prompts, not automatic failures — parser/registry/adapter boundaries can legitimately inspect types):

```bash
rg -n "Config\(\)\.Kind|\.GetName\(\) ==|\.\(type\)" core links cmd
```
