---
title: Use Robot Framework Tests for Real Lifecycle Behavior
impact: MEDIUM
impactDescription: Deploy/apply/runtime changes are verified end-to-end
tags: tests, robot, integration, lifecycle
---

## Use Robot Framework Tests for Real Lifecycle Behavior

Changes that touch real container lifecycle, networking, filesystem state, or CLI workflows need an integration test, not only unit tests. Containerlab's integration suite is Robot Framework under `tests/`, run via `tests/rf-run.sh`.

**Incorrect (claim apply works with only a unit test):**

```text
# unit test passes; never deployed a real lab to confirm apply reconciles it
```

**Correct (drive a real lab through the CLI):**

```bash
CLAB_BIN=$(pwd)/bin/containerlab ./tests/rf-run.sh docker tests/01-smoke/29-apply.robot
```

Reference: `tests/rf-run.sh`, `tests/01-smoke/29-apply.robot`
