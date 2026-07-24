---
title: Match Local Patterns; Don't Add Speculative Abstractions
impact: MEDIUM
impactDescription: Keeps one-off code small and consistent with its package
tags: go, simplicity, patterns
---

## Match Local Patterns; Don't Add Speculative Abstractions

Don't introduce a new interface, generic, or framework for a single caller. Match the surrounding package's patterns. Add an abstraction only when a real second implementation or caller exists — the right time to add a narrow interface is when behavior actually diverges, not before.

**Incorrect (a hypothetical framework for one helper):**

```go
type NodeBehaviorStrategyFactoryProvider interface{ Provide() Strategy }
// ...one implementation, used once
```

**Correct (a plain function next to its caller):**

```go
func runtimeContainerNodeName(ctr clabruntime.GenericContainer) string {
	if name := ctr.Labels[clabconstants.NodeName]; name != "" {
		return name
	}
	if len(ctr.Names) > 0 {
		return ctr.Names[0]
	}
	return ""
}
```

Reference: `core/runtime_state.go` (`runtimeContainerNodeName`)
