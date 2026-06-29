---
title: Keep Topology, Schema, and Docs Aligned
impact: HIGH
impactDescription: Prevents undocumented or unschematized user-facing topology behavior
tags: topology, schema, docs, examples, validation
---

## Keep Topology, Schema, and Docs Aligned

Topology syntax is a public API. When changing user-configurable behavior, update the whole surface:

- Parser and raw topology structs.
- Resolution and validation.
- `schemas/`.
- `docs/`.
- `lab-examples/` when examples should show the behavior.
- Unit tests for valid and invalid input.

Keep responsibilities separated:

- Raw topology structures describe input shape.
- Resolve code converts raw input to domain objects.
- Domain objects validate and perform behavior.
- Deploy/apply code should not parse YAML strings or infer topology semantics from raw fields.
