---
title: Endpoints Own Namespace Moves and Activation
impact: HIGH
impactDescription: Keeps endpoint-local state and side effects on the endpoint
tags: contracts, endpoints, namespaces
---

## Endpoints Own Namespace Moves and Activation

An endpoint owns its interface identity, runtime-discovered state, link back-reference (`GetLink`), namespace movement (`MoveTo`), and activation (`Activate`). Generic code should drive these through the `links.Endpoint` contract rather than reaching into concrete endpoint structs or re-deriving the node's namespace.

**Incorrect (generic code performs the move itself):**

```go
ns, _ := ns.GetNSFromPath(ep.GetNode().nsPath)
netlink.LinkSetNsFd(link, int(ns.Fd()))
```

**Correct (the endpoint moves itself):**

```go
if err := ep.MoveTo(ctx, ep.GetNode()); err != nil { return err }
```

Reference: `links/endpoint.go` (`MoveTo`, `Activate`, `GetLink`)
