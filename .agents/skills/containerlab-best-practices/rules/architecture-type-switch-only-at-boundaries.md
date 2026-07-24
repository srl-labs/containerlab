---
title: Type Assertions Belong Only at Boundaries
impact: HIGH
impactDescription: Distinguishes legitimate parser/factory dispatch from architecture smells
tags: architecture, boundaries, parsing, serialization, type-assertions
---

## Type Assertions Belong Only at Boundaries

Concrete type switches and type assertions are legitimate only at narrow boundaries: parser/factory routing after YAML or shorthand decoding, third-party adapters that hand back `any`, and compatibility shims while migrating an old API to a unified one. Everywhere else, a type assertion or kind check is a smell — first add the missing behavior to the link, endpoint, node, runtime, or topology resolver interface.

**Acceptable (serialization boundary — the wire format is type-specific):**

```go
func (r *LinkDefinition) MarshalYAML() (any, error) {
	switch r.Link.GetType() {
	case LinkTypeVEth: // emit veth shorthand
	case LinkTypeHost: // emit host shorthand
	}
}
```

**Smell (behavior selection in generic flow — add the method instead):**

```go
if stitcher, ok := link.(interface{ Stitch() error }); ok {
	return stitcher.Stitch()
}
```

Reference: `links/link.go` (`MarshalYAML`, `LinkType` factory), `core/graph.go`
