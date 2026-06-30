---
title: Make Cleanup Idempotent
impact: CRITICAL
impactDescription: Destroy and rollback survive partially-deployed labs
tags: lifecycle, destroy, cleanup, idempotency
---

## Make Cleanup Idempotent

Destroy, rollback, and cleanup run against labs that may be partially deployed, already gone, or left over from a crashed run. Treat "already absent" as success, not an error, so cleanup always converges.

**Incorrect (a missing container discovered from topology aborts cleanup):**

```go
containers, err := c.ListNodesContainers(ctx)
if err != nil {
	return err
}
```

**Correct (tolerate already-gone topology containers during destroy discovery):**

```go
containers, err := c.ListNodesContainersIgnoreNotFound(ctx)
if err != nil {
	return err
}
```

Reference: `core/destroy.go` (`ListNodesContainersIgnoreNotFound`), `runtime/runtime.go` (`DeleteContainer`)
