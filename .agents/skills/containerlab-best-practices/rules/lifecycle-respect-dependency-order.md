---
title: Respect Deploy Ordering Through the Dependency Manager
impact: HIGH
impactDescription: Prevents races and broken wait-for/health gating on deploy
tags: lifecycle, deploy, dependencies, ordering
---

## Respect Deploy Ordering Through the Dependency Manager

Node start order, `wait-for` dependencies, and health gating are coordinated by `core/dependency_manager`. Express ordering by registering dependencies and stages, not by hand-rolling sleeps, ad hoc goroutine ordering, or a fixed kind-based sequence.

**Incorrect (guess the order with a sleep):**

```go
deploy(srlNodes)
time.Sleep(5 * time.Second) // hope the fabric is up before clients
deploy(linuxNodes)
```

**Correct (register nodes; let the manager build and validate wait-for stages):**

```go
for _, n := range nodes {
	c.dependencyManager.AddNode(n)
}
if err := c.createWaitForDependency(); err != nil { return err }
if err := c.dependencyManager.CheckAcyclicity(); err != nil { return err }
```

Reference: `core/dependency_manager/`, `core/deploy.go` (`createWaitForDependency`)
