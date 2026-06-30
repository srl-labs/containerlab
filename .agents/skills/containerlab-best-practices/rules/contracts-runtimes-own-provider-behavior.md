---
title: Runtimes Own Provider Behavior
impact: HIGH
impactDescription: Keeps docker/podman differences behind ContainerRuntime
tags: contracts, runtime, docker, podman
---

## Runtimes Own Provider Behavior

Container, network, label, and namespace operations, plus every docker/podman API difference, belong behind `runtime.ContainerRuntime`. Generic code calls the interface; it never reaches for a provider-specific client or branches on the provider name.

**Incorrect (generic code talks to a provider client directly):**

```go
cli, _ := dockerC.NewClientWithOpts(dockerC.FromEnv, dockerC.WithAPIVersionNegotiation())
cli.ContainerStart(ctx, id, container.StartOptions{})
```

**Correct (go through the runtime contract):**

```go
if err := rt.StartContainer(ctx, id, node); err != nil { return err }
```

Reference: `runtime/runtime.go`, `runtime/docker/docker.go`, `runtime/podman/podman.go`
