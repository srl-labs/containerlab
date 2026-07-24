---
title: Return and Wrap Errors; Don't Only Log Them
impact: MEDIUM
impactDescription: Failures reach the user with enough context to fix the lab
tags: go, errors, diagnostics
---

## Return and Wrap Errors; Don't Only Log Them

Logging an error and continuing hides failure from the caller. Return it, and wrap it with the operation plus the lab/node/link/interface/file that failed so the message is actionable.

**Incorrect (swallow and continue):**

```go
if err := node.Deploy(ctx, deployParams); err != nil {
	log.Errorf("deploy failed: %v", err) // caller thinks it succeeded
}
```

**Correct (wrap with context and return):**

```go
if err := node.Deploy(ctx, deployParams); err != nil {
	return fmt.Errorf("deploying node %q: %w", node.Config().ShortName, err)
}
```

Reference: `nodes/node.go` (`Deploy`), `core/deploy.go`, `errors/`
