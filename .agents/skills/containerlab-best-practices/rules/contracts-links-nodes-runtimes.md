---
title: Put Behavior on Links, Nodes, and Runtimes
impact: HIGH
impactDescription: Prevents generic workflows from becoming concrete type dispatch tables
tags: links, nodes, runtimes, contracts, polymorphism
---

## Put Behavior on Links, Nodes, and Runtimes

Generic code should orchestrate through contracts.

Links own:

- Endpoint semantics through `GetEndpoints()` and any narrow link-owned optional interfaces.
- Deploy/remove behavior, MTU, vars, and link-specific apply behavior.

Nodes own:

- Endpoint normalization through `AddEndpoint`.
- Interface validation and indexing.
- Config generation, deploy hooks, health, and lifecycle policy.

Runtimes own:

- Docker/Podman API differences.
- Container, network, label, and namespace operations.
- Provider-specific behavior behind `ContainerRuntime`.

Do not add central lists of concrete link structs, node kinds, or runtime names in generic flows. Adding a new type should mostly require implementation, registration/parser/schema/docs as needed, and tests.
