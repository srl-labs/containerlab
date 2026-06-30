---
title: Dry-Run Must Match Real Execution
impact: HIGH
impactDescription: Keeps apply's plan honest so users trust the preview
tags: lifecycle, apply, reconcile, dry-run
---

## Dry-Run Must Match Real Execution

Apply computes a plan and can run it in dry-run mode. The planning decision (create / delete / restart / recreate / live-update a node or link) must be derived from the same logic that execution uses, so the preview matches what actually happens. Don't let dry-run and execution diverge into two code paths.

**Incorrect (planning re-decides differently from execution):**

```go
if dryRun {
	plan.recreatedNodeSet[nodeName] = struct{}{} // hard-coded; execution may choose otherwise
}
```

**Correct (one decision feeds both):**

```go
if err := c.planNodeReconciliation(ctx, plan); err != nil {
	return err
}
```

Reference: `core/apply.go`, `core/topology_reconcile.go`, `core/options_apply.go`
