---
title: Match Tests to Blast Radius
impact: MEDIUM
impactDescription: Risky lifecycle and user-facing changes get verified at the right level
tags: tests, validation, strategy
---

## Match Tests to Blast Radius

Scale coverage with risk. Pure logic gets a focused unit test; topology/schema changes get valid+invalid parse tests plus schema/docs updates; contract changes get interface-level tests; deploy/apply/runtime changes get package tests plus a Robot Framework integration test when feasible. State which tests you ran and which you skipped and why.

**Incorrect (assert against one concrete implementation only):**

```go
func TestEndpoints(t *testing.T) { got := (&LinkVEth{}).GetEndpoints() /* ... */ }
```

**Correct (exercise the contract that all types must satisfy):**

```go
func TestApplyRuntimeEndpoints(t *testing.T) {
	for _, l := range []Link{vethLink, macvlanLink, vxlanLink} { /* assert subset */ }
}
```

Reference: `links/apply_test.go`, `core/apply_test.go`
