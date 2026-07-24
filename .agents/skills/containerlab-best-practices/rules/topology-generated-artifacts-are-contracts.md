---
title: Generated Artifacts Are User-Facing Contracts
impact: HIGH
impactDescription: Protects inventory, certs, hosts, ssh config, export, and graphs
tags: topology, artifacts, inventory, certificates
---

## Generated Artifacts Are User-Facing Contracts

Containerlab generates files users depend on: TLS/PKI, Ansible/Nornir inventory, `/etc/hosts` entries, SSH config, topology-data export, and graphs. A change to a name, label, path, or default can silently break automation that consumes these even if no flag or YAML field changed. Update the generator and its tests together.

**Incorrect (change inventory grouping silently):**

```go
group := node.Config().Kind // inventory groups were keyed by ansible group; playbooks break
```

**Correct (preserve the established key; extend additively):**

```go
group := ansibleInventoryGroup(node.Config()) // keep existing grouping; add new groups, don't repurpose
```

Reference: `core/inventory.go`, `core/cert.go`, `core/hostsfile.go`, `core/sshconfig.go`, `core/export.go`, `core/graph.go`
