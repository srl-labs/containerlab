---
title: Preserve User Compatibility
impact: CRITICAL
impactDescription: Avoids breaking existing labs, automation, docs, and CLI workflows
tags: cli, topology, compatibility, user-contracts
---

## Preserve User Compatibility

Treat these as user-facing contracts:

- CLI commands, flags, defaults, output, and exit behavior.
- Topology YAML fields, defaults, shorthand syntax, and validation.
- Container labels, state files, generated configs, inventory, certificates, and paths.
- Docs, examples, and schemas.

Prefer additive behavior over changed defaults. If a breaking change is intentional, make migration explicit with validation, docs, schema updates, and tests.

Before changing user-visible behavior, inspect nearby code in `cmd/`, `types/`, `core/config.go`, `schemas/`, `docs/`, and `lab-examples/`.
