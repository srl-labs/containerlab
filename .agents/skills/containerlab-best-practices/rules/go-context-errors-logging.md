---
title: Use Context, Errors, and Logs Deliberately
impact: MEDIUM-HIGH
impactDescription: Makes failures diagnosable without hiding them from callers
tags: go, context, errors, logging, diagnostics
---

## Use Context, Errors, and Logs Deliberately

Follow local Go style and keep operational diagnostics actionable.

Guidelines:

- Pass `context.Context` through runtime calls, command execution, namespace work, and blocking operations.
- Return errors instead of only logging them.
- Wrap errors with useful operation details such as lab, node, link, interface, file path, runtime, or command.
- Keep logs structured and consistent with nearby code.
- Avoid global mutable state unless the subsystem already uses it and tests cover it.
- Keep helpers small and behavior-named, especially when they mutate host or runtime state.

Do not add speculative abstractions. Match existing package patterns first.
