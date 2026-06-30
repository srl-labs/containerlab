---
title: Type Switches Belong Only at Boundaries
impact: HIGH
impactDescription: Distinguishes legitimate parser/factory dispatch from architecture smells
tags: architecture, boundaries, parsing, serialization
---

## Type Switches Belong Only at Boundaries

Concrete type switches are legitimate where the format itself is type-encoded: YAML parsing and shorthand translation, central registration or factory wiring, serialization, and third-party adapters. Everywhere else, a type switch or kind check is a smell — first ask whether the behavior belongs on the link, endpoint, node, runtime, or topology resolver.

**Acceptable (serialization boundary — the wire format is type-specific):**

```go
func (r *LinkDefinition) MarshalYAML() (any, error) {
	switch r.Link.GetType() {
	case LinkTypeVEth: // emit veth shorthand
	case LinkTypeHost: // emit host shorthand
	}
}
```

**Smell (behavior selection in generic flow — push onto the type instead):**

```go
if strings.Contains(node.Config().Kind, "srl") { icon = "srl.svg" }
```

Reference: `links/link.go` (`MarshalYAML`, `LinkType` factory), `core/graph.go`
