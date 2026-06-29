---
title: Keep Lifecycle Operations Safe
impact: CRITICAL
impactDescription: Reduces deploy, destroy, apply, and reconcile regressions on real host state
tags: lifecycle, deploy, destroy, apply, reconcile, cleanup
---

## Keep Lifecycle Operations Safe

Deploy, destroy, apply, reconcile, start, stop, restart, save, and cleanup touch real containers, namespaces, links, files, and host state.

Design these paths to be:

- Idempotent when resources already exist or are already gone.
- Context-aware when operations can block or call a runtime.
- Explicit about whether nodes and links are created, deleted, restarted, recreated, or live-updated.
- Tolerant of partial failure during cleanup, while still returning actionable errors.
- Consistent between dry-run planning and actual execution.

Avoid helper functions with hidden side effects. If a helper mutates runtime or host state, make the name and tests show that.
