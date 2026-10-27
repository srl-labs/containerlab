# C9s runtime

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

/// note | Runtime and c9s documentation
This page documents the native `containerlab --runtime c9s` workflow. For c9s
installation, concepts, guides, and CRD reference, see the
[c9s documentation](https://c9s.run/docs).
///

/// warning | c9s version compatibility
The c9s runtime needs clabernetes with Node run state and the clabwire capture
contract (0.11 or newer). Containerlab checks this before its first cluster call
and fails with an error that names the required release; there is no fallback
for older clabernetes releases. See [Required clabernetes release](#required-clabernetes-release).

This guide describes containerlab 0.80+ with the direct-runtime API introduced
in c9s 0.9. It does not apply to launcher-based c9s 0.8 and older releases.
c9s 0.9 cannot upgrade an older installation in place; follow the
[c9s 0.9 release notes](https://c9s.run/docs/release-notes/0.9) to uninstall
the old release and recreate its labs.

Older instructions that mention launcher Pods, nested Docker, `LauncherProfile`,
`clabverter`, `disableExpose`, or configurable resource naming describe an
incompatible API. In c9s 0.9, devices are direct Pod containers,
`NodeProfile` replaces `LauncherProfile`, `exposeType: None` replaces
`disableExpose`, and the launcher and naming fields have been removed.
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

The `imagePull.pullSecrets` list names the docker-config secrets the kubelet
uses when pulling node images. It is set only when the deploy passes
`deploy --image-pull-secret <secret-name>`; without the flag no pull secret is
referenced, which is the right shape for public images and for clusters whose
container runtime already holds registry credentials. The referenced secret
must exist in the lab namespace (`regcred` is a common name for it by
Kubernetes convention).

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
| `inspect` | reads Node, Pod, and service status |
| `exec` | execs directly into the node's device container |
| `start` | sets the node's `spec.runState` to `Running` |
| `stop` | sets the node's `spec.runState` to `Stopped` |
| `restart` | requests a Pod replacement with a `c9s.run/restart-requested` token |
| `save` | runs the node's save lifecycle inside its device container |
| `events` | watches Clabernetes resources and pods |
| `logs` | streams the device container log |
| `capture` | captures lab link interfaces through the `clabwire` sidecar into one pcapng stream |
| `inspect interfaces` | lists the capturable link interfaces with their sysfs state |
| `validate` | checks c9s compatibility without creating Kubernetes resources |
| `deploy --dry-run` | compiles and diffs Namespace, ConfigMap, NodeProfile, Link, and Node resources without changing them |
| `deploy --emit-crs` | prints the Namespace, ConfigMap, and Topology (or NodeProfile, Link, and Node) manifests deploy would create, without contacting the cluster |

## Requirements

The c9s runtime expects:

- a reachable kubernetes cluster
- kubernetes 1.31 or newer
- clabernetes with Node run state and the clabwire capture contract (0.11 or
  newer) installed in the cluster
- the Clabernetes manager running and watching all lab namespaces
- kubernetes RBAC allowing containerlab to manage the required resources
- permission for lab namespaces to run the privileged c9s connectivity
  init-sidecar

/// note
Containerlab creates a dedicated namespace for each c9s lab. The kube identity
therefore needs cluster-scoped permission to get, create, and delete namespaces
unless you select an existing namespace with `CLAB_KUBE_NAMESPACE`.
///

### Required clabernetes release

Containerlab needs a clabernetes release that has:

- Node run state: `nodes.c9s.run` `spec.runState` (`Running` | `Stopped`), the
  `Stopped` condition, and restart requests through the
  `c9s.run/restart-requested` annotation
- the clabwire packet capture contract: `--nodeName`, repeated `--interface`,
  `--format pcapng` and `--list-interfaces`
- the namespace-bindable `clabernetes-lab-viewer` and `clabernetes-lab-editor`
  roles

Both features ship in the same release, so containerlab checks one field: the
published OpenAPI schema of `nodes.c9s.run` (`/openapi/v3/apis/c9s.run/v1alpha1`)
must have `spec.runState`. Every authenticated user may read that document, so
the check needs no extra RBAC. It runs once per command (once per runtime
instance for library users) before the first cluster call. Commands that do not
contact the cluster (`validate`, `deploy --emit-crs`, `graph`) skip it. Against
an older release every other command fails like this:

```text
the clabernetes release on this cluster is not supported: nodes.c9s.run has no
spec.runState (Node run state); the c9s runtime needs clabernetes with Node run
state and the clabwire capture contract (0.11 or newer)
```

Library users can run the check up front with
`(*core.CLab).CheckLabRuntimeSupport(ctx)` or the optional
`labruntime.SupportChecker` interface; the error wraps
`labruntime.ErrUnsupportedRuntimeVersion`.

See the
[c9s cluster requirements](https://c9s.run/docs/installation#cluster-requirements)
for Pod Security Admission, worker kernel, network policy, registry, and KVM
requirements.

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
subnet spans the namespace through the c9s routed management mesh.

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

/// note | Lab and node names
Kubernetes object names are lowercase DNS labels, and a lot of labs name their
nodes `R1` or `PE_1`. Containerlab sanitizes the lab name and every node name
Kubernetes cannot carry instead of rejecting the topology: the name is
lower-cased, every character outside `a-z`, `0-9` and `-` becomes `-`, a name
that would start with a digit is prefixed with `clab-`, and a name longer than
63 characters is truncated and suffixed with a hash. `R1` becomes `r1`, `PE_1`
becomes `pe-1`.

Every rename is logged, and the topology is rewritten before it reaches the
cluster, so links, `network-mode: container:<node>` and the staged file mounts
follow the node. Node names on the command line (`node start R1`,
`--node-filter R1`, `save`) are accepted in either form. Everything
containerlab reports back — `inspect`, container names, the `clab-node-name`
label — uses the sanitized name, because that is the name the objects carry in
the cluster.

To keep the original names reachable, containerlab injects a
`c9s.run/originalNodeName` label into the stored topology definition and onto
every c9s Node object. The label holds the node name exactly as the clab file
spells it, so tooling that only sees the clab file can select the c9s Nodes by
those names. The label key is reserved by containerlab: a value set for it in
the topology file is overwritten on every deploy.

Two node names that differ only in a character Kubernetes cannot carry (`R1`
and `r1`) would end up as one object; that is an error, not a merge.

`c9s-<lab-name>` must still fit in a DNS label of 63 characters, which leaves
at most 59 characters for the lab name.
///

Some commands intentionally look across namespaces:

- `inspect --all`
- `destroy --all`
- `events` without a lab

For these commands, containerlab uses all-namespaces kubernetes listing or
watching. A namespace override (`--namespace` or `CLAB_KUBE_NAMESPACE`) limits
them to that namespace, so an identity whose RBAC only covers one namespace can
still list, destroy and watch the labs in it. `events` with a lab (`-t` or
`--name`) watches only that lab, in the lab's namespace.

The c9s container ID includes the namespace so follow-up actions can still
target the right lab. `inspect` reports it as `container_id`, and events use it
as the actor ID:

```text
<namespace>/<lab>/<node>
```

For example:

```text
c9s-clos/clos/srl1
```

### Labs managed as direct c9s resources

This is an advanced workflow for users who manage Kubernetes resources through
GitOps or `kubectl` instead of letting containerlab create a `Topology`.
For example, a deployment pipeline can generate a bundle without contacting a
cluster:

```bash
containerlab --runtime c9s deploy --no-topology-cr --emit-crs \
  -t edge-lab.clab.yml > edge-lab.c9s.yaml
```

The bundle contains the lab's `NodeProfile`, `Node`, and `Link` resources, each
with this ownership label:

```yaml
metadata:
  labels:
    c9s.run/topologyOwner: edge-lab
```

After a GitOps controller or `kubectl apply` installs that bundle, containerlab
can discover the lab without a `Topology` resource:

```bash
containerlab --runtime c9s --namespace c9s-edge-lab \
  inspect --name edge-lab
```

Containerlab discovers a primitive-only lab from its labeled Nodes. It uses the
same label to associate the lab's Links, NodeProfiles, ConfigMaps, and
c9s-generated workloads for lifecycle, events, and destroy operations. All
resources must be in the selected namespace and use the same label value.
Manually authored resources can use the same contract, but an incorrect or
reused label can cause containerlab to operate on unrelated resources.

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

`--max-workers` limits concurrent ConfigMap apply requests. Destroy uses
label-scoped Kubernetes collection deletion; if RBAC or the API does not allow
that operation, it falls back to individual deletes limited by
`--max-workers`. Values above the Kubernetes client's burst limit are honored,
but produce a warning because client-side throttling may reduce their benefit.

/// warning | Node filtering
`deploy --node-filter` is not supported with the c9s runtime. Clabernetes owns
reconciliation of the complete set of Node and Link resources. Deploy the full
topology, then use node filtering with commands such as `start`, `stop`,
`restart`, `exec`, or `save` after the lab exists.
///

Deploy reconciles an existing lab in place. A Topology-owned lab — deployed by
containerlab or `kubectl` — has its `Topology` definition updated in place and
the controller converges the compiled resources on it.
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

### Validation, dry-run, and emitting manifests

All three modes use the same c9s preparation path as deploy, including extended-link
normalization, compatibility checks, and local-file staging checks:

```bash
containerlab --runtime c9s validate -t topo.clab.yml
containerlab --runtime c9s deploy --dry-run -t topo.clab.yml
containerlab --runtime c9s deploy --dry-run --format json -t topo.clab.yml
containerlab --runtime c9s deploy --emit-crs -t topo.clab.yml > topo-crs.yaml
```

`validate` reports whether the topology can be represented by the c9s runtime without reading
or changing lab resources. Like `deploy --emit-crs`, it never contacts the cluster and works
without a kubeconfig that loads: containerlab only loads the kubeconfig when a command first
needs the cluster, so a topology can be validated before the cluster is reachable. Compilation is fail-closed: every source construct c9s cannot
preserve is reported as an error, with its field and line, in one pass. A small set of
lossy-but-safe fields (shared management network selection, pinned host-side ports) is accepted
with a warning. `deploy --dry-run` additionally reads the selected cluster and reports the
exact create, update, and delete plan for the resources containerlab itself would change:
Namespace, ConfigMap, and the `Topology` — or, with `--no-topology-cr`, the NodeProfile, Link,
and Node resources. An empty `changes` list means the deployed resources already conform.

`deploy --emit-crs` prints those same resources as a multi-document YAML stream on stdout and
exits without contacting the cluster. Log output stays on stderr, so the stream can be redirected
to a file, edited, and applied with `kubectl apply -f`. The bundle contains the managed lab
Namespace (omitted when a namespace override is in effect), the staged ConfigMaps, and the
`Topology` — or, with `--no-topology-cr`, the compiled NodeProfile, Link, and Node resources.
Staged text files are plain `data` entries in their ConfigMaps; only files that are not valid
text are base64-encoded under `binaryData`.
Every object carries the `c9s.run/topologyOwner` label, so a lab applied from an emitted bundle
is discovered and managed like any other c9s lab. `--format json` prints the bundle as one
`v1/List` object. A later `deploy` of the same topology reconciles the lab back to the topology
file and discards manual edits made to the applied manifests.

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
namespaces, or in the namespace override when one is set. A single-lab inspect
uses its canonical `c9s-<lab-name>` namespace.
///

`inspect --format json` identifies every node uniquely and names the Kubernetes
workload that runs it, so clients can open terminals, follow logs or
port-forward without working out the pod themselves:

| Field | Value |
|---|---|
| `container_id` | `<namespace>/<lab>/<node>`, the same ID events use as actor |
| `namespace` | the lab's Kubernetes namespace |
| `pod` | the pod that runs the node; nodes grouped through `network-mode: container:<node>` share the pod of their primary node |
| `container` | the node's primary device container in the pod, the same container `exec` uses |
| `node_uid` | the UID of the c9s `Node` resource |
| `exposed_ports` | the ports the node's Service publishes (`Node.status.exposedPorts`): `port` on the node, `expose_port` on the Service, and `protocol` |
| `load_balancer_address` | the address of the node's LoadBalancer Service, when it has one |

```json
{
  "clos": [
    {
      "lab_name": "clos",
      "labPath": "clos.clab.yml",
      "absLabPath": "/home/user/clos.clab.yml",
      "name": "clos-srl1",
      "container_id": "c9s-clos/clos/srl1",
      "image": "ghcr.io/nokia/srlinux",
      "kind": "nokia_srlinux",
      "state": "running",
      "status": "healthy",
      "ipv4_address": "172.18.0.200",
      "ipv6_address": "N/A",
      "namespace": "c9s-clos",
      "pod": "srl1-7d9c6b5f4-x2k8p",
      "container": "node-srl1-3f2a",
      "node_uid": "0b6e2f60-3d0c-4c5e-9a43-5c1b7f1d2a9e",
      "exposed_ports": [
        { "port": 22, "expose_port": 22, "protocol": "tcp" },
        { "port": 57400, "expose_port": 57400, "protocol": "tcp" }
      ],
      "load_balancer_address": "172.18.0.200"
    }
  ]
}
```

These fields are only present for lab runtimes; Docker and Podman output is
unchanged. Fields without a value, such as `pod` for a stopped node, are left
out. `--details` keeps printing the raw records.

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

`exec` does not check for a local Docker or Podman daemon; the outer host only
needs to reach the cluster.

If any selected command returns nonzero, or pod exec itself fails, the outer
`containerlab exec` also returns nonzero. Successful and failed results that
were received are still printed, so automation can use both the output and the
process exit status.

## Logs

`logs` shows the log of a node's primary device container, the same container
`exec` uses:

```bash
containerlab --runtime c9s logs -t topo.clab.yml -n srl1
containerlab --runtime c9s logs --name clos -n srl1 --tail 100 --follow --timestamps
```

It reads the `pods/log` subresource of the node's pod, so RBAC must allow
`get` on `pods/log`. Kubernetes returns stdout and stderr as one stream. See
the [`logs` command](../../cmd/logs.md) for the flags.

## Packet capture

[`containerlab capture`](../../cmd/capture.md) captures lab link interfaces of
one or more nodes into one pcapng stream. Each node's pod runs the `clabwire`
sidecar, which realized the node's links; containerlab execs its capture
command (`/clabernetes/manager node-runtime packet-capture`) in the sidecar.
No capture service runs in the cluster, and nothing is captured outside the
lab's planned link interfaces.

```bash
# both ends of a link, for 30 seconds
containerlab -r c9s capture -t clos.clab.yml \
  --target srl1:e1-1 --target srl2:e1-1 --duration 30s -w srl.pcapng

# live in Wireshark until Wireshark or Ctrl+C ends it
containerlab -r c9s capture --name clos --target srl1:e1-1,e1-2 -w - | wireshark -k -i -

# what can be captured, with the peer of each link
containerlab -r c9s capture --name clos --list
```

How it works:

- containerlab locates each target node's pod and runs one `clabwire` capture
  per node, with `--nodeName`, every requested interface of that node and
  `--format pcapng`.
- The streams are merged into one section: one Interface Description Block per
  target named `<node>:<interface>` (the node as the topology spells it),
  written before the first packet, and the packets of every target with their
  interface IDs remapped.
- `clabwire` limits one capture to one hour. Longer captures, and captures
  without `--duration`, chain captures with a few seconds of overlap and switch
  in the middle of it; packets near the switch are matched by content so each
  is written once. A follow-up that does not start in time is reported as a
  gap, with its time span.
- `clabwire` audit records (JSON on its stderr, schema
  `c9s.direct-packet-capture-audit/v1alpha1`) with status `Denied` or `Failed`
  end the command with their reason. `Started` and `Succeeded` go to the debug
  log.

Interface names are the names inside the node; targets may also use the alias
the topology used for an interface:

| Kind | Interface | Alias |
| ---- | --------- | ----- |
| linux | `eth1` | |
| SR Linux | `e1-1` | `ethernet-1/1` |
| vr-sros / nokia_sros | `eth3` | `1/1/3` |

The management interface and the pod's `c9s0` interface cannot be captured.

Ctrl+C or SIGTERM ends a capture cleanly: the output stays a valid pcapng
file and the command exits with code 0. Kubernetes does not stop a command
when its exec stream closes, so an interrupted `clabwire` capture keeps
running in the pod until its next packet fails to write or its segment ends
(at most one hour); it does not hold any lab resource.

Capturing needs `get` and `create` on `pods/exec` in the lab namespace, which
`clabernetes-lab-editor` grants. `inspect interfaces -r c9s` lists the same
interfaces, with the MAC address, index, MTU and operational state read from
sysfs in the device container (type `veth`, which is how `clabwire` realizes
lab links).

## Start, stop, and restart

Node lifecycle commands ask the clabernetes controllers for a run state; they
never touch Deployments or Pods.

```bash
containerlab --runtime c9s stop -t topo.clab.yml -n srl1
containerlab --runtime c9s start -t topo.clab.yml -n srl1
containerlab --runtime c9s restart -t topo.clab.yml
```

`stop` and `start` send a merge patch to the node's `Node` resource:

```json
{"spec": {"runState": "Stopped"}}
```

(`"Running"` for `start`). `stop` returns once the controller reports the
`Stopped` condition as `True`, which it does when no Pod of the node remains.
`start` returns once the condition is `False` for the patched generation and
the node's readiness is `ready`. Both wait at most `--timeout`.

`restart` sets a new token in the `c9s.run/restart-requested` annotation of the
node. The controller replaces the Pod once per new token, and `restart` returns
when `status.observedRestartToken` equals the token and the node is ready. A
stopped node is started by the same patch.

What a stopped node keeps and loses:

- The `Node`, its Services, PersistentVolumeClaims and Links stay. Saved
  configuration survives a stop and start.
- Its links go carrier-down on every peer, like an unplugged cable;
  `events` reports them as interface `down` events.
- `inspect` shows the node as `stopped` (`stopping` while its Pod terminates).
- clabernetes keeps a running peer of a stopped node `notready`: its
  connectivity sidecar waits for the far end of the wire. Deploy, apply, start
  and restart treat such a node as settled when its containers are ready, and
  log `link peers stopped: <nodes>` instead of waiting for it.

Nodes that share a Pod (`network-mode: container:<node>`) follow the run state
of their primary node. Stopping, starting or restarting any node of the group
acts on the primary, once.

The controllers keep reconciling while nodes are stopped. A `deploy` that
changes a stopped node updates its `Node`; the node stays stopped and boots the
new definition on its next `start`. Containerlab never sends `spec.runState`
during deploy, and with `--no-topology-cr` it carries the run state of existing
Nodes over, so an apply never starts a stopped node.

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

The c9s runtime can stream Node, Topology, pod, interface state and
interface-stat events:

```bash
containerlab --runtime c9s events --format json
containerlab --runtime c9s events --initial-state
containerlab --runtime c9s events --interface-stats --format json
containerlab --runtime c9s events --name clos --format json
containerlab --runtime c9s --namespace team-a events --format json
```

For c9s, events do not come from Docker events on the outer host. Containerlab
watches:

- c9s `Node` resources carrying `c9s.run/topologyOwner`
- `Topology` resources
- Pods labeled with `c9s.run/topologyOwner`

What the stream covers:

| Invocation | Scope |
|---|---|
| `events` | every namespace |
| `--namespace <ns> events` or `CLAB_KUBE_NAMESPACE=<ns>` | the labs in `<ns>` |
| `events -t <topo>` or `events --name <lab>` | that lab, in its namespace (`c9s-<lab>`, or the namespace override) |

An identity whose RBAC only covers one namespace needs one of the scoped forms;
watching every namespace requires cluster-wide `list` and `watch`.

With `--initial-state`, the stream starts with synthetic events for the current
c9s node state and the state of every interface, then continues with live
watches.

Container events use the node's `<namespace>/<lab>/<node>` ID as actor, the
`container_id` inspect reports.

### Interface events

Containerlab samples the interfaces of every running node by exec into its
device container. One exec per node and interval reads `/proc/net/dev` and
`/sys/class/net/*/operstate` (with index, MTU, MAC address, flags and alias).
All containers of a device pod share one network namespace, so the samples
cover the node's pod interfaces.

Interface state events are always streamed. They use the same schema as the
interface events of the local runtimes, with the operational state as action:

```json
{
  "timestamp": "2026-10-27T10:15:02.123456789Z",
  "type": "interface",
  "action": "down",
  "actor_id": "c9s-clos/clos/srl1",
  "actor_name": "clos-srl1",
  "actor_full_id": "srl1-7d9c6b5f4-x2k8p",
  "attributes": {
    "ifname": "e1-1",
    "state": "lowerlayerdown",
    "index": "7",
    "mtu": "9232",
    "mac": "aa:c1:ab:5e:2f:01",
    "lab": "clos",
    "node": "srl1",
    "name": "clos-srl1",
    "namespace": "c9s-clos",
    "pod": "srl1-7d9c6b5f4-x2k8p",
    "origin": "clabernetes"
  }
}
```

- `action` is `up` when the operational state is `up`, or `unknown` on an
  administratively up interface (loopback, tunnels); otherwise `down`.
- `attributes.state` is the raw operational state from
  `/sys/class/net/<if>/operstate`. An interface that disappears, or whose node
  stops running, is reported once as `down` with state `notpresent`.
- An event is emitted when an interface appears, when its action or state
  changes, and, with `--initial-state`, once for every interface at the start.
- Device containers without a shell report statistics only, no interface state.

With `--interface-stats`, the same exec also produces `interface stats` events
with byte and packet counters and rates. They use the node ID as actor, like
the state events. `--interface-stats-interval` sets the sampling interval of
both (default 1s).

/// note | Polling, not netlink
c9s interface state and statistics are sampled periodically. The first sample
seeds the counters, and rates start with the second sample. Short-lived changes
between samples can be missed. Every running node gets one exec per interval,
so use a longer `--interface-stats-interval` for large labs.
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

Local startup configurations, licenses, bind sources, `env-files`, the
`copy-to-flash` [kind-specific config
key](../nodes.md#kind-specific-config), and other path-bearing node fields
are copied into per-node ConfigMaps and staged at the paths the node expects.
An inline
`startup-config` is staged as a partial configuration and merged over the
kind's default config, exactly like the local runtimes. Each staged file is
currently limited to 950 KB. These projections are snapshots taken at deploy
time, not mutable host bind mounts; run deploy again after changing a source
file.

Remote startup-config and license URLs use the same HTTP(S), S3, FTP, SFTP,
and SCP download support as local runtimes. Containerlab downloads them before
creating cluster resources and includes their contents in the staged ConfigMaps;
the cluster does not need access to those URLs. Remote startup-config filenames
retain their `.partial` marker. Licenses can also be embedded as multiline strings.

### Device state persistence

By default, deploy enables [Clabernetes persistence](https://c9s.run/docs/guides/persistence)
on the lab: every node's artifact volume is backed by a per-node
PersistentVolumeClaim, and the in-cluster claim plays the role the
[lab directory](../conf-artifacts.md) plays with local runtimes. The startup
configuration seeds the node once; from then on the configuration the device
saved wins, and a pod restart, eviction, or unrelated topology change never
silently reverts saved work. `enforce-startup-config` makes the declared
configuration win on every boot, exactly as it does locally.

This requires a dynamically provisionable (default) storage class in the
cluster. Deploy with `--no-persistence` to run nodes on ephemeral storage
instead; every pod replacement then factory-resets the node to its declared
configuration.

No local lab directory is created for c9s labs: the cluster holds the lab
state. Use `containerlab --runtime c9s save --copy <dir>` to export saved
configurations to local disk when you want a copy outside the cluster.

Deploy sets the claims' reclaim policy to `Retain`, so they outlive their
`Node` resources, like a local lab directory outlives its containers:

| Command | Persistent claims |
|---|---|
| `destroy` | kept, with the managed `c9s-<lab>` namespace that holds them; the next deploy of the lab boots from them |
| `destroy --cleanup` | deleted, then the managed namespace |
| `redeploy` | kept, so the redeployed nodes boot from their saved state |
| `redeploy --cleanup`, `deploy --reconfigure` | deleted before the lab is deployed again |

A plain destroy reports the claims it keeps. To delete them later, run
`destroy --cleanup` for the lab, or delete them with
`kubectl -n <namespace> delete pvc -l c9s.run/topologyOwner=<lab>`.

To re-seed a single node from its declared configuration
without redeploying the lab, annotate its Node with a device-state reset
token:

```bash
kubectl -n <namespace> annotate node.c9s.run <node> \
  c9s.run/device-state-reset=$(date +%s) --overwrite
```

## Node access from Go

Programs that use containerlab as a Go library, such as clab-api-server, open
terminals, follow logs, reach node ports and read resource usage through
optional lab runtime interfaces in `github.com/srl-labs/containerlab/labruntime`,
implemented by the c9s runtime:

| Interface | Method | What it does |
|---|---|---|
| `NodeLocator` | `LocateNode(ctx, NodeRequest) ([]NodeLocation, error)` | the namespace, pod, primary device container, Node UID, exposed ports and LoadBalancer address of nodes |
| `NodeAttacher` | `Attach(ctx, AttachRequest, AttachStreams) error` | an interactive exec with stdin, an optional TTY and a resize channel, in the device container or a chosen container |
| `NodeLogStreamer` | `StreamLogs(ctx, LogRequest, io.Writer) error` | the device container log, with tail, follow and timestamps |
| `NodePortForwarder` | `DialPort(ctx, NodeRequest, port) (net.Conn, error)` | a connection to a TCP port of the node's pod through `pods/portforward` (SSH, web UIs) |
| `NodeStatsSampler` | `SampleStats(ctx, InspectRequest) ([]NodeStats, error)` | CPU and memory per node from `metrics.k8s.io` PodMetrics, summing the node's device containers; without the metrics API the error wraps `labruntime.ErrMetricsUnavailable` |
| `PacketCapturer` | `CaptureInterfaces(ctx, NodeRequest) ([]CaptureInterface, error)` | the capturable link interfaces of nodes, with alias, peer node and peer interface |
| `PacketCapturer` | `Capture(ctx, CaptureRequest, io.Writer) error` | one pcapng stream of the request's targets, for `Duration` (0: until ctx ends), `PacketLimit` and `SnapLength`; gaps go to `OnGap` |
| `InterfaceInspector` | `InspectInterfaces(ctx, NodeRequest) ([]*types.ContainerInterfaces, error)` | the `inspect interfaces` view of the link interfaces |
| `SupportChecker` | `CheckSupport(ctx, Identity) error` | the clabernetes release check |

The `core` package wraps them per lab and node name, accepting the node name
as the topology file spells it or as it was sanitized:
`CLab.LocateLabRuntimeNodes`, `CLab.AttachLabRuntimeNode`,
`CLab.DialLabRuntimeNodePort`, `CLab.SampleLabRuntimeNodeStats`,
`CLab.CaptureInterfaces`, `CLab.Capture`, `CLab.LabRuntimeInterfaces`,
`CLab.CheckLabRuntimeSupport` and `CLab.NodeLogs`, which also serves Docker and
Podman.

```go
c, err := clabcore.NewContainerLab(
	clabcore.WithRuntime("c9s", nil),
	clabcore.WithTopologyName("clos"),
	clabcore.WithLabRuntimeIdentity(clablabruntime.Identity{
		User:   "clab:alice",
		Groups: []string{"clab:users"},
	}),
)
if err != nil {
	return err
}

conn, err := c.DialLabRuntimeNodePort(ctx, "srl1", 22)

// stream pcapng to a websocket, a file or Wireshark until ctx ends
err = c.Capture(ctx, clablabruntime.CaptureRequest{
	Targets: []clablabruntime.CaptureTarget{
		{Node: "srl1", Interface: "ethernet-1/1"},
		{Node: "srl2", Interface: "e1-1"},
	},
}, w)
```

### Acting as a user

A server that runs labs for many users acts as each user, so the cluster's RBAC
decides what a user may do and its audit log names the user:

- `labruntime.Identity{User, Groups}` is Kubernetes user impersonation.
  `labruntime.Config.Identity` sets it for a runtime, every request type has an
  `Identity` field that overrides it per call, and
  `clabcore.WithLabRuntimeIdentity` sets it for every call of a `CLab`.
- The c9s runtime builds one set of Kubernetes clients per identity from the
  kubeconfig and caches them. The impersonation applies to every transport:
  REST calls, watches, log streams, and the websocket and SPDY connections of
  exec, attach and port-forward.
- Without an identity, the kubeconfig is used as it is. The CLI has no
  impersonation flags: a CLI user acts with their own kubeconfig rights.

The kubeconfig identity then needs the `impersonate` verb on the users and
groups it acts as.

## RBAC requirements

clabernetes ships two ClusterRoles meant to be bound per lab namespace with a
RoleBinding: `clabernetes-lab-viewer` (read) and `clabernetes-lab-editor`
(read, write c9s resources and ConfigMaps, delete PersistentVolumeClaims,
`pods/exec` and `pods/portforward`). Users who work with labs in existing
namespaces need only `clabernetes-lab-editor` there; it has no write access to
Deployments or Pods, and containerlab needs none:

```bash
kubectl create rolebinding alice-lab-editor -n <namespace> \
  --clusterrole clabernetes-lab-editor --user alice
containerlab --runtime c9s --namespace <namespace> deploy -t topo.clab.yml
```

`inspect`, `events` and `logs` work with `clabernetes-lab-viewer`. Creating the
per-lab namespace is a cluster-scoped right the lab roles do not include; select
a precreated namespace with `--namespace` or `CLAB_KUBE_NAMESPACE`, or grant
namespace rights separately.

In detail, the kube identity used by the outer containerlab process must be able
to:

- get, create, and delete namespaces when using automatic per-lab namespaces
- create, get, list, watch, update, patch, and delete c9s `Node` resources
  (`patch` for `start`, `stop` and `restart`)
- create, get, list, watch, and delete c9s `Link` and `NodeProfile` resources
- get, list, watch, and delete `Topology` resources when Topology-based labs
  must remain manageable
- list and watch Pods
- create, get, list, update, and delete ConfigMaps used for local files
- exec into device Pods with `pods/exec`, also for `capture` and `inspect
  interfaces` (in the `clabwire` and device containers)
- list and delete PersistentVolumeClaims for `destroy --cleanup`
- read container logs with `pods/log` for `logs`
- port-forward with `pods/portforward` for `NodePortForwarder`
- list `pods.metrics.k8s.io` for `NodeStatsSampler`

Useful checks:

```bash
kubectl auth can-i get namespaces
kubectl auth can-i create namespaces
kubectl auth can-i delete namespaces
kubectl auth can-i create nodes.c9s.run -n <namespace>
kubectl auth can-i list nodes.c9s.run -n <namespace>
kubectl auth can-i update nodes.c9s.run -n <namespace>
kubectl auth can-i patch nodes.c9s.run -n <namespace>
kubectl auth can-i delete nodes.c9s.run -n <namespace>
kubectl auth can-i create links.c9s.run -n <namespace>
kubectl auth can-i delete links.c9s.run -n <namespace>
kubectl auth can-i create nodeprofiles.c9s.run -n <namespace>
kubectl auth can-i delete nodeprofiles.c9s.run -n <namespace>
kubectl auth can-i list pods -n <namespace>
kubectl auth can-i watch pods -A
kubectl auth can-i create pods --subresource exec -n <namespace>
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

Install Clabernetes and its CRDs before using `--runtime c9s`. A cluster
without the c9s API fails the version check with `the c9s.run/v1alpha1 API is
not served (is clabernetes installed?)`.

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

`exec`, `save`, `save --copy`, and the interface events of `events` need pod
exec into the device pod.

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
  pseudo-nodes, node-name collisions after Kubernetes sanitization, and
  commands or flags with no c9s implementation.

Management access behaves like containerlab: every node gets an allocated
management address (honoring the topology `mgmt` subnet and static
`mgmt-ipv4`/`mgmt-ipv6` addresses). c9s 0.9 carries the shared management
subnet over a routed cross-Pod mesh with proxy ARP and IPv6 neighbor discovery;
it is not a bridged L2 segment between Pods. Devices can still reach each
other's management addresses directly. From outside the cluster, Kubernetes
Services (LoadBalancer by default), pod IPs, and DNS are the access paths. The `--network`,
`--ipv4-subnet`, and `--ipv6-subnet` flags remain rejected because they imply
native Docker-network semantics.

Known command differences:

- `deploy --node-filter` and `destroy --node-filter` are rejected; a filtered
  destroy never falls through to whole-lab deletion.
- Deploy flags `--graph`, `--skip-post-deploy`, `--skip-labdir-acl`,
  `--export-template`, `--restore`, and `--restore-all` are rejected.
- Destroy flags `--graceful` and `--keep-mgmt-net` are rejected. `--cleanup`
  also deletes the lab's persistent claims (saved device state) and removes
  the local lab directory; a plain destroy keeps the claims.
- Local Docker commands on the outer host are not authoritative for c9s labs.
- `inspect interfaces` lists only the lab link interfaces (those `capture`
  accepts), not the management or pod interfaces. Host-side `tc` or netem
  operations do not have the same local namespace access they have with Docker
  labs.
- `graph` renders from the topology file only: `--mermaid`, `--dot`, `--drawio`
  and `--offline` work, the default mode that reads running containers is
  rejected, and so is `--node-filter`. `--drawio` runs the clab-io-draw
  container, so it needs Docker or Podman on the outer host.
- `tools` commands operate on local containers and host networking and are
  rejected with an error when the `c9s` runtime is selected.
- Per-node `runtime: docker` or `runtime: podman` is not the same as selecting
  the global `c9s` lab runtime.
- A lab name maps to one canonical `c9s-<lab-name>` namespace in the selected
  cluster.
- c9s 0.9 removed `Topology.spec.naming`. A topology with an explicit
  `prefix: ""` currently emits that legacy field in the default Topology mode
  and is rejected by the c9s 0.9 schema. Use `--no-topology-cr` for such a
  topology.

/// note
Use kubernetes state as the source of truth for c9s labs:

```bash
kubectl get nodes.c9s.run -A -l c9s.run/topologyOwner
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l c9s.run/topologyOwner=<lab>
```

///
