---
title: Add a Narrow Optional Interface When Semantics Diverge
impact: CRITICAL
impactDescription: Avoids type switches and silent bugs when a broad method's contract differs
tags: architecture, interfaces, optional-interface, apply
---

## Add a Narrow Optional Interface When Semantics Diverge

When generic code needs behavior that is *not* an existing method — and especially when its contract differs from a broad method — do not overload the broad method and do not type-switch. Add a narrow optional interface that the relevant types implement, with a sane fallback for the rest. Apply needs runtime-owned endpoints, a subset of `GetEndpoints()` (macvlan excludes its host endpoint, vxlan excludes the remote endpoint), so the subset lives on the types.

**Incorrect (reuse a broad method whose contract differs — silently wrong):**

```go
// includes the parent/remote endpoints apply must skip
return materialEndpoints(l.GetEndpoints())
```

**Correct (narrow optional interface with fallback):**

```go
type runtimeEndpointProvider interface{ runtimeEndpoints() []Endpoint }

func ApplyRuntimeEndpoints(l Link) []Endpoint {
	if link, ok := l.(runtimeEndpointProvider); ok {
		return materialEndpoints(link.runtimeEndpoints())
	}
	return materialEndpoints(l.GetEndpoints()) // fallback: runtime set == all endpoints
}
```

Reference: `links/apply.go`
