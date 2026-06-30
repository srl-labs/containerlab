# Containerlab Agent Guidance

This repository includes an agent skill package for Containerlab-specific engineering rules:

- Skill entrypoint: `.agents/skills/containerlab-best-practices/SKILL.md`
- Compiled guide: `.agents/skills/containerlab-best-practices/AGENTS.md`
- Individual rules: `.agents/skills/containerlab-best-practices/rules/`

Use these rules whenever changing Containerlab Go code, CLI behavior, topology parsing, schemas, docs, node/link implementations, runtimes, apply/reconcile, deploy/destroy, or tests.

## Core Principles

- Preserve user compatibility. Topology syntax, CLI flags, labels, state files, and generated artifacts are user-facing contracts.
- Keep behavior at the owning abstraction. Links own link behavior, nodes own kind behavior, runtimes own provider behavior, and core orchestrates rather than re-implementing those details.
- Treat deploy, destroy, apply, and reconcile as operationally sensitive. Prefer idempotent cleanup, context-aware operations, and explicit lifecycle decisions.
- Update docs, schemas, examples, and tests with user-visible behavior changes.

## Before Editing

1. Read the relevant rule file under `.agents/skills/containerlab-best-practices/rules/`.
2. Search for existing extension points before adding branches:

```bash
rg -n "type .* interface|Register\\(|Resolve\\(|GetEndpoints|LinkApplyMode|ContainerRuntime|cobra.Command" links nodes core runtime cmd types
```

   And check whether you are about to add a type/kind/runtime-name branch the architecture forbids:

```bash
rg -n "Config\\(\\)\\.Kind|\\.GetName\\(\\) ==|\\.\\(type\\)" core links cmd
```

3. Keep changes scoped to the affected package boundary. Adding a node kind means a new `nodes/<kind>/` package plus `Register(r *nodes.NodeRegistry)` — not a new `case` in core, cmd, or links.
4. Add focused tests that exercise the contract, not only the one implementation you changed.

## Validation

Use the smallest meaningful check first:

```bash
go test ./links ./nodes ./core ./runtime/...
```

Run `make test` when the change crosses package boundaries. Run relevant Robot Framework integration tests for deploy/apply/runtime behavior.
