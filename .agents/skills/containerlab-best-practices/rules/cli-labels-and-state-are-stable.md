---
title: Labels, State Files, and Paths Are Stable
impact: HIGH
impactDescription: Protects automation that reads labels, inventory, and lab directories
tags: cli, labels, state, artifacts
---

## Labels, State Files, and Paths Are Stable

Container labels, the lab directory layout (`clab-<lab>/`), generated file names, and inspect/output formats are consumed by users and tooling. Renaming a label or moving a generated file is a breaking change even though no flag changed.

**Incorrect (rename an established label):**

```go
labels["clab-nodename"] = node.ShortName // breaks `docker ps --filter label=clab-node-name`
```

**Correct (use the established constant; add new keys, don't rename):**

```go
labels[constants.NodeName] = node.ShortName // "clab-node-name"
```

Reference: `constants/labels.go`, `runtime/`
