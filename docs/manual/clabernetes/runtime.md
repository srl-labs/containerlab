# Containerlab runtime

Containerlab can use Clabernetes as a lab runtime. Use `c9s` to select it.
With the c9s runtime selected,
containerlab keeps the familiar CLI workflow, but the actual lab runs in a
kubernetes cluster.

```bash
containerlab --runtime c9s deploy -t topo.clab.yml
```

or, if you prefer environment variables:

```bash
export CLAB_RUNTIME=c9s
containerlab deploy -t topo.clab.yml
```

/// note | Runtime, not converter
This page describes the native `containerlab --runtime c9s` workflow.
The [Quickstart](quickstart.md) still shows the manifest-driven `clabverter`
workflow, which remains useful when you want to generate and apply kubernetes
manifests yourself.
///

## How it works

When the c9s runtime is selected, containerlab does not create local Docker or
Podman containers for the lab nodes. It validates the rendered topology in
memory, using the same fail-closed compiler as Clabernetes, and persists it as
a c9s `Topology` resource:

```yaml
apiVersion: c9s.run/v1alpha1
kind: Topology
metadata:
  name: <lab-name>
  namespace: c9s-<lab-name>
spec:
  definition:
    containerlab: |
      <rendered containerlab topology>
```

The Clabernetes manager compiles that definition into the primary c9s
resources — `Node`, `Link`, and `NodeProfile` — and owns them from then on.
`Node` and `Link` carry the per-node and per-wire state, while `NodeProfile`
resources carry the reusable Kubernetes-side policy (expose, resources,
scheduling, probes, management subnets).

Pass `deploy --no-topology-cr` to skip the `Topology` resource: containerlab
then compiles the topology client-side and creates and reconciles the `Node`,
`Link`, and `NodeProfile` resources directly. No `Topology` object or complete
topology definition is persisted in that mode, so every c9s object stays
bounded as the lab grows.

The Clabernetes manager plans each Node and runs the network OS image **as a
regular container in a regular pod** — there is no nested Docker and no
launcher. A connectivity sidecar in the same pod wires the links and the
management network. Nodes using `network-mode: container:<primary>` become
extra containers in the primary node's pod, and SR-SIM chassis components
become extra containers of their node's pod.

Both deployment modes produce the same lab: inspection, lifecycle, and
destruction discover a lab through its `Topology` resource or, for labs
deployed with `--no-topology-cr`, through the `c9s.run/topologyOwner` label on
its primitive resources.

/// note
The device containers are ordinary Kubernetes containers. `kubectl logs` and
`kubectl exec` against the device pod reach the actual network OS. A
`docker ps` on the machine where you ran the outer `containerlab` command is
not the source of truth for c9s labs.
///

The c9s runtime currently supports the main lab lifecycle and node operations:

| Command | c9s behavior |
| ------- | ------------ |
| `deploy` | creates the `Topology` resource and waits for readiness; `--no-topology-cr` creates `NodeProfile`, `Link`, and `Node` resources directly instead |
| `destroy` | deletes the lab's resources and its containerlab-managed namespace |
| `inspect` | reads Node, Deployment, Pod, and service status |
| `exec` | execs directly into the node's device container |
| `start` | scales node Deployments to `1` |
| `stop` | scales node Deployments to `0` and pauses reconciliation |
| `restart` | restarts node Deployments |
| `save` | runs the node's save lifecycle inside its device container |
| `events` | watches Clabernetes resources and pods |
| `validate` | checks c9s compatibility without creating Kubernetes resources |
| `deploy --dry-run` | compiles and diffs Namespace, ConfigMap, NodeProfile, Link, and Node resources without changing them |

## Requirements

The c9s runtime expects:

- a reachable kubernetes cluster
- kubernetes 1.31 or newer
- Clabernetes CRDs installed in the cluster
- the Clabernetes manager running and watching all lab namespaces
- kubernetes RBAC allowing containerlab to manage the required resources

/// note
Containerlab creates a dedicated namespace for each c9s lab. The kube identity
therefore needs cluster-scoped permission to get, create, and delete namespaces
unless you select an existing namespace with `CLAB_KUBE_NAMESPACE`.
///

## Selecting the cluster

The runtime uses the kubernetes client-go configuration loader. It selects the
kubeconfig in this order:

1. `CLAB_KUBECONFIG`, when set
2. normal client-go kubeconfig loading rules

You can override the kube context with:

```bash
export CLAB_KUBE_CONTEXT=<context-name>
```

To deploy into an existing namespace instead of a per-lab namespace, use the
global `--namespace` option:

```bash
containerlab --runtime c9s --namespace default deploy -t topo.clab.yml
```

For a persistent shell or automation setting, use:

```bash
export CLAB_KUBE_NAMESPACE=default
```

An explicit namespace override must already exist. Containerlab places the lab
resources there but does not create, label, or delete the namespace itself.
`--namespace` takes precedence over `CLAB_KUBE_NAMESPACE`.

/// tip
`CLAB_RUNTIME=c9s` is worth exporting in shell profiles, CI jobs, or
automation environments that always target c9s.
///

## Namespace rules

When `CLAB_KUBE_NAMESPACE` is unset, every lab deployed through the c9s runtime
gets a dedicated namespace named `c9s-<lab-name>`. For example, this command:

```bash
containerlab --runtime c9s deploy -t clos.clab.yml
```

creates the `c9s-clos` namespace and places the lab's `Topology`,
`NodeProfile`, `Node`, `Link`, ConfigMap, Deployment, Pod, Service, and PVC
resources there. Inspect,
exec, start, stop, restart, save, and destroy derive the same namespace from the
lab name.

The namespace is also the topology boundary: links can only connect Nodes in
one namespace, wire identities are namespace-unique, and the lab's management
subnet forms one L2 domain across the namespace.

Containerlab labels namespaces it creates with the runtime and lab owner. A
normal destroy removes such a managed namespace after its lab resources are
gone. If `c9s-<lab-name>` existed before deploy and does not carry those
ownership labels, containerlab uses it but preserves it during destroy.

Set `--namespace` when a shared or externally managed namespace is required:

```bash
containerlab --runtime c9s --namespace default deploy -t clos.clab.yml
```

All single-lab lifecycle commands must use the same flag or environment
override. Because the namespace is then shared, node and other resource names
can conflict between labs.

/// warning | Lab and node names
`c9s-<lab-name>` must be a valid Kubernetes DNS label and cannot exceed 63
characters. This leaves at most 59 characters for the lab name.

Node names are used verbatim as Kubernetes object, Deployment, and Service
names: they must be lowercase RFC 1035 labels (start with a letter, contain
only `a-z`, `0-9`, and `-`). The runtime rejects invalid node names before
creating anything.
///

Some commands intentionally look across namespaces:

- `inspect --all`
- `destroy --all`
- `events`

For these commands, containerlab uses all-namespaces kubernetes listing or
watching. The synthetic c9s container ID includes the namespace so follow-up
actions can still target the right lab:

```text
<namespace>/<lab>/<node>
```

For example:

```text
c9s-clos/clos/srl1
```

Primitive-only labs created outside containerlab (for example with
`clabverter --emit-crs`) are also manageable when their Nodes, Links, and
NodeProfiles carry the common `c9s.run/topologyOwner=<lab-name>` label.
Containerlab uses that label as the lab boundary for list, inspect, lifecycle,
events, and destroy operations.

## Deploy

Deploying with the c9s runtime looks like a regular containerlab deployment:

```bash
containerlab --runtime c9s deploy -t topo.clab.yml
```

The deploy flow is:

1. containerlab parses and checks the topology file.
2. It stages local files into per-node ConfigMaps.
3. It compiles the final topology in memory into self-contained Nodes, Links,
   and NodeProfiles, rejecting anything the c9s runtime cannot realize.
4. It creates the `Topology` resource; the Clabernetes manager compiles it
   into the same NodeProfile, Link, and Node resources and owns them. With
   `--no-topology-cr`, containerlab instead creates profiles and Links first,
   then Nodes, so the manager never plans a workload against a partial wiring
   view.
5. It waits until all Nodes report ready, streaming per-node lifecycle
   progress (planning, file preparation, image pulls, connectivity, container
   startup).
6. It inspects the resulting kubernetes state and prints the node table.

The runtime enables c9s status probes on the generated `NodeProfile`. A Node is
ready when its device plan is applied, its files are prepared, its connectivity
sidecar converged, and every device application container is running and ready.
When an image defines a healthcheck, c9s translates it into the container's
startup and readiness probes. Containerlab does not guess readiness ports or
special-case kinds and images, so the same baseline works for arbitrary
containerlab nodes.

Readiness is atomic for a `network-mode: container:<primary>` group: the group
shares one pod, and every member Node reports ready only while all of its own
containers satisfy the readiness contract.

For an image without a healthcheck, this is a process-level signal: a running
network OS may still be booting services or converging protocols. Use an
image-defined healthcheck or an explicit c9s TCP/SSH probe when the lab
requires application-level readiness.

The c9s runtime uses a ten-minute timeout by default because large NOS images
can take several minutes to load and boot. Override it when a lab needs a
different startup window, for example:

```bash
containerlab --runtime c9s --timeout 10m deploy -t topo.clab.yml
```

Deterministic planning failures (an unsupported construct that only the device
planner can detect, a registry that keeps refusing image metadata) abort the
wait early with the controller's message instead of running out the timeout.

`deploy --reconfigure` first deletes all resources in the existing lab and
then deploys them again.

/// warning | Node filtering
`deploy --node-filter` is not supported with the c9s runtime. Clabernetes owns
reconciliation of the complete set of Node and Link resources. Deploy the full
topology, then use node filtering with commands such as `start`, `stop`,
`restart`, `exec`, or `save` after the lab exists.
///

Deploy reconciles an existing lab in place. A Topology-owned lab — deployed by
containerlab, `kubectl`, or `clabverter` — has its `Topology` definition
updated in place and the controller converges the compiled resources on it.
For a lab deployed with `--no-topology-cr`, containerlab creates, updates, or
removes the corresponding c9s `Node`, `Link`, `NodeProfile`, and staged
`ConfigMap` resources directly. Deploying such a lab again without the flag
creates the `Topology` resource and the controller adopts the label-matched
primitive resources. The reverse is rejected: a Topology-owned lab cannot be
deployed with `--no-topology-cr` because the controller would revert every
direct write — destroy the lab first when you want to switch it to direct
management.

Use `deploy --reconfigure` when you explicitly want to delete and recreate every resource. Use a
different lab name or namespace when you want a separate lab:

```bash
containerlab --runtime c9s --name <new-lab-name> deploy -t topo.clab.yml
```

A failure while waiting for a newly created lab to become ready rolls back the
resources and managed namespace created by that deployment. A timeout while
reconciling a lab that already existed retains the lab for diagnosis and a
later corrective reconciliation.

### Validation and dry-run

Both commands use the same c9s preparation path as deploy, including extended-link
normalization, compatibility checks, and local-file staging checks:

```bash
containerlab --runtime c9s validate -t topo.clab.yml
containerlab --runtime c9s deploy --dry-run -t topo.clab.yml
containerlab --runtime c9s deploy --dry-run --format json -t topo.clab.yml
```

`validate` reports whether the topology can be represented by the c9s runtime without reading
or changing lab resources. Compilation is fail-closed: every source construct c9s cannot
preserve is reported as an error, with its field and line, in one pass. A small set of
lossy-but-safe fields (shared management network selection, pinned host-side ports) is accepted
with a warning. `deploy --dry-run` additionally reads the selected cluster and reports the
exact create, update, and delete plan for the resources containerlab itself would change:
Namespace, ConfigMap, and the `Topology` — or, with `--no-topology-cr`, the NodeProfile, Link,
and Node resources. An empty `changes` list means the deployed resources already conform.

## Inspect

Inspect works with a topology file, a lab name, or all known c9s labs:

```bash
containerlab --runtime c9s inspect -t topo.clab.yml
containerlab --runtime c9s inspect --name clos
containerlab --runtime c9s inspect --all
```

For c9s labs, inspect reads kubernetes resources instead of local container
runtime state. It collects the lab name, namespace, aggregate state, node
readiness, node kind and image, and the address a user reaches the node at: the
LoadBalancer address when Clabernetes exposes one, otherwise the node's
allocated management address.

/// note
`inspect --all` groups c9s Nodes by `c9s.run/topologyOwner` across all
namespaces. A single-lab inspect uses its canonical `c9s-<lab-name>` namespace.
///

Useful kubernetes checks for the same state are:

```bash
kubectl -n <namespace> get node.c9s.run,link.c9s.run,nodeprofile.c9s.run,deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```

## Exec

`exec` runs the user command in the node's device container:

```bash
containerlab --runtime c9s exec -t topo.clab.yml --cmd 'ip addr'
```

Under the hood, containerlab:

1. resolves the target nodes from the Clabernetes lab state
2. finds the device pod for each node (grouped nodes share the primary node's pod)
3. selects the node's application container from the Node status — for an
   SR-SIM chassis this is the active-preferred CPM component
4. uses kubernetes pod exec directly into that container

/// note
The command executes in the actual device container. RBAC must allow
`pods/exec`, and the device pod must be running.
///

If any selected command returns nonzero, or pod exec itself fails, the outer
`containerlab exec` also returns nonzero. Successful and failed results that
were received are still printed, so automation can use both the output and the
process exit status.

## Start, stop, and restart

Node lifecycle commands operate on the kubernetes Deployments created by
Clabernetes.

```bash
containerlab --runtime c9s stop -t topo.clab.yml
containerlab --runtime c9s start -t topo.clab.yml
containerlab --runtime c9s restart -t topo.clab.yml
```

`stop` sets the Clabernetes ignore-reconcile label on the selected `Node`
resources, then scales their Deployments to `0`:

```text
c9s.run/ignoreReconcile=true
```

The label prevents the Clabernetes manager from immediately reconciling the
nodes back to the running state. A stopped node's links go carrier-down on
every peer, exactly like unplugging a cable.

`start` scales the selected Deployments back to `1` and clears the
corresponding Node labels. A grouped secondary shares lifecycle with its
primary node's Deployment.

`restart` patches each selected Deployment with a restart annotation and waits
for it to become ready:

```text
kubectl.kubernetes.io/restartedAt=<utc timestamp>
```

## Save

Saving a c9s lab runs each node's save lifecycle inside its device container:

```bash
containerlab --runtime c9s save -t topo.clab.yml
```

For each selected node, containerlab derives the node's typed lifecycle
command from its device container and runs it with the `Save` phase. That
executes the same containerlab kind `save` implementation used by the local
runtimes, against the live device. Nodes without a save-capable container
(for example plain linux nodes) are skipped.

`save --copy` streams the files the save produced back to the machine where
the outer containerlab command runs:

```bash
containerlab --runtime c9s save -t topo.clab.yml --copy ./startup-configs
```

The copied files follow the normal containerlab copy layout:

```text
<copy-destination>/<lab-dir>/<node>/<saved-files>
```

For example:

```text
./startup-configs/clab-clos/srl1/config/config.json
```

/// note
`save` still depends on node kind support. If a node kind does not produce saved
files, the c9s runtime has nothing to copy for that node.
///

## Events

The c9s runtime can stream Node, Topology, pod, and interface-stat events:

```bash
containerlab --runtime c9s events --format json
containerlab --runtime c9s events --initial-state
containerlab --runtime c9s events --interface-stats --format json
```

For c9s, events do not come from Docker events on the outer host. Containerlab
watches:

- c9s `Node` resources carrying `c9s.run/topologyOwner`
- `Topology` resources
- Pods labeled with `c9s.run/topologyOwner`

With `--initial-state`, the stream starts with synthetic events for the current
c9s node state and then continues with live watches.

With `--interface-stats`, containerlab periodically reads `/proc/net/dev`
inside each node's device container. All containers of a device pod share one
network namespace, so the statistics cover the node's pod interfaces.

/// note | Polling, not netlink
c9s interface statistics are sampled periodically. The first sample seeds the
counters, and rates start with the second sample. Short-lived changes between
samples can be missed.
///

## Lab artifacts

With c9s, the primary artifacts are kubernetes resources and the per-node plan
artifacts inside the device pods.

The primary kubernetes resources are:

```bash
kubectl -n <namespace> get node.c9s.run,link.c9s.run,nodeprofile.c9s.run \
  -l c9s.run/topologyOwner=<lab>
```

Related resources are selected with Clabernetes labels:

```bash
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```

To find one node's device pod:

```bash
kubectl -n <namespace> get pod -l c9s.run/direct-workload=<node>
```

`kubectl logs` and `kubectl exec` on that pod default to the node's primary
application container. Add `-c` to reach a specific chassis component, the
`clabwire` connectivity sidecar, or the `planner` preparation container.

Local startup configurations, licenses, bind sources, `env-files`,
`extras.srl-agents`, and `extras.ceos-copy-to-flash` paths are copied into
per-node ConfigMaps and staged at the paths the node expects. An inline
`startup-config` is staged as a partial configuration and merged over the
kind's default config, exactly like the local runtimes. Each staged file is
currently limited to 950 KB. These projections are snapshots taken at deploy
time, not mutable host bind mounts; run deploy again after changing a source
file.

## RBAC requirements

The kube identity used by the outer containerlab process must be able to:

- get, create, and delete namespaces when using automatic per-lab namespaces
- create, get, list, watch, update, and delete c9s `Node` resources
- create, get, list, watch, and delete c9s `Link` and `NodeProfile` resources
- get, list, watch, and delete `Topology` resources when Topology-based labs
  must remain manageable
- list and watch Pods
- list, get, and update Deployments
- create, get, list, update, and delete ConfigMaps used for local files
- exec into device Pods with `pods/exec`

Useful checks:

```bash
kubectl auth can-i get namespaces
kubectl auth can-i create namespaces
kubectl auth can-i delete namespaces
kubectl auth can-i create nodes.c9s.run -n <namespace>
kubectl auth can-i list nodes.c9s.run -n <namespace>
kubectl auth can-i update nodes.c9s.run -n <namespace>
kubectl auth can-i delete nodes.c9s.run -n <namespace>
kubectl auth can-i create links.c9s.run -n <namespace>
kubectl auth can-i delete links.c9s.run -n <namespace>
kubectl auth can-i create nodeprofiles.c9s.run -n <namespace>
kubectl auth can-i delete nodeprofiles.c9s.run -n <namespace>
kubectl auth can-i list pods -n <namespace>
kubectl auth can-i watch pods -A
kubectl auth can-i create pods/exec -n <namespace>
kubectl auth can-i update deployments -n <namespace>
```

## Troubleshooting

### No kubeconfig or wrong context

Typical symptoms:

```text
failed to init the lab runtime: failed to load Kubernetes client config: ...
```

Check:

```bash
kubectl config current-context
kubectl cluster-info
echo "$CLAB_KUBECONFIG"
echo "$CLAB_KUBE_CONTEXT"
```

Fix the kubeconfig, context, or cluster access, then run the containerlab command
again.

### Namespace creation fails

Typical symptoms:

```text
failed to create c9s namespace "c9s-<lab>": ...
```

Check:

```bash
kubectl auth can-i get namespaces
kubectl auth can-i create namespaces
kubectl get namespace c9s-<lab>
```

Grant the selected kube identity permission to manage namespaces. You may also
pre-create the canonical namespace; containerlab will use it and will not
delete it unless it carries containerlab's runtime and lab-owner labels:

```bash
kubectl create namespace c9s-<lab>
```

With `--namespace` or `CLAB_KUBE_NAMESPACE` set, containerlab requires that
namespace to exist and never creates or deletes it:

```bash
kubectl get namespace default
containerlab --runtime c9s --namespace default deploy -t topo.clab.yml
```

### CRDs are missing

The c9s runtime talks to:

```text
nodes.c9s.run
links.c9s.run
nodeprofiles.c9s.run
```

Typical symptoms:

```text
the server could not find the requested resource
```

Check:

```bash
kubectl api-resources | grep -i c9s
kubectl get crd nodes.c9s.run links.c9s.run nodeprofiles.c9s.run
```

Install Clabernetes and its CRDs before using `--runtime c9s`.

### Manager is not reconciling

The primitive resources may exist, but no node Deployments or Pods appear.

Check:

```bash
kubectl get pods -A | grep -i clabernetes
kubectl -n <namespace> get node.c9s.run,link.c9s.run,nodeprofile.c9s.run \
  -l c9s.run/topologyOwner=<lab>
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```

If deploy waits until timeout, check the Clabernetes manager logs and verify
that it watches the namespace where the primitive resources were created.

### Nodes do not become ready

Deploy waits for every Node to report `status.readiness=ready` and streams the
per-node lifecycle phase while waiting. The Node conditions name the phase a
node is stuck in:

```bash
kubectl -n <namespace> get node.c9s.run <node> -o wide
kubectl -n <namespace> describe node.c9s.run <node>
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```

`PlanApplied` covers device planning and image metadata resolution,
`Prepared` the file staging init container, `ConnectivityReady` the
connectivity sidecar, and `ContainersReady` the device containers themselves.
Common causes include bad topology data, image pull failures, missing pull
secrets, or unsupported node settings.

### Inspect shows no containers

For c9s, `inspect` looks for c9s Nodes grouped by
`c9s.run/topologyOwner` (and Topologies), not local Docker containers.

Check:

```bash
containerlab --runtime c9s inspect --all
kubectl get nodes.c9s.run -A -l c9s.run/topologyOwner
```

If `docker ps` on the outer host is empty, that is perfectly normal for c9s.
The node containers live in the cluster.

### Exec, save, or stats cannot reach a node

`exec`, `save`, `save --copy`, and `events --interface-stats` need pod exec
into the device pod.

Check:

```bash
kubectl -n <namespace> get pod -l c9s.run/direct-workload=<node> -o wide
kubectl auth can-i create pods/exec -n <namespace>
kubectl -n <namespace> exec -it deploy/<node> -- sh
```

## Current limitations

The c9s runtime is not a complete drop-in replacement for the local Docker or
Podman runtime. Several containerlab features assume local containers, local
network namespaces, or direct access to the host container runtime.

The runtime divides compatibility into three categories:

- Native-equivalent: point-to-point links at any MTU (the fabric wire adapts to
  the cluster network automatically), carrier propagation, lifecycle
  operations, node configuration, staged startup files, exec, save, inspect,
  management addressing, and group-atomic readiness.
- Documented c9s semantics: Kubernetes Service/LoadBalancer management access,
  `host:` endpoints materialized in the device pod's worker network namespace,
  the c9s cross-Pod fabric wire, and ConfigMap-backed local files.
- Accepted with warnings: management network selection fields that have no
  Kubernetes meaning (`mgmt.network`, `mgmt.bridge`, `mgmt.mtu`,
  `mgmt.external-access`) and pinned host-side ports (`8080:80` keeps the
  node-side port and drops the host half).
- Rejected with a named error before anything is created: every other field
  c9s cannot preserve (for example `cpu-set`, `stages`, link `labels`/`vars`,
  `mgmt-net:` and `macvlan` endpoints), bridge/ovs-bridge/host/ext-container
  pseudo-nodes, node names that are not valid Kubernetes names, and commands
  or flags with no c9s implementation.

Management access behaves like containerlab: every node gets an allocated
management address (honoring the topology `mgmt` subnet and static
`mgmt-ipv4`/`mgmt-ipv6` addresses), and the management subnet is one L2 domain
across the lab namespace, so devices reach each other's management addresses
directly. From outside the cluster, Kubernetes Services (LoadBalancer by
default), pod IPs, and DNS are the access paths. The `--network`,
`--ipv4-subnet`, and `--ipv6-subnet` flags remain rejected because they imply
native Docker-network semantics.

Known command differences:

- `deploy --node-filter` and `destroy --node-filter` are rejected; a filtered
  destroy never falls through to whole-lab deletion.
- Deploy flags `--graph`, `--max-workers`, `--skip-post-deploy`,
  `--skip-labdir-acl`, `--export-template`, `--restore`, and `--restore-all`
  are rejected.
- Destroy flags `--graceful`, `--keep-mgmt-net`, and `--max-workers` are
  rejected. `--cleanup` removes the local lab directory in addition to the
  cluster resources.
- Local Docker commands on the outer host are not authoritative for c9s labs.
- `inspect interfaces` is rejected. Host-side `tc` or netem operations do not
  have the same local namespace access they have with Docker labs.
- `graph` and `tools` commands operate on local containers and host networking
  and are rejected with an error when the `c9s` runtime is selected.
- Per-node `runtime: docker` or `runtime: podman` is not the same as selecting
  the global `c9s` lab runtime.
- A lab name maps to one canonical `c9s-<lab-name>` namespace in the selected
  cluster.

/// note
Use kubernetes state as the source of truth for c9s labs:

```bash
kubectl get nodes.c9s.run -A -l c9s.run/topologyOwner
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```

///
