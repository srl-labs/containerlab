# Clabernetes Quickstart

The best way to understand how clabernetes works is to walk through a short example where we create a three-node k8s
cluster and deploy a lab there.

This quickstart uses [kind](https://kind.sigs.k8s.io/) to create a local kubernetes cluster and
then deploys clabernetes into. Once clanernetes is installed we deploy a small
[topology with two SR Linux nodes](../../lab-examples/two-srls.md) connected back to back together.

Once the lab is deployed, we explain how clabverter & clabernetes work in unison to to make the original topology files deployable onto the cluster
with tunnels stitching lab nodes together to form point to point connections between the nodes.  
Buckle up!

## Creating a cluster

Clabernetes goal is to allow users to run networking labs with containerlab's simplicity and ease of use but with the scaling powers of kubernetes. To simulate the scaling aspect, we'll use [`kind`](https://kind.sigs.k8s.io/) to create a local multi-node kubernetes cluster. If you already have a k8s cluster, feel free to use it instead.

With the following command we instruct kind to setup a three node k8s cluster with two worker and one control plane nodes.

```bash
kind create cluster --name c9s --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
  - role: worker
EOF
```

Don't forget to install [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/)!

When the cluster is ready we can proceed with installing clabernetes.

## Installing clabernetes

Clabernetes is [installed](install.md) into a kubernetes cluster using [helm](https://helm.sh):

We use `alpine/helm` container image here instead of installing the tool locally; you can skip this step if you already have `helm` installed.

```bash
alias helm="docker run --network host -ti --rm -v $(pwd):/apps -w /apps \
    -v ~/.kube:/root/.kube -v ~/.helm:/root/.helm \
    -v ~/.config/helm:/root/.config/helm \
    -v ~/.cache/helm:/root/.cache/helm \
    alpine/helm:3.12.3"
```

```bash title="Installing latest clabernetes version"
helm upgrade --install \
    clabernetes oci://ghcr.io/srl-labs/clabernetes/clabernetes
```

A successful installation will result in a `clabernetes-manager` deployment of three pods running in
the cluster:

```bash
$ kubectl get pods -o wide #(1)!
NAME                                   READY   STATUS     RESTARTS   AGE   IP           NODE          NOMINATED NODE   READINESS GATES
clabernetes-manager-7d8d9b4785-9xbnn   0/1     Init:0/1   0          27s   10.244.2.2   c9s-worker    <none>           <none>
clabernetes-manager-7d8d9b4785-hpc6h   0/1     Init:0/1   0          27s   10.244.1.3   c9s-worker2   <none>           <none>
clabernetes-manager-7d8d9b4785-z47dr   1/1     Running    0          27s   10.244.1.2   c9s-worker2   <none>           <none>
```

1. Note, that `clabernetes-manager` is installed as a 3-node deployment, and you can see that two pods might be in Init stay for a little while until the leader election is completed.

We will also need `clabverter` CLI to convert containerlab topology files to clabernetes manifests. As per clabverter [installation instructions](install.md#clabverter) we will setup an alias for its latest version:

--8<-- "docs/manual/clabernetes/install.md:cv-install"

## Installing Load Balancer

To get access to the nodes deployed by clabernetes from outside of the k8s cluster we will install need a load balancer. Any load balancer will do, we will use up [kube-vip](https://kube-vip.io/) in this quickstart. Moreover, if no external access to the nodes is required, load balancer installation can be skipped altogether.

Following [kube-vip + kind](https://kube-vip.io/docs/usage/kind/) installation instructions we execute the following commands:

```bash
kubectl apply -f https://kube-vip.io/manifests/rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/kube-vip/kube-vip-cloud-provider/main/manifest/kube-vip-cloud-controller.yaml
kubectl create configmap --namespace kube-system kubevip --from-literal range-global=172.18.1.10-172.18.1.250
```

Next we setup kube-vip's CLI tool:

```bash
KVVERSION=$(curl -sL https://api.github.com/repos/kube-vip/kube-vip/releases | jq -r ".[0].name")
alias kube-vip="docker run --network host --rm ghcr.io/kube-vip/kube-vip:$KVVERSION"
```

And install kube-vip load balancer daemonset in ARP mode:

```bash
kube-vip manifest daemonset --services --inCluster --arp --interface eth0 | kubectl apply -f -
```

We can check kube-vip daemonset pods are running on both worker nodes:

```bash
$ kubectl get pods -A -o wide | grep kube-vip
kube-system          kube-vip-cloud-provider-54c878b6c5-qwvf5    1/1     Running   0          91s   10.244.0.5   c9s-control-plane   <none>           <none>
kube-system          kube-vip-ds-fj7qp                           1/1     Running   0          9s    172.18.0.3   c9s-worker2         <none>           <none>
kube-system          kube-vip-ds-z8q67                           1/1     Running   0          9s    172.18.0.4   c9s-worker          <none>           <none>
```

## Deploying a topology

Clabernetes biggest advantage is that it uses the same topology file format as containerlab; as much as possible. Undestandably though, the original [Containerlab's topology file](../../manual/topo-def-file.md) is not something you can deploy on k8s as is.  
We've created a converter tool called `clabverter` that takes containerlab topology file and converts it to kubernetes manifests. The manifests can then be deployed on a k8s cluster.

So how do we do that? Just enter the directory where original `clab.yml` file is located and let `clabverter` do its job. With the [Two SR Linux nodes](../../lab-examples/two-srls.md) lab this would look like this:

```bash
❯ cd lab-examples/srl02/

❯ ls
srl02.clab.yml  srl1.cfg  srl2.cfg
```

 Take a look at manifest file that defines a simple
[2-node topology](https://github.com/srl-labs/clabernetes/blob/main/examples/two-srl.c9s.yml) consisting of two SR Linux nodes:

```yaml
---
apiVersion: topology.clabernetes/v1alpha1
kind: Containerlab
metadata:
  name: clab-srl02
  namespace: srl02
spec:
  # nodes' startup config is omitted for brevity
  config: |-
    name: srl02
    topology:
      nodes:
        srl1:
          kind: srl
          image: ghcr.io/nokia/srlinux:23.7.1

        srl2:
          kind: srl
          image: ghcr.io/nokia/srlinux:23.7.1
      links:
        - endpoints: ["srl1:e1-1", "srl2:e1-1"]
```

As you can see, the familiar Containerlab topology is simply wrapped in a `Containerlab` Custom
Resource. The `spec.config` field contains the containerlab's topology definition in its entirety.

The `metadata.name` field is the name of the topology. The `metadata.namespace` field is the namespace in which the topology
will be deployed.

!!!note
    1. We use `c9s.yml` extension for clabernetes manifests. Because clabernetes == c9s.
    2. A `clabverter` CLI will be available to users to convert existing containerlab `clab.yml` files to `c9s.yml` manifests!

Before deploying this lab we need to create the namespace as set in our Clabernetes resource:

```bash
kubectl create namespace srl02
```

And now we are ready to deploy our first clabernetes topology by downloading it from clabernetes repo and feeding to kubectl:

```bash
curl -sL https://raw.githubusercontent.com/srl-labs/clabernetes/main/examples/two-srl.c9s.yml | \
kubectl apply -f -
```

## Verifying the deployment

Once the topology is deployed, clabernetes will do its magic. Let's verify run some verification commands to see what objects get created:

Starting with listing `Containerlab` CRs in the `srl02` namespace we can see it is available:

```bash
$ kubectl get --namespace srl02 Containerlab
NAME         AGE
clab-srl02   26m
```

Looking in the Containerlab CR we can see that clabernetes took the topology definition from the `spec.config` field and split it to sub-topologies that are outlined in the `status.configs` section
of the resource:

```bash
kubectl get --namespace srl02 Containerlabs clab-srl02 -o yaml
```

```yaml
# --snip--
status:
  configs: |
    srl1:
        name: clabernetes-srl1
        prefix: null
        topology:
            defaults:
                ports:
                    - 60000:21/tcp
                    # here goes a list of ports exposed
                    # by default
            nodes:
                srl1:
                    kind: srl
                    image: ghcr.io/nokia/srlinux:23.7.1
            links:
                - endpoints:
                    - srl1:e1-1
                    - host:srl1-e1-1
        debug: false
    srl2:
        name: clabernetes-srl2
        prefix: null
        topology:
            nodes:
                srl2:
                    kind: srl
                    image: ghcr.io/nokia/srlinux:23.7.1
            links:
                - endpoints:
                    - srl2:e1-1
                    - host:srl2-e1-1
```

The subtopologies are then deployed as deployments (which in their turn create pods) in the cluster, and
containerlab that runs inside each pod deploys the topology as it would normally do on a single node:

```bash title="Listing pods in srl02 namespace"
$ kubectl get pods --namespace srl02 -o wide
NAME                               READY   STATUS    RESTARTS   AGE   IP           NODE           NOMINATED NODE   READINESS GATES
clab-srl02-srl1-7bf78d568c-jw9q6   1/1     Running   0          24s   10.244.1.12   kind-worker   <none>           <none>
clab-srl02-srl2-59fb9465d-k5vkb    1/1     Running   0          24s   10.244.1.11   kind-worker   <none>           <none>
```

We see that two pods are running on different worker nodes (they may run on the same node, if schedulers decides so).
These pods run containerlab inside in a docker-in-docker mode and each node deploys a subset of
the original topology. We can enter the pod and use containerlab CLI to verify the topology:

```bash
kubectl exec -n srl02 -it clab-srl02-srl1-7bf78d568c-jw9q6 -- bash
```

And in the pod's shell we swim in the familiar containerlab waters:

```bash
root@clab-srl02-srl1-77f7585fbc-m9v54:/clabernetes# clab ins -a
+---+-----------+------------------+-----------------------------------+--------------+------------------------------+------+---------+----------------+----------------------+
| # | Topo Path |     Lab Name     |               Name                | Container ID |            Image             | Kind |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-----------+------------------+-----------------------------------+--------------+------------------------------+------+---------+----------------+----------------------+
| 1 | topo.yaml | clabernetes-srl1 | clabernetes-clabernetes-srl1-srl1 | 0a16495fb358 | ghcr.io/nokia/srlinux:23.7.1 | srl  | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
+---+-----------+------------------+-----------------------------------+--------------+------------------------------+------+---------+----------------+----------------------+
```

We can `cat topo.yaml` to see the subset of a topology that containerlab started in this pod.

!!!note
    It is worth reiterating it again, unmodified containerlab runs inside a pod as if it runs in a Linux system in a standalone mode. It has access to the Docker API and schedules nodes in exactly the same way as if no k8s exists.

## Accessing the nodes

There are two common ways to access the network OS'es shell:

1. External access using Load Balancer service.
2. Getting access to the pod's shell and from there login to the nested NOS. No LB is required.

We are going to show you both options.

### Load Balancer

Adding a Load Balancer to the k8s cluster makes accessing the nodes almost as easy when working with containerlab.
Kube-vip load balancer that we added a few steps before is going to create a LoadBalancer k8s service for each exposed
port.

By default, clabernetes exposes[^1] the following ports for each lab node:

| Protocol | Ports                                                                             |
| -------- | --------------------------------------------------------------------------------- |
| tcp      | `21`, `80`, `443`, `830`, `5000`, `5900`, `6030`, `9339`, `9340`, `9559`, `57400` |
| udp      | `161`                                                                             |

The good work that LB is doing can be seen by running:

```bash
$ kubectl get -n srl02 svc --field-selector='type=LoadBalancer'
NAME                 TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)                                                                                                                                                                                                   AGE
clab-srl02-srl1      LoadBalancer   10.96.130.76    172.18.1.10   161:31968/UDP,21:32230/TCP,22:30722/TCP,23:32329/TCP,80:31708/TCP,443:30392/TCP,830:30954/TCP,5000:30852/TCP,5900:30031/TCP,6030:31250/TCP,9339:32620/TCP,9340:30104/TCP,9559:32624/TCP,57400:30806/TCP   39s
clab-srl02-srl1-vx   ClusterIP      10.96.198.42    <none>        4789/UDP                                                                                                                                                                                                  40s
clab-srl02-srl2      LoadBalancer   10.96.227.153   172.18.1.11   161:32330/UDP,21:31274/TCP,22:31321/TCP,23:31466/TCP,80:30947/TCP,443:32154/TCP,830:32300/TCP,5000:30241/TCP,5900:30688/TCP,6030:30977/TCP,9339:30847/TCP,9340:32687/TCP,9559:31699/TCP,57400:32644/TCP   39s
clab-srl02-srl2-vx   ClusterIP      10.96.161.133   <none>        4789/UDP
```

The two LoadBalancer services provide external IPs (`172.18.1.10` and `172.18.1.11`) for the lab nodes. The long list of ports are the ports clabernetes exposes by default which spans both regular SSH as well as other common automation interfaces.

You can immediately SSH into one of the nodes using its External-IP:

```
❯ ssh admin@172.18.1.12
The authenticity of host '172.18.1.12 (172.18.1.12)' can't be established.
ED25519 key fingerprint is SHA256:kpR071TK0ll86/vdYbsDE+PBI81RoYigZTRugTgFbl8.
This key is not known by any other names
Are you sure you want to continue connecting (yes/no/[fingerprint])? yes
Warning: Permanently added '172.18.1.12' (ED25519) to the list of known hosts.
................................................................
:                  Welcome to Nokia SR Linux!                  :
:              Open Network OS for the NetOps era.             :
:                                                              :
:    This is a freely distributed official container image.    :
:                      Use it - Share it                       :
:                                                              :
: Get started: https://learn.srlinux.dev                       :
: Container:   https://go.srlinux.dev/container-image          :
: Docs:        https://doc.srlinux.dev/23-7                    :
: Rel. notes:  https://doc.srlinux.dev/rn23-7-1                :
: YANG:        https://yang.srlinux.dev/v23.7.1                :
: Discord:     https://go.srlinux.dev/discord                  :
: Contact:     https://go.srlinux.dev/contact-sales            :
................................................................

admin@172.18.1.12's password: 
Using configuration file(s): []
Welcome to the srlinux CLI.
Type 'help' (and press <ENTER>) if you need any help using this.
--{ running }--[  ]--
A:srl1#  
```

Other services, like gNMI, JSON-RPC, SNMP are available as well since those ports are already exposed.

### Pod Shell

Load Balancer makes it easy to get external access to the lab nodes, but don't panic if for whatever reason you can't install one.
It is still possible to access the nodes without LB, it will just be less convenient and powerful.

For example, to access `srl1` lab node in our k8s cluster we just need to figure out which pod runs this node.

Since all pods are named after the nodes they are running, we can find the right one by listing all pods in a namespace:

```bash
$ kubectl get pods -n srl02
NAME                               READY   STATUS    RESTARTS   AGE
clab-srl02-srl1-55477468c4-vprj4   1/1     Running   0          23m
clab-srl02-srl2-5b948687d5-pwhlr   1/1     Running   0          23m
```

Looking at the pod nameed `clab-srl02-srl1-55477468c4-vprj4` we understand that it runs `srl1` node we specified in the topology. To get shell access to this node we can run:

```bash
kubectl -n srl02 exec -it clab-srl02-srl1-55477468c4-vprj4 -- ssh admin@srl1
```

We essentially execute `ssh admin@srl1` command inside the pod, as you'd normally do with containerlab.

## Datapath stitching

One of the challanges associated with distributed labs is to enable connectivity between the nodes as per user's intent.

Thanks to k8s and accompanying Load Balancer service, the management network access is taken care of. You get access to the management interfaces of each pod out of the box. But what about the data plane?

In containerlab this is easy to solve by creating the veth pairs between the nodes, but things are a bit more complicated in distributed environments like k8s.

Remember our manifest file we deployed in the beginning of this quickstart? It had a single link between two nodes defined in the same way you'd do it in containerlab:

```yaml title=""
# snip
links:
  - endpoints: ["srl1:e1-1", "srl2:e1-1"]
```

How does clabernetes layout this link when the lab nodes srl1 and srl2 can be scheduled on different worker nodes? Well, clabernetes takes original link definition as provided by a user and transforms it into a set of point-topoint VXLAN tunnels[^2] that stitch the nodes together.

Two nodes appear to be connected to each other as if they were connected with a veth pair. We can check that LLDP neighbors are discovered on either other side of the link:

```srl

```

???warning "VXLAN and MTU"
    VXLAN tunnels are susceptible to MTU issues. Check the MTU value for `vx-*` link in your pod to see what value has been set by the kernel and adjust your node's link/IP MTU accordingly.

    ```bash
    root@clab-srl02-srl1-55477468c4-vprj4:/clabernetes# ip l | grep vx
    9: vx-srl1-e1-1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    ```
    In our kind cluster that has a single network attached, the VXLAN tunnel is routed through the management network interface of the pod. It is possible to configure kind nodes to have more than one network and therefore have a dedicated network for the VXLAN tunnels with a higher MTU value.

[^1]: Default exposed ports can be ovewritten by a user via Containerlab CR.
[^2]: Using containerlab's [vxlan tunneling workflow](../../manual/multi-node.md#vxlan-tunneling) to create tunnels.
