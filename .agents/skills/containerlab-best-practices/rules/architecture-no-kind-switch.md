---
title: Never Branch on node.Config().Kind
impact: CRITICAL
impactDescription: Keeps 60+ node kinds extensible without central edits
tags: architecture, nodes, kinds, polymorphism
---

## Never Branch on node.Config().Kind

Node kinds are the largest extension surface (60+ kinds in `nodes/`). Generic code must never switch on the kind string. Let the node answer, and switch on the returned value, not the kind. Apply's "live-update vs restart vs recreate" decision is the model: it is kind-specific, but the node owns the policy via `LinkApplyMode`, read through `nodes.LinkApplyModeForNode`.

**Incorrect (a 61st kind means editing this and N other sites):**

```go
switch node.Config().Kind {
case "vr_vmx", "vr_xrv9k": mode = recreate
case "ceos":              mode = restart
default:                  mode = live
}
```

**Correct (the node owns the policy; switch on the mode enum):**

```go
switch nodes.LinkApplyModeForNode(ctx, node) {
case nodes.LinkApplyModeRecreate: // ...
}
```

Reference: `nodes/node.go` (`LinkApplyModeForNode`), `nodes/vr_node.go`, `nodes/ceos/ceos.go`
