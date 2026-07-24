---
title: Thread context.Context Through Blocking Work
impact: MEDIUM
impactDescription: Cancellation and timeouts reach runtime, exec, and namespace calls
tags: go, context, cancellation
---

## Thread context.Context Through Blocking Work

Any function that talks to a runtime, runs a command, touches a namespace, or otherwise blocks should take and forward `ctx`. Don't store a context in a struct and don't synthesize a new background context partway down the call stack.

**Incorrect (synthesizes its own context):**

```go
func runCmd(n nodes.Node, cmd *clabexec.ExecCmd) error {
	_, err := n.RunExec(context.TODO(), cmd)
	return err
}
```

**Correct (accept and forward it):**

```go
func runCmd(ctx context.Context, n nodes.Node, cmd *clabexec.ExecCmd) error {
	_, err := n.RunExec(ctx, cmd)
	return err
}
```

Reference: `nodes/node.go` (`RunExec`), `exec/`, `runtime/runtime.go` (`Exec`)
