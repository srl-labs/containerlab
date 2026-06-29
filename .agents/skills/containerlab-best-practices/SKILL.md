---
name: containerlab-best-practices
description: Containerlab engineering rules for writing, reviewing, or refactoring Go code, CLI commands, topology parsing, schemas, docs, nodes, links, runtimes, deploy/destroy/apply/reconcile lifecycle flows, and tests. Use for any Containerlab change where user compatibility, operational safety, package boundaries, extension points, context/error/logging behavior, or docs/schema/test coverage matters.
---

# Containerlab Best Practices

## Overview

Use this skill to make Containerlab changes that fit the existing CLI, topology, runtime, and extension architecture. The link `GetEndpoints` problem is one example of a broader rule: put behavior at the abstraction that owns it, and keep user-facing contracts stable.

## Rule Categories by Priority

| Priority | Category | Impact | Prefix |
|----------|----------|--------|--------|
| 1 | User compatibility | CRITICAL | `cli-` |
| 2 | Operational lifecycle safety | CRITICAL | `lifecycle-` |
| 3 | Architecture and extension boundaries | CRITICAL | `architecture-` |
| 4 | Topology, schema, and docs | HIGH | `topology-` |
| 5 | Node, link, and runtime contracts | HIGH | `contracts-` |
| 6 | Go context, errors, and logging | MEDIUM-HIGH | `go-` |
| 7 | Tests and validation | MEDIUM-HIGH | `tests-` |

## Quick Reference

- `cli-user-compatibility` - Treat CLI flags, topology syntax, labels, output formats, and state as compatibility contracts.
- `lifecycle-state-safety` - Make deploy, destroy, apply, reconcile, cleanup, and rollback behavior idempotent and context-aware.
- `architecture-extension-boundaries` - Keep behavior behind owning interfaces and registries; avoid scattered concrete type lists.
- `topology-schema-docs` - Update topology parsing, schema, docs, examples, and tests together.
- `contracts-links-nodes-runtimes` - Put link behavior on links, kind behavior on nodes, and provider behavior on runtimes.
- `go-context-errors-logging` - Propagate context, wrap errors with actionable detail, and keep logs useful without hiding failures.
- `tests-validation` - Use focused unit tests, package tests, and Robot integration tests according to blast radius.

## How to Use

1. Read the most relevant rule file under `rules/` before editing affected code.
2. For broad reviews, read the compiled guide in `AGENTS.md`.
3. Before adding conditionals for a kind, link type, runtime, or command mode, search for an existing interface, registry, parser, or options structure.
4. Verify with focused package tests. Use integration tests when behavior touches real container lifecycle, networking, filesystem state, or CLI workflows.

## Review Checklist

- Does the change preserve existing topology syntax, CLI flags, labels, file locations, and output formats unless a migration is intentional?
- Is operational behavior idempotent across retries, failed deploys, interrupted apply runs, and cleanup?
- Is behavior implemented at the owning abstraction rather than with broad type switches or kind checks?
- Are docs, schemas, examples, and generated assets updated for user-visible changes?
- Are errors actionable, wrapped, and returned instead of only logged?
- Does the test plan match the blast radius?

## Full Compiled Document

For the complete guide with expanded examples, read `AGENTS.md`.
