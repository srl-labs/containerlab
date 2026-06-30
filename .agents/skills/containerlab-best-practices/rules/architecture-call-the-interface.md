---
title: Call the Interface, Don't Type-Switch
impact: CRITICAL
impactDescription: Adding a type stays a one-file change instead of a thousand-place edit
tags: architecture, interfaces, links, polymorphism
---

## Call the Interface, Don't Type-Switch

When generic code needs behavior that an interface already exposes, call the method. Listing concrete types and handling each one means every new type must edit this site and every other one like it.

**Incorrect (generic code re-lists concrete link types):**

```go
switch link := l.(type) {
case *LinkVEth:
	return link.Endpoints
case *LinkDummy:
	return link.Endpoints
}
```

**Correct (the contract already exposes it):**

```go
return l.GetEndpoints()
```

Reference: `links/link.go` (`Link.GetEndpoints`)
