---
title: Nodes Own Kind-Specific Behavior
impact: HIGH
impactDescription: Keeps per-kind logic on the node instead of in generic flows
tags: contracts, nodes, kinds
---

## Nodes Own Kind-Specific Behavior

Endpoint normalization, interface-name validation, interface indexing, config generation, deploy hooks, health, and link-apply policy are node responsibilities. Generic code should call the `nodes.Node` methods, not re-implement kind logic.

**Incorrect (generic code validates an interface name per kind):**

```go
if node.Config().Kind == "srl" && !strings.HasPrefix(name, "e1-") {
	return fmt.Errorf("invalid interface %q for srl", name)
}
```

**Correct (the node validates its own interface name):**

```go
if err := node.CheckInterfaceName(); err != nil { return err }
```

Reference: `nodes/node.go` (`AddEndpoint`, `CheckInterfaceName`, `CalculateInterfaceIndex`, `DeployEndpoints`, `PostDeploy`, `LinkApplyMode`)
