# Installation

Install the c9s manager and CRDs by following the maintained
[c9s installation guide](https://c9s.run/docs/installation).

/// warning | Version compatibility
The c9s runtime in containerlab 0.80+ targets the direct-runtime API introduced
in c9s 0.9. Launcher-based c9s 0.8 and older releases are not compatible.

c9s 0.9 has no in-place upgrade path from an older release. Uninstall the old
release, update any retained manifests, and install c9s again as described in
the [c9s 0.9 release notes](https://c9s.run/docs/release-notes/0.9).
Existing launcher-era lab resources are not migrated.
///

After the current c9s manager is running, containerlab can submit a compatible
topology through the Kubernetes API:

```bash
containerlab --runtime c9s deploy -t topo.clab.yml
```

The command creates a lab namespace, stages local files as ConfigMaps, creates a
c9s `Topology` resource, and waits for the c9s controllers to create the
`NodeProfile`, `Link`, `Node`, and workload resources. It requires a working
kubeconfig and sufficient Kubernetes permissions; installing c9s alone does
not grant those permissions to the local user.

See the [Containerlab runtime](runtime.md) guide for kubeconfig selection,
namespace handling, required permissions, and the complete command workflow.
