---
title: CLI Flags and Topology Syntax Are Contracts
impact: CRITICAL
impactDescription: Prevents breaking existing labs, scripts, and CI
tags: cli, topology, compatibility
---

## CLI Flags and Topology Syntax Are Contracts

Command names, flags, defaults, exit codes, topology YAML fields, and shorthand syntax are public API. Renaming, removing, or repurposing them breaks users' scripts and pipelines. Add new flags/fields; do not change the meaning of existing ones.

**Incorrect (repurpose an existing flag):**

```go
// --node-filter used to take names; now it silently takes a regex
cmd.Flags().StringVar(&nodeFilter, "node-filter", "", "regex of nodes")
```

**Correct (add a new flag, keep the old behavior):**

```go
cmd.Flags().StringVar(&nodeFilter, "node-filter", "", "comma-separated node names")
cmd.Flags().StringVar(&nodeFilterRegex, "node-filter-regex", "", "regex of nodes")
```

Reference: `cmd/` (cobra commands)
