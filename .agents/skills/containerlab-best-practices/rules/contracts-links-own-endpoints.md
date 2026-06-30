---
title: Links Own Endpoint Sets and Link Semantics
impact: HIGH
impactDescription: Keeps link behavior out of generic dispatch tables
tags: contracts, links, endpoints
---

## Links Own Endpoint Sets and Link Semantics

A link owns its endpoint collection, deploy/remove behavior, MTU, vars, and any apply-specific subset. Generic code should read these through the `links.Link` contract, including dedicated methods such as `GetRuntimeEndpoints()` when the generic endpoint set has the wrong semantics. Do not bolt on optional provider assertions in callers.

**Incorrect (reach into the concrete struct):**

```go
veth := l.(*LinkVEth)
for _, ep := range veth.Endpoints { /* ... */ }
```

**Correct (use the contract):**

```go
for _, ep := range l.GetEndpoints() { /* ... */ }
```

Reference: `links/link.go`, `links/apply.go`
