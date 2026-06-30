---
title: Prefer Additive Behavior Over Changed Defaults
impact: CRITICAL
impactDescription: Avoids silent behavior changes for existing topologies
tags: cli, defaults, compatibility, migration
---

## Prefer Additive Behavior Over Changed Defaults

Changing a default changes behavior for every existing topology that did not set the value. Prefer an opt-in. If a default genuinely must change, make the migration explicit with validation, docs, schema, and tests — never silently.

**Incorrect (flip a default in place):**

```go
const DefaultVethLinkMTU = 1500 // was 9500; every existing lab's MTU silently changes
```

**Correct (keep the default; let users opt in):**

```go
const DefaultVethLinkMTU = 9500
// users set `mtu:` per link/default to choose another value
```

Reference: `core/config.go`, `types/`
