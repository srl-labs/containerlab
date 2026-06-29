---
title: Match Tests to Blast Radius
impact: MEDIUM-HIGH
impactDescription: Ensures risky operational and user-facing changes are verified at the right level
tags: tests, validation, robot, docs, schemas
---

## Match Tests to Blast Radius

Choose tests according to risk:

- Pure logic: focused Go unit tests.
- Topology parsing or validation: valid and invalid topology tests plus schema/docs updates.
- Link/endpoint/node/runtime contracts: package tests through interfaces and fakes where useful.
- CLI behavior: command, flag, output, and docs tests.
- Deploy/apply/reconcile/runtime behavior: package tests and relevant Robot Framework integration tests when feasible.

Useful commands:

```bash
go test ./links ./nodes ./core ./runtime/...
make test
CLAB_BIN=$(pwd)/bin/containerlab ./tests/rf-run.sh docker tests/<path to robot file>
```

Mention any tests that were not run and why.
