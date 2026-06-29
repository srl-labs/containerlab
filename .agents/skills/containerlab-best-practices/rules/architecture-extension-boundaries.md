---
title: Respect Extension Boundaries
impact: CRITICAL
impactDescription: Keeps node kinds, link types, runtimes, and topology features extensible
tags: architecture, interfaces, registries, package-boundaries
---

## Respect Extension Boundaries

Use Containerlab's extension points instead of duplicating type knowledge in generic code.

Prefer:

- Domain interfaces such as `links.Link`, `links.RawLink`, `links.Endpoint`, endpoint ownership/move contracts, `nodes.Node`, and `runtime.ContainerRuntime`.
- Node and runtime registries.
- Topology parser and resolver boundaries.
- Narrow optional interfaces for behavior that should not become part of a broad interface.

**Incorrect:**

```go
switch link := l.(type) {
case *LinkVEth:
	return link.Endpoints
case *LinkDummy:
	return link.Endpoints
}
```

**Correct:**

```go
return l.GetEndpoints()
```

Concrete type switches are acceptable at parser, registry, compatibility, and third-party adapter boundaries. Elsewhere, first ask whether the behavior belongs on the link, endpoint, node, runtime, or topology resolver.
