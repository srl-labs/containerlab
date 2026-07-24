---
title: Thread Context Through Lifecycle Operations
impact: HIGH
impactDescription: Makes deploy/destroy/apply cancellable and timeout-aware
tags: lifecycle, context, cancellation
---

## Thread Context Through Lifecycle Operations

Deploy, destroy, apply, exec, and every runtime/namespace call can block on real I/O. Pass the caller's `context.Context` through so cancellation and timeouts propagate. Do not start a fresh `context.Background()` deep in a lifecycle path.

**Incorrect (drops the caller's context):**

```go
func (n *node) Deploy() error {
	return rt.CreateContainer(context.Background(), n.cfg)
}
```

**Correct (propagate it):**

```go
func (n *node) Deploy(ctx context.Context) error {
	return rt.CreateContainer(ctx, n.cfg)
}
```

Reference: `core/deploy.go`, `nodes/node.go`, `runtime/runtime.go`
