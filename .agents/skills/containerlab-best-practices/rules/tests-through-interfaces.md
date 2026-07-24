---
title: Test Contracts Through Interfaces and Fakes
impact: MEDIUM
impactDescription: Tests stay valid as new kinds, links, and runtimes are added
tags: tests, interfaces, fakes
---

## Test Contracts Through Interfaces and Fakes

When the behavior under test is a contract (link, endpoint, node, runtime), test against the interface with a fake rather than wiring a real container. This both proves the contract and keeps the test fast and deterministic.

**Incorrect (needs a real runtime to test apply logic):**

```go
rt, _ := docker.NewDockerRuntime() // requires docker in CI for a pure-logic test
```

**Correct (a fake satisfying the interface):**

```go
type applyRuntimeFakeLink struct{ eps []Endpoint }
func (l *applyRuntimeFakeLink) GetEndpoints() []Endpoint { return l.eps }
// ...drive ApplyRuntimeEndpoints with the fake
```

Reference: `links/apply_test.go`, `mocks/`
