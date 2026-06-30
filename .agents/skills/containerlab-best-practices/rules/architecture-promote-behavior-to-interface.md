---
title: Promote Missing Behavior to the Owning Interface
impact: CRITICAL
impactDescription: Moves missed implementations from runtime failures to compile-time errors
tags: architecture, interfaces, type-assertions, apply
---

## Promote Missing Behavior to the Owning Interface

When generic code needs behavior that is not on the interface it already receives, add the method to the owning interface and implement it for every concrete type. A type assertion moves missed implementations from compile time to runtime, which defeats the point of the interface. Apply needs runtime-owned endpoints, a subset of `GetEndpoints()` (macvlan excludes its host endpoint, vxlan excludes the remote endpoint), so the subset belongs on `Link`.

**Incorrect (optional provider hides missing implementations until runtime):**

```go
type runtimeEndpointProvider interface{ runtimeEndpoints() []Endpoint }

func ApplyRuntimeEndpoints(l Link) []Endpoint {
	if link, ok := l.(runtimeEndpointProvider); ok {
		return materialEndpoints(link.runtimeEndpoints())
	}
	return materialEndpoints(l.GetEndpoints())
}
```

**Correct (the owning contract exposes the behavior):**

```go
type Link interface {
	GetEndpoints() []Endpoint
	GetRuntimeEndpoints() []Endpoint
}

func ApplyRuntimeEndpoints(l Link) []Endpoint {
	return materialEndpoints(l.GetRuntimeEndpoints())
}
```

Reference: `links/link.go`, `links/apply.go`
