---
title: Read Effective Config Through the Inheritance Chain
impact: HIGH
impactDescription: Honors defaults -> kinds -> groups -> node without re-implementing the merge
tags: topology, inheritance, defaults, config
---

## Read Effective Config Through the Inheritance Chain

Per-node values resolve through **defaults -> kinds -> groups -> node**. Read the effective value through the topology helpers (`GetNode*`), which apply that precedence. Reading a raw field directly, or re-implementing the merge in feature code, silently ignores `kinds`/`groups`/`defaults` settings.

**Incorrect (reads only the node-level field):**

```go
img := topo.Nodes[name].Image // ignores defaults/kinds/groups image
```

**Correct (resolve through the chain):**

```go
img := topo.GetNodeImage(name) // applies defaults -> kinds -> groups -> node
```

Reference: `types/topology.go` (`Defaults`/`Kinds`/`Groups`, `GetNode*`)
