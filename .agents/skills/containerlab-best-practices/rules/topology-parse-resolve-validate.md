---
title: Keep Parse, Resolve, Validate, and Deploy Separate
impact: HIGH
impactDescription: Stops deploy code from re-parsing YAML or inferring semantics from raw fields
tags: topology, parsing, resolution, validation
---

## Keep Parse, Resolve, Validate, and Deploy Separate

Topology flows through distinct stages: raw structs describe input shape, resolve converts raw input to domain objects, domain objects validate and act, and deploy/apply drives the domain contracts. Deploy code must not parse YAML strings or infer topology meaning from raw fields.

**Incorrect (deploy re-parses raw input):**

```go
parts := strings.Split(rawLink.Endpoints[0], ":") // node:iface, parsed at deploy time
node := topo.Nodes[parts[0]]
```

**Correct (resolve once, then use the domain object):**

```go
link, err := rawLink.Resolve(resolveParams)
for _, ep := range link.GetEndpoints() { /* ... */ }
```

Reference: `links/link.go` (`RawLink.Resolve`), `core/clab.go` (`ResolveLinks`), `core/config.go`
