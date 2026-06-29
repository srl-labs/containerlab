# Containerlab Best Practices

**Version 1.1.0**  
Containerlab agent guidance  
June 2026

> This document is for agents maintaining, reviewing, or refactoring Containerlab. It covers CLI compatibility, topology behavior, lifecycle safety, architecture boundaries, runtimes, nodes, links, endpoints, tests, and docs.

## Abstract

Containerlab is a CLI that manages real labs, containers, namespaces, links, generated files, and user topology definitions. Good changes preserve user contracts, keep operational flows recoverable, and place behavior at the abstraction that owns it. The link endpoint type-switch problem is one concrete example of the broader rule: generic code should orchestrate through contracts, not rediscover implementation details.

## Table of Contents

1. [User Compatibility](#1-user-compatibility) - **CRITICAL**
2. [Operational Lifecycle Safety](#2-operational-lifecycle-safety) - **CRITICAL**
3. [Architecture and Extension Boundaries](#3-architecture-and-extension-boundaries) - **CRITICAL**
4. [Topology, Schema, and Docs](#4-topology-schema-and-docs) - **HIGH**
5. [Link, Endpoint, Node, and Runtime Contracts](#5-link-endpoint-node-and-runtime-contracts) - **HIGH**
6. [Go Context, Errors, and Logging](#6-go-context-errors-and-logging) - **MEDIUM-HIGH**
7. [Tests and Validation](#7-tests-and-validation) - **MEDIUM-HIGH**

## 1. User Compatibility

**Impact: CRITICAL**

Treat user-facing behavior as a contract. This includes CLI commands and flags, topology YAML syntax, defaults, labels, state files, generated inventory, generated configs, output formats, and docs examples.

Avoid silent breaking changes. If behavior must change, make the migration explicit with validation, docs, and tests.

Before changing user-visible behavior, check:

- `cmd/` for command and flag behavior.
- `types/`, `core/config.go`, and topology parsing for config defaults.
- `schemas/`, `docs/`, and `lab-examples/` for user-facing contract updates.
- Existing tests that assert CLI output, topology parsing, or generated files.

Prefer additive options and clear validation errors over changing defaults.

## 2. Operational Lifecycle Safety

**Impact: CRITICAL**

Deploy, destroy, apply, reconcile, cleanup, save, start, stop, and restart operate on real host/container state. Make these paths idempotent, context-aware, and safe under partial failure.

Guidelines:

- Pass `context.Context` through runtime, node, and link operations.
- Return errors with enough context for the user to fix the lab.
- Make cleanup tolerate resources that are already gone.
- Avoid helper functions with hidden side effects unless the name and tests make the side effect obvious.
- Keep dry-run and actual reconcile decisions consistent.
- Preserve existing state-file and generated-artifact compatibility unless migration is intentional.

When changing lifecycle code, identify whether each affected node or link is created, deleted, restarted, recreated, or live-updated.

## 3. Architecture and Extension Boundaries

**Impact: CRITICAL**

Containerlab is extended through registries, interfaces, and package boundaries. Generic code should orchestrate; concrete implementations should own their behavior.

Use existing abstractions first:

- `links.Link`, `links.RawLink`, `links.Endpoint`, and endpoint ownership/move interfaces.
- `nodes.Node` and narrow optional node interfaces.
- `runtime.ContainerRuntime`.
- Node and runtime registries.
- Topology resolve/validation layers.

**Incorrect: generic code re-lists link implementations**

```go
func endpoints(l links.Link) []links.Endpoint {
	switch link := l.(type) {
	case *links.LinkVEth:
		return link.Endpoints
	case *links.LinkDummy:
		return link.Endpoints
	default:
		return nil
	}
}
```

**Correct: delegate to the contract**

```go
func endpoints(l links.Link) []links.Endpoint {
	return l.GetEndpoints()
}
```

Use concrete type switches only at explicit boundaries:

- YAML parsing and legacy shorthand translation.
- Central registration or factory wiring.
- Compatibility shims where the old format is type-encoded.
- Third-party library adapters.

If an interface is missing behavior, add the smallest owning-domain method or a narrow optional interface near the caller.

## 4. Topology, Schema, and Docs

**Impact: HIGH**

Topology syntax is a public API. Keep parsing, resolution, validation, schema, docs, and examples aligned.

For topology changes:

- Parse into raw structures.
- Resolve into domain objects.
- Validate through node/link/domain methods.
- Deploy through domain contracts.
- Update `schemas/`, `docs/`, and `lab-examples/` when users can see or configure the behavior.

Do not let deployment code parse YAML strings. Do not put deploy behavior on raw topology structs. Do not accept invalid input silently if an actionable validation error is possible.

## 5. Link, Endpoint, Node, and Runtime Contracts

**Impact: HIGH**

Put behavior where it belongs:

- Links own link semantics such as endpoint lists, deploy/remove behavior, MTU, vars, and link-specific apply endpoints.
- Endpoints own endpoint-local behavior such as interface identity, runtime-discovered state, namespace moves, activation, and link back-references.
- Nodes own kind-specific endpoint normalization, interface validation, deploy hooks, config generation, health, and lifecycle policy.
- Runtimes own Docker/Podman-specific API differences, labels, container/network operations, and provider behavior.

For links, prefer `Link.GetEndpoints()` or link-owned optional interfaces over central concrete type switches.

For endpoints, prefer `Endpoint` methods and endpoint-owner contracts over code that reaches into concrete endpoint structs.

For nodes, prefer methods such as `AddEndpoint`, `CheckInterfaceName`, `CalculateInterfaceIndex`, `DeployEndpoints`, `PostDeploy`, and `LinkApplyMode` over generic `Config().Kind` checks.

For runtimes, generic code should call `ContainerRuntime` methods instead of checking runtime names.

Adding a new kind, link type, endpoint type, or runtime should require localized edits: implementation, registry/parser/schema/docs as needed, and focused tests.

## 6. Go Context, Errors, and Logging

**Impact: MEDIUM-HIGH**

Follow the existing Go style and keep operational diagnostics useful.

Guidelines:

- Thread `context.Context` through work that can block, talk to a runtime, touch namespaces, or run commands.
- Return errors instead of only logging them. Log when useful, but do not hide failure from callers.
- Wrap errors with operation, node/link/lab name, interface name, or file path when that helps the user act.
- Keep logs structured and consistent with nearby code.
- Avoid package-global mutable state unless the existing subsystem already uses it and tests cover it.
- Keep helper functions small and named after behavior, especially when they mutate host/container state.

Do not add broad abstractions for one-off code. Match existing local patterns before inventing new ones.

## 7. Tests and Validation

**Impact: MEDIUM-HIGH**

Let test coverage scale with risk:

- Narrow pure logic change: focused Go unit test.
- Topology parse or schema change: parsing tests plus schema/docs/example update.
- Link/endpoint/node contract change: interface-level tests with fakes where useful.
- Apply/reconcile/deploy/runtime change: package tests plus the relevant Robot Framework integration test when feasible.
- CLI behavior change: command/flag/output tests and docs update.

Useful commands:

```bash
go test ./links ./nodes ./core ./runtime/...
make test
CLAB_BIN=$(pwd)/bin/containerlab ./tests/rf-run.sh docker tests/<path to robot file>
```

Useful architecture review search:

```bash
rg -n "\\.\\(type\\)|type switch|Config\\(\\)\\.Kind|GetType\\(\\)|runtimeName|Endpoint|cobra.Command" links nodes core runtime cmd
```

Treat grep results as review prompts, not automatic failures. Parser, registry, and adapter boundaries can legitimately inspect types.
