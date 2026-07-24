---
name: containerlab-best-practices
description: Containerlab engineering rules for writing, reviewing, or refactoring Go code, CLI commands, topology parsing, schemas, docs, nodes, kinds, links, endpoints, runtimes, deploy/destroy/apply/reconcile lifecycle flows, generated artifacts, and tests. Use for any Containerlab change where user compatibility, operational safety, package boundaries, extension points, context/error/logging behavior, or docs/schema/test coverage matters.
license: MIT
metadata:
  author: containerlab
  version: "2.0.0"
---

# Containerlab Best Practices

27 rules across 7 categories, prioritized by impact. One rule underlies the rest: put behavior on the abstraction that owns it, and keep user-facing contracts stable. The link `GetEndpoints` story is just the easiest illustration — the same rule spans every subsystem, and the largest is node kinds (60+), not links.

Containerlab is much more than links and apply. A change can touch: **node kinds** (`nodes/`, behind `nodes.Node`, registered via `Register(*NodeRegistry)`), **runtimes** (`runtime/` docker/podman behind `ContainerRuntime`), **links/endpoints** (`links/`), **topology + config inheritance** (`types/`, `core/config.go`), **lifecycle + ordering** (`core/`, `core/dependency_manager/`), **CLI + tools** (`cmd/`, `tools_*`), and **generated artifacts** (cert/PKI, inventory, `/etc/hosts`, ssh config, export, graphs).

## When to Apply

- Adding or changing a node kind, link type, endpoint type, or runtime.
- Touching deploy, destroy, apply, reconcile, restart, save, or cleanup.
- Changing CLI commands, flags, topology YAML, defaults, labels, or generated files.
- Reviewing or refactoring Go code in `links`, `nodes`, `core`, `runtime`, or `cmd`.
- Writing tests for any of the above.

## Rule Categories by Priority

| Priority | Category | Impact | Prefix |
|----------|----------|--------|--------|
| 1 | User compatibility | CRITICAL | `cli` |
| 2 | Operational lifecycle safety | CRITICAL | `lifecycle` |
| 3 | Architecture and extension boundaries | CRITICAL | `architecture` |
| 4 | Link, endpoint, node, and runtime contracts | HIGH | `contracts` |
| 5 | Topology, schema, and docs | HIGH | `topology` |
| 6 | Go context, errors, and logging | MEDIUM-HIGH | `go` |
| 7 | Tests and validation | MEDIUM-HIGH | `tests` |

## Quick Reference

### 1. User Compatibility (`cli`) — CRITICAL
- `cli-flags-and-syntax-are-contracts` — Don't rename/repurpose flags or topology fields; add new ones.
- `cli-additive-over-default-changes` — Prefer opt-in over flipping a default in place.
- `cli-labels-and-state-are-stable` — Keep labels, lab dirs, and generated file names stable.

### 2. Operational Lifecycle Safety (`lifecycle`) — CRITICAL
- `lifecycle-idempotent-cleanup` — Treat already-gone resources as success.
- `lifecycle-thread-context` — Propagate the caller's context through deploy/destroy/apply.
- `lifecycle-dryrun-matches-execution` — Plan and execute share one decision function.
- `lifecycle-respect-dependency-order` — Order deploys via the dependency manager, not sleeps.

### 3. Architecture and Extension Boundaries (`architecture`) — CRITICAL
- `architecture-call-the-interface` — Call the interface method; don't type-switch.
- `architecture-promote-behavior-to-interface` — Missing behavior belongs on the owning interface.
- `architecture-no-kind-switch` — Never branch on `node.Config().Kind`.
- `architecture-no-runtime-name-check` — Don't check runtime names; call a runtime method.
- `architecture-register-dont-branch` — New kinds/runtimes register; they don't add a central case.
- `architecture-type-switch-only-at-boundaries` — Type assertions/switches only at parser/factory, third-party adapter, or compatibility boundaries.

### 4. Link, Endpoint, Node, and Runtime Contracts (`contracts`) — HIGH
- `contracts-links-own-endpoints` — Read endpoints through `links.Link`, not concrete fields.
- `contracts-endpoints-own-moves` — Endpoints own namespace moves and activation.
- `contracts-nodes-own-kind-behavior` — Kind logic lives on `nodes.Node` methods.
- `contracts-runtimes-own-provider-behavior` — Docker/podman differences stay behind `ContainerRuntime`.

### 5. Topology, Schema, and Docs (`topology`) — HIGH
- `topology-parse-resolve-validate` — Keep parse, resolve, validate, and deploy separate.
- `topology-config-inheritance` — Read effective values through defaults → kinds → groups → node.
- `topology-keep-schema-docs-aligned` — Struct + schema + docs + example + tests, together.
- `topology-generated-artifacts-are-contracts` — Inventory/certs/hosts/ssh/export/graph are contracts.

### 6. Go Context, Errors, and Logging (`go`) — MEDIUM-HIGH
- `go-thread-context` — Accept and forward `ctx`; don't synthesize a new one.
- `go-return-and-wrap-errors` — Return and wrap errors; don't only log them.
- `go-no-speculative-abstractions` — Match local patterns; abstract only on a real second caller.

### 7. Tests and Validation (`tests`) — MEDIUM-HIGH
- `tests-match-blast-radius` — Scale coverage with risk; say what you skipped.
- `tests-through-interfaces` — Test contracts with fakes, not a real runtime.
- `tests-robot-for-lifecycle` — Real lifecycle changes get a Robot Framework test.

## How to Use

1. Read the most relevant rule file under `rules/` before editing affected code. Each rule is one focused do/don't with an Incorrect and Correct example.
2. The filename prefix selects the section; see `rules/_sections.md` for section impact and ordering, and `rules/_template.md` to add a rule.
3. Before adding a conditional for a kind, link type, endpoint type, runtime, or command mode, search for an existing interface, registry, parser, or options struct.
4. For a broad review, read the compiled `AGENTS.md`.

## Full Compiled Document

For the complete guide with the subsystem map and every rule expanded, read `AGENTS.md`.
