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
Podman containers for the lab nodes. Instead, it renders the final topology and
stores it in a Clabernetes `Topology` custom resource:

```yaml
apiVersion: clabernetes.containerlab.dev/v1alpha1
kind: Topology
metadata:
  name: <lab-name>
  namespace: <namespace>
spec:
  definition:
    containerlab: |
      <rendered containerlab topology yaml>
```

The Clabernetes manager then reconciles this resource into kubernetes objects,
usually one launcher Deployment and Pod per topology node. Each launcher pod
runs containerlab inside the pod and starts the real node container there.

/// note
The node containers are nested inside the launcher pods. A `docker ps` on the
machine where you ran the outer `containerlab` command is not the source of
truth for c9s labs.
///

The c9s runtime currently supports the main lab lifecycle and node operations:

| Command | c9s behavior |
| ------- | ------------ |
| `deploy` | creates the Clabernetes `Topology` resource and waits for readiness |
| `destroy` | deletes the `Topology` resource |
| `inspect` | reads `Topology`, Deployment, Pod, and service status |
| `exec` | execs through the launcher pod into the nested node container |
| `start` | scales node Deployments to `1` |
| `stop` | scales node Deployments to `0` and pauses reconciliation |
| `restart` | restarts node Deployments |
| `save` | runs `containerlab save` inside launcher pods |
| `events` | watches Clabernetes resources and pods |

## Requirements

The c9s runtime expects:

- a reachable kubernetes cluster
- Clabernetes CRDs installed in the cluster
- the Clabernetes manager running and watching the lab namespace
- a namespace that already exists for the lab
- kubernetes RBAC allowing containerlab to manage the required resources

/// warning
The c9s runtime does not create namespaces for you. Create the target namespace
first, or select an existing namespace.
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

You can override the lab namespace with:

```bash
export CLAB_KUBE_NAMESPACE=<namespace>
```

If no namespace is set with `CLAB_KUBE_NAMESPACE` or the selected kube context,
containerlab uses `default`.

/// tip
`CLAB_RUNTIME=clabernetes` and `CLAB_KUBE_NAMESPACE=<namespace>` are often the
two variables worth exporting in shell profiles, CI jobs, or automation
environments that always target c9s.
///

## Namespace rules

For normal single-lab commands, containerlab operates in one namespace:

1. an internal request namespace, when the caller provides one
2. `CLAB_KUBE_NAMESPACE`
3. the namespace from the selected kube context
4. `default`

For example:

```bash
CLAB_KUBE_NAMESPACE=lab-a \
  containerlab --runtime clabernetes deploy -t topo.clab.yml
```

creates the `Topology` resource in the `lab-a` namespace when a topology with
the same name does not already exist there.

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
default/clos/srl1
```

## Deploy

Deploying with the c9s runtime looks like a regular containerlab deployment:

```bash
containerlab --runtime clabernetes deploy -t topo.clab.yml
```

The deploy flow is:

1. containerlab parses and checks the topology file.
2. It renders the final topology YAML.
3. It creates a Clabernetes `Topology` resource.
4. It waits until Clabernetes reports the topology as ready.
5. It inspects the resulting kubernetes state and prints the node table.

`deploy --reconfigure` first deletes the existing `Topology` resource and then
deploys it again.

/// warning | Node filtering
`deploy --node-filter` is not supported with the c9s runtime. Clabernetes owns
reconciliation of the complete topology stored in the `Topology` resource.
Deploy the full topology, then use node filtering with commands such as
`start`, `stop`, `restart`, `exec`, or `save` after the topology exists.
///

Deploy is create-only. If a `Topology` with the same name already exists in the
same namespace, containerlab fails the deployment:

```text
the '<lab>' lab has already been deployed in namespace '<namespace>'.
```

Use `deploy --reconfigure` to replace the existing lab, or use a different lab
name or namespace when you want a separate lab:

```bash
containerlab --runtime clabernetes --name <new-lab-name> deploy -t topo.clab.yml
```

## Inspect

Inspect works with a topology file, a lab name, or all known c9s labs:

```bash
containerlab --runtime clabernetes inspect -t topo.clab.yml
containerlab --runtime clabernetes inspect --name clos
containerlab --runtime clabernetes inspect --all
```

For c9s labs, inspect reads kubernetes resources instead of local container
runtime state. It collects the topology name, namespace, topology state, node
readiness, node kind and image, and load-balancer management address when
Clabernetes exposes one.

/// note
`inspect --all` lists c9s topologies across all namespaces. A single-lab
inspect uses the selected namespace.
///

Useful kubernetes checks for the same state are:

```bash
kubectl get topologies -A
kubectl -n <namespace> get topology <lab> -o yaml
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l clabernetes/topologyOwner=<lab>
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

## Start, stop, and restart

Node lifecycle commands operate on the kubernetes Deployments created by
Clabernetes.

```bash
containerlab --runtime clabernetes stop -t topo.clab.yml
containerlab --runtime clabernetes start -t topo.clab.yml
containerlab --runtime clabernetes restart -t topo.clab.yml
```

`stop` sets the Clabernetes ignore-reconcile label and scales the selected node
Deployments to `0`:

```text
clabernetes/ignoreReconcile=true
```

The label prevents the Clabernetes manager from immediately reconciling the
nodes back to the running state.

`start` scales the selected Deployments back to `1` and clears the
ignore-reconcile label when all nodes are running again.

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

The c9s runtime can stream topology, pod, and interface-stat events:

```bash
containerlab --runtime clabernetes events --format json
containerlab --runtime clabernetes events --initial-state
containerlab --runtime clabernetes events --interface-stats --format json
```

For c9s, events do not come from Docker events on the outer host. Containerlab
watches:

- Clabernetes `Topology` resources
- Pods labeled with `clabernetes/topologyOwner`

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

The main kubernetes resource is:

```bash
kubectl -n <namespace> get topology <lab> -o yaml
```

Related resources are selected with Clabernetes labels:

```bash
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l clabernetes/topologyOwner=<lab>
```

To find one node launcher pod:

```bash
kubectl -n <namespace> get pod \
  -l clabernetes/topologyOwner=<lab>,clabernetes/topologyNode=<node>
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

- create, get, list, watch, update, and delete Clabernetes `Topology` resources
- list and watch Pods
- list, get, and update Deployments
- exec into launcher Pods with `pods/exec`

Useful checks:

```bash
kubectl auth can-i get topologies.clabernetes.containerlab.dev -n <namespace>
kubectl auth can-i create topologies.clabernetes.containerlab.dev -n <namespace>
kubectl auth can-i update topologies.clabernetes.containerlab.dev -n <namespace>
kubectl auth can-i delete topologies.clabernetes.containerlab.dev -n <namespace>
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

### Namespace does not exist

Typical symptoms:

```text
failed to create clabernetes topology <namespace>/<lab>: namespaces "<namespace>" not found
```

Check:

```bash
kubectl get namespace <namespace>
echo "$CLAB_KUBE_NAMESPACE"
kubectl config view --minify --output 'jsonpath={..namespace}{"\n"}'
```

Create the namespace or select an existing one:

```bash
kubectl create namespace <namespace>
export CLAB_KUBE_NAMESPACE=<namespace>
```

### CRDs are missing

The c9s runtime talks to:

```text
topologies.clabernetes.containerlab.dev
```

Typical symptoms:

```text
the server could not find the requested resource
```

Check:

```bash
kubectl api-resources | grep -i clabernetes
kubectl get crd topologies.clabernetes.containerlab.dev
```

Install Clabernetes and its CRDs before using `--runtime clabernetes`.

### Manager is not reconciling

The CRD may exist and the `Topology` resource may be created, but no node
Deployments or Pods appear.

Check:

```bash
kubectl get pods -A | grep -i clabernetes
kubectl -n <namespace> get topology <lab> -o yaml
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l clabernetes/topologyOwner=<lab>
```

If deploy waits until timeout, check the Clabernetes manager logs and verify
that it watches the namespace where the `Topology` was created.

### Topology reports deployfailed

During deploy, containerlab fails immediately if Clabernetes reports:

```text
status.topologyState=deployfailed
```

Check:

```bash
kubectl -n <namespace> get topology <lab> -o yaml
kubectl -n <namespace> describe topology <lab>
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l clabernetes/topologyOwner=<lab>
```

Common causes include bad topology data, image pull failures, missing pull
secrets, unsupported node settings, pod security policy, or a launcher pod that
cannot run nested Docker.

### Inspect shows no containers

For c9s, `inspect` looks for Clabernetes topologies, not local Docker
containers.

Check:

```bash
containerlab --runtime clabernetes inspect --all
kubectl get topologies -A
echo "$CLAB_KUBE_NAMESPACE"
```

If `docker ps` on the outer host is empty, that can be perfectly normal for c9s.
The node containers live inside launcher pods.

### Exec, save, or stats cannot reach a node

`exec`, `save`, `save --copy`, and `events --interface-stats` need pod exec into
the launcher pod.

Check:

```bash
kubectl -n <namespace> get pod \
  -l clabernetes/topologyOwner=<lab>,clabernetes/topologyNode=<node> \
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

Known differences:

- `deploy --node-filter` is not supported.
- Local Docker commands on the outer host are not authoritative for c9s labs.
- Local network namespace features are not equivalent in c9s.
- `inspect interfaces` and host-side `tc` or netem operations do not have the
  same local namespace access they have with Docker labs.
- Some `tools` commands create local helper containers and are not modeled as
  Clabernetes `Topology` resources.
- Per-node `runtime: docker` or `runtime: podman` is not the same as selecting
  the global `clabernetes` lab runtime.
- Two c9s labs can have the same lab name in different namespaces.

/// note
Use kubernetes and launcher-pod state as the source of truth for c9s labs:

```bash
kubectl get topologies -A
kubectl -n <namespace> get deploy,pod,svc,cm,pvc \
  -l clabernetes/topologyOwner=<lab>
```
///
