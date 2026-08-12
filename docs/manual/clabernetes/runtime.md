# Containerlab runtime

Containerlab can use Clabernetes as a lab runtime. With the c9s runtime selected,
containerlab keeps the familiar CLI workflow, but the actual lab runs in a
kubernetes cluster.

```bash
containerlab --runtime clabernetes deploy -t topo.clab.yml
```

or, if you prefer environment variables:

```bash
export CLAB_RUNTIME=clabernetes
containerlab deploy -t topo.clab.yml
```

/// note | Runtime, not converter
This page describes the native `containerlab --runtime clabernetes` workflow.
The [Quickstart](quickstart.md) still shows the manifest-driven `clabverter`
workflow, which remains useful when you want to generate and apply kubernetes
manifests yourself.
///

## How it works

When the c9s runtime is selected, containerlab does not create local Docker or
Podman containers for the lab nodes. It compiles the rendered topology in
memory, using the same compiler as Clabernetes, and creates the primary c9s
resources directly:

```yaml
apiVersion: c9s.run/v1alpha1
kind: Node
metadata:
  name: <node-name>
  namespace: c9s-<lab-name>
  labels:
    c9s.run/topologyOwner: <lab-name>
spec:
  kind: <containerlab-kind>
  image: <node-image>
---
apiVersion: c9s.run/v1alpha1
kind: Link
metadata:
  name: <link-name>
  namespace: c9s-<lab-name>
spec:
  endpointA: {nodeName: <node-a>, interfaceName: <interface-a>}
  endpointB: {nodeName: <node-b>, interfaceName: <interface-b>}
```

`LauncherProfile` resources carry reusable launcher policy. No `Topology`
object or complete topology definition is persisted, so every c9s object stays
bounded as the lab grows. Each launcher pod runs containerlab inside the pod
and starts the real node container there. Nodes using `network-mode:
container:<primary>` share the primary node's launcher pod.

For backward compatibility, all-namespace inspection and destruction still
discover older labs that have a `Topology` compatibility resource. New runtime
deployments are Node/Link-first.

/// note
The node containers are nested inside the launcher pods. A `docker ps` on the
machine where you ran the outer `containerlab` command is not the source of
truth for c9s labs.
///

The c9s runtime currently supports the main lab lifecycle and node operations:

| Command | c9s behavior |
| ------- | ------------ |
| `deploy` | creates `LauncherProfile`, `Node`, and `Link` resources and waits for readiness |
| `destroy` | deletes the lab's resources and its containerlab-managed namespace |
| `inspect` | reads Node, Deployment, Pod, and service status |
| `exec` | execs through the launcher pod into the nested node container |
| `start` | scales node Deployments to `1` |
| `stop` | scales node Deployments to `0` and pauses reconciliation |
| `restart` | restarts node Deployments |
| `save` | runs `containerlab save` inside launcher pods |
| `events` | watches Clabernetes resources and pods |
| `validate` | runs the strict c9s compiler without creating Kubernetes resources |
| `deploy --dry-run` | compiles and diffs Namespace, ConfigMap, LauncherProfile, Link, and Node resources without changing them |

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
containerlab --runtime clabernetes --namespace default deploy -t topo.clab.yml
```

For a persistent shell or automation setting, use:

```bash
export CLAB_KUBE_NAMESPACE=default
```

An explicit namespace override must already exist. Containerlab places the lab
resources there but does not create, label, or delete the namespace itself.
`--namespace` takes precedence over `CLAB_KUBE_NAMESPACE`.

/// tip
`CLAB_RUNTIME=clabernetes` is worth exporting in shell profiles, CI jobs, or
automation environments that always target c9s.
///

## Namespace rules

When `CLAB_KUBE_NAMESPACE` is unset, every lab deployed through the c9s runtime
gets a dedicated namespace named `c9s-<lab-name>`. For example, this command:

```bash
containerlab --runtime clabernetes deploy -t clos.clab.yml
```

creates the `c9s-clos` namespace and places the lab's `LauncherProfile`, `Node`,
`Link`, ConfigMap, Deployment, Pod, Service, and PVC resources there. Inspect,
exec, start, stop, restart, save, and destroy derive the same namespace from the
lab name.

Containerlab labels namespaces it creates with the runtime and lab owner. A
normal destroy removes such a managed namespace after its lab resources are
gone. If `c9s-<lab-name>` existed before deploy and does not carry those
ownership labels, containerlab uses it but preserves it during destroy.

Set `--namespace` when a shared or externally managed namespace is required:

```bash
containerlab --runtime clabernetes --namespace default deploy -t clos.clab.yml
```

All single-lab lifecycle commands must use the same flag or environment
override. Because the namespace is then shared, node and other resource names
can conflict between labs.

/// warning | Lab names
`c9s-<lab-name>` must be a valid Kubernetes DNS label and cannot exceed 63
characters. This leaves at most 59 characters for the lab name.
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

Primitive-only labs created outside containerlab are also manageable when
their Nodes, Links, and LauncherProfiles carry the common
`c9s.run/topologyOwner=<lab-name>` label. Containerlab uses that label as the
lab boundary for list, inspect, lifecycle, events, and destroy operations.

## Deploy

Deploying with the c9s runtime looks like a regular containerlab deployment:

```bash
containerlab --runtime clabernetes deploy -t topo.clab.yml
```

The deploy flow is:

1. containerlab parses and checks the topology file.
2. It stages local files into per-node ConfigMaps.
3. It compiles the final topology in memory into self-contained Nodes, Links,
   and LauncherProfiles.
4. It creates profiles and Links first, then Nodes after the complete wiring
   policy exists.
5. It waits until all Nodes report ready.
6. It inspects the resulting kubernetes state and prints the node table.

The runtime enables c9s startup and readiness probes on the generated
`LauncherProfile`. c9s checks that each nested Docker container exists, is
running, and is not paused, restarting, or dead. When an image defines a Docker
healthcheck, that healthcheck must also be healthy. Containerlab does not guess
readiness ports or special-case kinds and images, so the same baseline works for
arbitrary containerlab nodes.

Readiness is atomic for a `network-mode: container:<primary>` group. The one
launcher Pod is ready only while every nested group member satisfies the generic
readiness contract. Because every Node in the group inherits that Deployment's
readiness, a restarting secondary makes both the primary and secondary Nodes
not ready.

For an image without a Docker healthcheck, this is a process-level signal: a
running network OS may still be booting services or converging protocols. Use an
image-defined healthcheck or an explicit c9s TCP/SSH probe when the lab requires
application-level readiness.

The c9s runtime uses a ten-minute timeout by default because large NOS images
can take several minutes to load and boot. Override it when a lab needs a
different startup window, for example:

```bash
containerlab --runtime clabernetes --timeout 10m deploy -t topo.clab.yml
```

`deploy --reconfigure` first deletes all resources in the existing lab and
then deploys them again.

/// warning | Node filtering
`deploy --node-filter` is not supported with the c9s runtime. Clabernetes owns
reconciliation of the complete set of Node and Link resources. Deploy the full
topology, then use node filtering with commands such as `start`, `stop`,
`restart`, `exec`, or `save` after the lab exists.
///

Deploy reconciles an existing lab in place. Containerlab compiles the requested topology and
creates, updates, or removes the corresponding c9s `Node`, `Link`, `LauncherProfile`, and staged
`ConfigMap` resources. New Nodes remain staged until the complete Link set is present. Labs
created through the older compatibility `Topology` API retain that controller ownership and
have their `Topology` definition updated in place.

Use `deploy --reconfigure` when you explicitly want to delete and recreate every resource. Use a
different lab name or namespace when you want a separate lab:

```bash
containerlab --runtime clabernetes --name <new-lab-name> deploy -t topo.clab.yml
```

A failure while waiting for a newly created lab to become ready rolls back the
resources and managed namespace created by that deployment. A timeout while
reconciling a lab that already existed retains the lab for diagnosis and a
later corrective reconciliation.

### Validation and dry-run

Both commands use the same strict c9s preparation path as deploy, including
extended-link normalization and local-file staging checks:

```bash
containerlab --runtime clabernetes validate -t topo.clab.yml
containerlab --runtime clabernetes deploy --dry-run -t topo.clab.yml
containerlab --runtime clabernetes deploy --dry-run --format json -t topo.clab.yml
```

`validate` reports whether the topology fits the c9s runtime subset without
reading or changing lab resources. `deploy --dry-run` additionally reads the
selected cluster and reports the exact create, update, and delete plan for
Namespace, ConfigMap, LauncherProfile, Link, Node, or an older compatibility
Topology. An empty `changes` list means the deployed resources already conform.

## Inspect

Inspect works with a topology file, a lab name, or all known c9s labs:

```bash
containerlab --runtime clabernetes inspect -t topo.clab.yml
containerlab --runtime clabernetes inspect --name clos
containerlab --runtime clabernetes inspect --all
```

For c9s labs, inspect reads kubernetes resources instead of local container
runtime state. It collects the lab name, namespace, aggregate state, node
readiness, node kind and image, and load-balancer management address when
Clabernetes exposes one.

/// note
`inspect --all` groups c9s Nodes by `c9s.run/topologyOwner` across all
namespaces. A single-lab inspect uses its canonical `c9s-<lab-name>` namespace.
///

Useful kubernetes checks for the same state are:

```bash
kubectl -n <namespace> get node.c9s.run,link.c9s.run,launcherprofile.c9s.run,deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```

## Exec

`exec` runs the user command in the nested node container:

```bash
containerlab --runtime clabernetes exec -t topo.clab.yml --cmd 'ip addr'
```

Under the hood, containerlab:

1. resolves the target nodes from the Clabernetes lab state
2. finds the launcher pod for each node
3. uses kubernetes pod exec into the launcher pod
4. runs `docker exec <node> <user-command>` inside that launcher pod

/// note
The command executes in the node container, not in the launcher pod shell. RBAC
must allow `pods/exec`, and the launcher pod must be ready.
///

If any selected nested command returns nonzero, or pod exec itself fails, the
outer `containerlab exec` also returns nonzero. Successful and failed results
that were received are still printed, so automation can use both the output and
the process exit status.

## Start, stop, and restart

Node lifecycle commands operate on the kubernetes Deployments created by
Clabernetes.

```bash
containerlab --runtime clabernetes stop -t topo.clab.yml
containerlab --runtime clabernetes start -t topo.clab.yml
containerlab --runtime clabernetes restart -t topo.clab.yml
```

`stop` sets the Clabernetes ignore-reconcile label on the selected launcher
`Node` resources, then scales their Deployments to `0`:

```text
c9s.run/ignoreReconcile=true
```

The label prevents the Clabernetes manager from immediately reconciling the
nodes back to the running state.

`start` scales the selected Deployments back to `1` and clears the corresponding
Node labels. For an older compatibility lab, it also clears the Topology label
when all launchers are running again. A grouped secondary shares lifecycle
with its primary launcher node.

`restart` patches each selected Deployment with a restart annotation and waits
for it to become ready:

```text
kubectl.kubernetes.io/restartedAt=<utc timestamp>
```

## Save

Saving a c9s lab uses the containerlab process running inside each launcher pod:

```bash
containerlab --runtime clabernetes save -t topo.clab.yml
```

For each selected node, the outer containerlab process finds the launcher pod
and runs:

```bash
containerlab save -t /clabernetes/topo.clab.yaml
```

inside that pod.

`save --copy` streams the saved files back to the machine where the outer
containerlab command runs:

```bash
containerlab --runtime clabernetes save -t topo.clab.yml --copy ./startup-configs
```

The copied files follow the normal containerlab copy layout:

```text
<copy-destination>/<lab-dir>/<node>/<saved-files>
```

For example:

```text
./startup-configs/clab-clos/srl1/config-260605_085424.json
./startup-configs/clab-clos/srl1/config.json -> config-260605_085424.json
```

/// note
`save` still depends on node kind support. If a node kind does not produce saved
files, the c9s runtime has nothing to copy for that node.
///

## Events

The c9s runtime can stream Node, compatibility Topology, pod, and
interface-stat events:

```bash
containerlab --runtime clabernetes events --format json
containerlab --runtime clabernetes events --initial-state
containerlab --runtime clabernetes events --interface-stats --format json
```

For c9s, events do not come from Docker events on the outer host. Containerlab
watches:

- c9s `Node` resources carrying `c9s.run/topologyOwner`
- compatibility `Topology` resources for older labs
- Pods labeled with `c9s.run/topologyOwner`

With `--initial-state`, the stream starts with synthetic events for the current
c9s node state and then continues with live watches.

With `--interface-stats`, containerlab periodically execs through the launcher
pod and reads `/proc/net/dev` from the nested node container:

```bash
docker exec <node> cat /proc/net/dev
```

/// note | Polling, not netlink
c9s interface statistics are sampled periodically. The first sample seeds the
counters, and rates start with the second sample. Short-lived changes between
samples can be missed.
///

## Lab artifacts

With c9s, the primary artifacts are kubernetes resources and files inside the
launcher pods.

The primary kubernetes resources are:

```bash
kubectl -n <namespace> get node.c9s.run,link.c9s.run,launcherprofile.c9s.run \
  -l c9s.run/topologyOwner=<lab>
```

Related resources are selected with Clabernetes labels:

```bash
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```

To find one node launcher pod:

```bash
kubectl -n <namespace> get pod \
  -l c9s.run/topologyOwner=<lab>,c9s.run/topologyNode=<node>
```

Inside each launcher pod, Clabernetes uses:

```text
/clabernetes
```

The topology used by the inner containerlab process lives at:

```text
/clabernetes/topo.clab.yaml
```

Per-node containerlab artifacts commonly live under:

```text
/clabernetes/clab-clabernetes-<node>/<node>/
```

Local startup configurations, licenses, bind sources, `env-files`,
`extras.srl-agents`, and `extras.ceos-copy-to-flash` paths are copied into
per-node ConfigMaps and projected at the paths the inner containerlab process
expects. Each staged file is currently limited to 950 KB. These projections
are snapshots taken at deploy time, not mutable host bind mounts; run deploy
again after changing a source file.

/// tip
When debugging from inside a launcher pod, the usual containerlab and Docker
commands are useful again:

```bash
containerlab inspect
docker ps
docker exec <node> ip addr
ls -la /clabernetes
```
///

## RBAC requirements

The kube identity used by the outer containerlab process must be able to:

- get, create, and delete namespaces when using automatic per-lab namespaces
- create, get, list, watch, update, and delete c9s `Node` resources
- create, get, list, watch, and delete c9s `Link` and `LauncherProfile` resources
- get, list, watch, and delete compatibility `Topology` resources when older
  Topology-based labs must remain manageable
- list and watch Pods
- list, get, and update Deployments
- create, get, list, update, and delete ConfigMaps used for local files
- exec into launcher Pods with `pods/exec`

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
kubectl auth can-i create launcherprofiles.c9s.run -n <namespace>
kubectl auth can-i delete launcherprofiles.c9s.run -n <namespace>
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
containerlab --runtime clabernetes --namespace default deploy -t topo.clab.yml
```

### CRDs are missing

The c9s runtime talks to:

```text
nodes.c9s.run
links.c9s.run
launcherprofiles.c9s.run
```

Typical symptoms:

```text
the server could not find the requested resource
```

Check:

```bash
kubectl api-resources | grep -i clabernetes
kubectl get crd nodes.c9s.run links.c9s.run launcherprofiles.c9s.run
```

Install Clabernetes and its CRDs before using `--runtime clabernetes`.

### Manager is not reconciling

The primitive resources may exist, but no node Deployments or Pods appear.

Check:

```bash
kubectl get pods -A | grep -i clabernetes
kubectl -n <namespace> get node.c9s.run,link.c9s.run,launcherprofile.c9s.run \
  -l c9s.run/topologyOwner=<lab>
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```

If deploy waits until timeout, check the Clabernetes manager logs and verify
that it watches the namespace where the primitive resources were created.

### Nodes do not become ready

Deploy waits for every Node to report `status.readiness=ready`.

Check:

```bash
kubectl -n <namespace> get node.c9s.run,link.c9s.run \
  -l c9s.run/topologyOwner=<lab> -o yaml
kubectl -n <namespace> describe node.c9s.run <node>
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```

Common causes include bad topology data, image pull failures, missing pull
secrets, unsupported node settings, pod security policy, or a launcher pod that
cannot run nested Docker.

### Inspect shows no containers

For c9s, `inspect` looks for c9s Nodes grouped by
`c9s.run/topologyOwner` (and compatibility Topologies), not local Docker
containers.

Check:

```bash
containerlab --runtime clabernetes inspect --all
kubectl get nodes.c9s.run -A -l c9s.run/topologyOwner
```

If `docker ps` on the outer host is empty, that can be perfectly normal for c9s.
The node containers live inside launcher pods.

### Exec, save, or stats cannot reach a node

`exec`, `save`, `save --copy`, and `events --interface-stats` need pod exec into
the launcher pod.

Check:

```bash
kubectl -n <namespace> get pod \
  -l c9s.run/topologyOwner=<lab>,c9s.run/topologyNode=<node> \
  -o wide
kubectl auth can-i create pods/exec -n <namespace>
kubectl -n <namespace> exec -it <launcher-pod> -- sh
```

From inside the launcher pod:

```bash
docker ps
docker exec <node> true
ls -la /clabernetes/topo.clab.yaml
```

## Current limitations

The c9s runtime is not a complete drop-in replacement for the local Docker or
Podman runtime. Several containerlab features still assume local containers,
local network namespaces, or direct access to the host container runtime.

The runtime divides compatibility into three categories:

- Native-equivalent: normal point-to-point links and MTU, lifecycle operations,
  node configuration, staged startup files, exec, inspect, and group-atomic
  readiness.
- Documented c9s semantics: Kubernetes Service/LoadBalancer management access,
  `host:` endpoints in the launcher Pod network namespace, c9s internal
  cross-Pod link transport, and ConfigMap-backed local files.
- Rejected: external bridge/host pseudo-nodes, macvlan and `mgmt-net:` links,
  explicit native VXLAN/stitch/dummy link types, link labels or vars that would
  be discarded, native shared-management-network settings, and commands or
  flags with no c9s implementation.

Management access is not a shared management network. Each launcher has its
own nested Docker management network, so static `mgmt-ipv4`/`mgmt-ipv6`
addresses are launcher-local and are not cluster-routable between Pods.
Kubernetes Services, LoadBalancers, and DNS are the supported management access
path. Explicit topology `mgmt` settings and the `--network`, `--ipv4-subnet`,
and `--ipv6-subnet` flags are rejected to avoid implying native Docker-network
semantics.

Known command differences:

- `deploy --node-filter` and `destroy --node-filter` are rejected; a filtered
  destroy never falls through to whole-lab deletion.
- Deploy flags `--graph`, `--max-workers`, `--skip-post-deploy`,
  `--skip-labdir-acl`, `--export-template`, `--restore`, and `--restore-all`
  are rejected.
- Destroy flags `--graceful`, `--cleanup`, `--keep-mgmt-net`, and
  `--max-workers` are rejected.
- Local Docker commands on the outer host are not authoritative for c9s labs.
- Local network namespace features are not equivalent in c9s.
- `inspect interfaces` is rejected. Host-side `tc` or netem operations do not
  have the same local namespace access they have with Docker labs.
- `graph` and `tools` commands operate on local containers and host networking
  and are rejected with an error when the `clabernetes` runtime is selected.
- Per-node `runtime: docker` or `runtime: podman` is not the same as selecting
  the global `clabernetes` lab runtime.
- A lab name maps to one canonical `c9s-<lab-name>` namespace in the selected
  cluster.

/// note
Use kubernetes and launcher-pod state as the source of truth for c9s labs:

```bash
kubectl get nodes.c9s.run -A -l c9s.run/topologyOwner
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```
///
