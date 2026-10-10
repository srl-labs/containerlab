---
status: new
tags:
  - Clabernetes
---

# Clabernetes (c9s)

[Clabernetes](https://c9s.run), or c9s, schedules containerlab topologies across the nodes of a
Kubernetes cluster. It allows users to run massive labs with hundreds of nodes across the horizontally scalable infrastructure or using familiar Kubernetes interface for lab purposes of any size.

The c9s project maintains [its own documentation site](https://c9s.run/docs) covering installation,
architecture, API, configuration, and operations:

Understanding that Kubernetes workflows may be challenging to grasp from the get-go, Containerlab strives to provide a seamless experience for its user by offering adding C9s as a lab runtime. The same containerlab commands - deploy, destroy, inspect, and others - can be used to manage labs on Kubernetes clusters.  
See the [C9s runtime](runtime.md) for details.

> The former `clabverter` workflow was removed in c9s 0.9. Use the native containerlab runtime for new deployments.
