---
title: Don't Check Runtime Names; Call a Runtime Method
impact: HIGH
impactDescription: Keeps docker/podman differences behind the runtime contract
tags: architecture, runtime, polymorphism
---

## Don't Check Runtime Names; Call a Runtime Method

Generic code that special-cases `"docker"` or `"podman"` by name leaks provider differences out of the runtime layer. Put the difference behind a `runtime.ContainerRuntime` method and call it; each provider implements its own behavior.

**Incorrect (provider behavior leaks into generic code):**

```go
if rt.GetName() == "podman" {
	socket = "/run/podman/podman.sock"
} else {
	socket = "/var/run/docker.sock"
}
```

**Correct (ask the runtime):**

```go
socket, err := rt.GetRuntimeSocket()
if err != nil {
	return err
}
```

Reference: `runtime/runtime.go` (`ContainerRuntime.GetRuntimeSocket`), `runtime/docker/docker.go`, `runtime/podman/podman.go`
