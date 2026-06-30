---
title: Register New Kinds and Runtimes; Don't Add a Central Case
impact: CRITICAL
impactDescription: Adding a kind/runtime touches its own package, not core/cmd/links
tags: architecture, registries, nodes, runtime, extension
---

## Register New Kinds and Runtimes; Don't Add a Central Case

Node kinds and runtimes are wired through registries. A new kind is a new `nodes/<kind>/` package implementing `nodes.Node` plus a `Register(r *nodes.NodeRegistry)` call; a new runtime registers with `runtime.Register`. If adding a type forces you to edit a `switch`/`if` in `core`, `cmd`, or `links`, the behavior is in the wrong place — move it onto the type.

**Incorrect (central registry of concrete types in generic code):**

```go
func newNode(kind string) nodes.Node {
	switch kind {
	case "srl":   return &srl{}
	case "ceos":  return &ceos{}
	// every new kind edits here
	}
}
```

**Correct (each kind self-registers; generic code asks the registry):**

```go
// in nodes/<kind>/<kind>.go
func Register(r *nodes.NodeRegistry) {
	r.Register(kindNames, func() nodes.Node { return new(myKind) }, nil)
}
// generic code:
node, err := reg.NewNodeOfKind(kind)
```

Reference: `nodes/node_registry.go`, `nodes/srl/srl.go` (`Register`)
