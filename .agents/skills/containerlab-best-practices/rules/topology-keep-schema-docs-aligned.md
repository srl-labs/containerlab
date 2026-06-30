---
title: Keep Schema, Docs, and Examples Aligned With Topology
impact: HIGH
impactDescription: Prevents undocumented or unschematized user-facing fields
tags: topology, schema, docs, examples
---

## Keep Schema, Docs, and Examples Aligned With Topology

A new or changed topology field is not done until the JSON schema, docs, and examples match. Users author topologies against the schema (editor validation/completion) and the docs; an unschematized field fails validation or goes unnoticed.

**Incorrect (add a hypothetical topology field to the struct only):**

```go
type NodeDefinition struct {
	FooBar string `yaml:"foo-bar,omitempty"` // new field, but not in schema or docs
}
```

**Correct (struct + schema + docs + example + tests):**

```text
types/node_definition.go   # e.g. StartupDelay yaml tag
types/types.go             # JSON/export shape when applicable
schemas/clab.schema.json   # e.g. startup-delay property
docs/manual/nodes.md        # user-facing docs
lab-examples/...            # example when useful
```

Reference: `schemas/clab.schema.json`, `docs/`, `lab-examples/`
