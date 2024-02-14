# Clabernetes Quickstart

The best way to understand how clabernetes works is to walk through a short example where we deploy a simple but representative [lab](https://learn.srlinux.dev/blog/2024/vlans-on-sr-linux/#the-lab) using clabernetes.

Do you already have a kubernetes cluster? Great! You can skip the cluster creation step and jump straight to [Installing Clabernetes](install.md) part.

But if you don't have a cluster yet, don't panic, we'll create one together. We are going to use [kind](https://kind.sigs.k8s.io/) to create a local kubernetes cluster and
then install clabernetes into it. Once clabernetes is installed we deploy a small
[topology with two SR Linux nodes and two client nodes](https://learn.srlinux.dev/blog/2024/vlans-on-sr-linux/#the-lab).

If all goes to plan, the lab will be successfully deployed! Clabverter & clabernetes work in unison to make the original topology files deployable onto the cluster
with tunnels stitching lab nodes together to form point to point connections between the nodes.

Let's see how it all works, buckle up!

## Creating a cluster

Clabernetes goal is to allow users to run networking labs with containerlab's simplicity and ease of use, but with the scaling powers of kubernetes. Surely, it is best to have a real deal available to you, but for demo purposes we'll use [`kind`](https://kind.sigs.k8s.io/) to create a local multi-node kubernetes cluster. If you already have a k8s cluster, feel free to use it instead -- clabernetes can run in any kubernetes cluster[^1]!

With the following command we instruct kind to set up a three node k8s cluster with two worker and one control plane nodes.

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

<!-- --8<-- [start:helm-alias] -->
```bash
alias helm="docker run --network host -ti --rm -v $(pwd):/apps -w /apps \
    -v ~/.kube:/root/.kube -v ~/.helm:/root/.helm \
    -v ~/.config/helm:/root/.config/helm \
    -v ~/.cache/helm:/root/.cache/helm \
    alpine/helm:3.12.3"
```
<!-- --8<-- [end:helm-alias] -->

--8<-- "docs/manual/clabernetes/install.md:chart-install"

Note, that we install clabernetes in a `c9s` namespace. This is not a requirement, but it is a good practice to keep clabernetes manager deployment in a separate namespace.

A successful installation will result in a `clabernetes-manager` deployment of three pods running in
the cluster:

```{.bash .no-select}
kubectl get -n c9s pods -o wide --namespace clabernetes #(1)!
```

1. Note, that `clabernetes-manager` is installed as a 3-node deployment, and you can see that two pods might be in Init stay for a little while until the leader election is completed.

<div class="embed-result">
```
NAME                                   READY   STATUS    RESTARTS   AGE    IP            NODE          NOMINATED NODE   READINESS GATES
clabernetes-manager-7ccb98897c-7ctnt   1/1     Running   0          103s   10.244.2.15   c9s-worker    <none>           <none>
clabernetes-manager-7ccb98897c-twzxw   1/1     Running   0          96s    10.244.1.15   c9s-worker2   <none>           <none>
clabernetes-manager-7ccb98897c-xhgkl   1/1     Running   0          103s   10.244.1.14   c9s-worker2   <none>           <none>
```
</div>

## Installing Load Balancer

To get access to the nodes deployed by clabernetes from outside the k8s cluster we need a load balancer. Any load balancer will do, but we will use [kube-vip](https://kube-vip.io/) in this quickstart.

/// note
Load Balancer installation can be skipped if you don't need external access to the lab nodes. You can still access the nodes from inside the cluster by entering the pod's shell and then logging into the node.
///

Following [kube-vip + kind](https://kube-vip.io/docs/usage/kind/) installation instructions we execute the following commands:

```bash
kubectl apply -f https://kube-vip.io/manifests/rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/kube-vip/kube-vip-cloud-provider/main/manifest/kube-vip-cloud-controller.yaml
kubectl create configmap --namespace kube-system kubevip \
  --from-literal range-global=172.18.1.10-172.18.1.250
```

Next we set up the kube-vip CLI:

```bash
KVVERSION=$(curl -sL https://api.github.com/repos/kube-vip/kube-vip/releases | \
  jq -r ".[0].name")
alias kube-vip="docker run --network host \
  --rm ghcr.io/kube-vip/kube-vip:$KVVERSION"
```

And install kube-vip load balancer daemonset in ARP mode:

```bash
kube-vip manifest daemonset --services --inCluster --arp --interface eth0 | \
kubectl apply -f -
```

We can check kube-vip daemonset pods are running on both worker nodes:

```{.bash .no-select}
kubectl get pods -A -o wide | grep kube-vip
```

<div class="embed-result">
```bash
kube-system          kube-vip-cloud-provider-54c878b6c5-qwvf5    1/1     Running   0          91s   10.244.0.5   c9s-control-plane   <none>           <none>
kube-system          kube-vip-ds-fj7qp                           1/1     Running   0          9s    172.18.0.3   c9s-worker2         <none>           <none>
kube-system          kube-vip-ds-z8q67                           1/1     Running   0          9s    172.18.0.4   c9s-worker          <none>           <none>
```
</div>

## Clabverter

Clabernetes motto is "containerlab at scale" and therefore we wanted to make it work with the same topology definition file format as containerlab does. Understandably though, the original [Containerlab's topology file](../../manual/topo-def-file.md) is not something you can deploy on Kubernetes cluster as is.

To make sure you have a smooth sailing in the clabernetes waters we've created a clabernetes companion tool called `clabverter`; it takes a containerlab topology file and converts it to several manifests native to Kubernetes and clabernetes. Clabverter then can also apply those manifests to the cluster on your behalf.

Clabverter is not a requirement to run clabernetes, but it is a helper tool to convert containerlab topologies to clabernetes resources and kubernetes objects.

As per clabverter's [installation instructions](install.md#clabverter) we will setup an alias that uses the latest available clabverter container image:

--8<-- "docs/manual/clabernetes/install.md:cv-install"

## Deploying with clabverter

We are now ready to deploy our lab using clabernetes with the help of clabverter. First we clone the lab repository:

```bash title="Cloning the lab"
git clone --depth 1 https://github.com/srl-labs/srlinux-vlan-handling-lab.git \
  && cd srlinux-vlan-handling-lab
```

And then, while standing in the lab directory, let `clabverter` do its job:

```{.bash .no-select title="Converting the containerlab topology to clabernetes manifests and applying it"}
clabverter --stdout | \
kubectl apply -f - #(1)!
```

1. `clabverter` converts the original containerlab topology to a set of k8s manifests and applies them to the cluster.

    We will cover what `clabverter` does in more details in the user manual some time later, but if you're curious, you can check the manifests it generates by running `clabverter --stdout > manifests.yml` and inspecting the `manifests.yml` file.

In the background, `clabverter` created the `Topology` custom resource (CR) in the `c9s-vlan`[^5] namespace that defines our topology and also created a set of config maps for each startup config used in the lab.

## Verifying the deployment

Once clabverter is done, clabernetes controller casts its spell known as *reconciliation* in the k8s world. It takes the spec of the `Topology` CR and creates a set of deployments, config maps and services that are required for lab's operation.

Let's run some verification commands to see what we have in our cluster so far.

Starting with listing `Topology` CRs in the `c9s-vlan` namespace:

``` {.bash .no-select}
kubectl get --namespace c9s-vlan Topology
```

<div class="embed-result">
```
NAME   KIND           AGE
vlan   containerlab   14h
```
</div>

Looking in the Topology CR we can see that the original containerlab topology definition can be found under the `spec.definition.containerlab` field of the custom resource. Clabernetes took the original topology and split it to sub-topologies that are outlined in the `status.configs` section of the resource:

``` {.bash .no-select}
kubectl get --namespace c9s-vlan Topology vlan -o yaml
```

<div class="embed-result" markdown>
/// tab | `spec.config`
```yaml
spec:
  definition:
    containerlab: |-
      name: vlan

      topology:
        nodes:
          srl1:
            kind: nokia_srlinux
            image: ghcr.io/nokia/srlinux:23.10.1
            startup-config: configs/srl.cfg

          srl2:
            kind: nokia_srlinux
            image: ghcr.io/nokia/srlinux:23.10.1
            startup-config: configs/srl.cfg

          client1:
            kind: linux
            image: ghcr.io/srl-labs/alpine
            binds:
              - configs/client.sh:/config.sh
            exec:
              - "ash -c '/config.sh 1'"

          client2:
            kind: linux
            image: ghcr.io/srl-labs/alpine
            binds:
              - configs/client.sh:/config.sh
            exec:
              - "ash -c '/config.sh 2'"

        links:
          # links between client1 and srl1
          - endpoints: [client1:eth1, srl1:e1-1]

          # inter-switch link
          - endpoints: [srl1:e1-10, srl2:e1-10]

          # links between client2 and srl2
          - endpoints: [srl2:e1-1, client2:eth1]

```
///
///tab | `status.configs`
```yaml
# --snip--
status:
  configs:
    client1: |
      name: clabernetes-client1
      prefix: ""
      topology:
          defaults:
              ports:
                  - 60000:21/tcp
                  # here goes a list of exposed ports
          nodes:
              client1:
                  kind: linux
                  image: ghcr.io/srl-labs/alpine
                  exec:
                      - ash -c '/config.sh 1'
                  binds:
                      - configs/client.sh:/config.sh
                  ports: []
          links:
              - endpoints:
                  - client1:eth1
                  - host:client1-eth1
      debug: false
    client2: |
      name: clabernetes-client2
      # similar configuration as for client1
    srl1: |
      name: clabernetes-srl1
      prefix: ""
      topology:
          defaults:
              ports:
                  - 60000:21/tcp
                  # here goes a list of exposed ports
          nodes:
              srl1:
                  kind: nokia_srlinux
                  startup-config: configs/srl.cfg
                  image: ghcr.io/nokia/srlinux:23.10.1
                  ports: []
          links:
              - endpoints:
                  - srl1:e1-1
                  - host:srl1-e1-1
              - endpoints:
                  - srl1:e1-10
                  - host:srl1-e1-10
      debug: false
    srl2: |
      name: clabernetes-srl2
      # similar configuration as for srl1
```

///
</div>

If you take a closer look at the sub-topologies you will see that they are just mini, one-node-each, containerlab topologies. Clabernetes deploys these sub-topologies as deployments in the cluster.

Each deployment pod runs containerlab inside, and containerlab runs the sub topology; each pod deploys the sub-topology as it would normally do on a single node :exploding_head::

``` {.bash .no-select title="Listing pods in c9s-vlan namespace"}
kubectl get pods --namespace c9s-vlan -o wide
```

<div class="embed-result">
```
NAME                            READY   STATUS    RESTARTS   AGE   IP            NODE          NOMINATED NODE   READINESS GATES
vlan-client1-699dbcfd8b-r2fgc   1/1     Running   0          14h   10.244.1.12   c9s-worker2   <none>           <none>
vlan-client2-7db5d589c6-pb8pd   1/1     Running   0          14h   10.244.2.14   c9s-worker    <none>           <none>
vlan-srl1-868f9858cb-xqkbf      1/1     Running   0          14h   10.244.2.13   c9s-worker    <none>           <none>
vlan-srl2-676784b5cb-7gt22      1/1     Running   0          14h   10.244.1.13   c9s-worker2   <none>           <none>
```
</div>

We see four pods running, one pod per each lab node of our original containerlab topology. Pods are scheduled on different worker nodes by the k8s scheduler ensuring optimal resource utilization[^2].

Inside each pod, containerlab runs the sub-topology as if it would run on a standalone Linux system. It has access to the Docker API and schedules nodes in exactly the same way as if no k8s exists.  
We can enter the pod's shell and use containerlab CLI to verify the topology:

```{.bash .no-select}
kubectl exec -it -n c9s-vlan pod/vlan-client1-699dbcfd8b-r2fgc -- bash
```

And in the pod's shell we swim in the familiar containerlab waters:

```{.bash .no-select}
[*]─[client1]─[/clabernetes]
└──> containerlab inspect #(1)!
```

1. If you do not see any nodes in the `inspect` output give it a few minutes, as containerlab is pulling the image and starting the nodes. Monitor this process with `tail -f containerlab.log`.

<div class="embed-result">
```
INFO[0000] Parsing & checking topology file: topo.clab.yaml
+---+---------+--------------+-------------------------+-------+---------+----------------+----------------------+
| # |  Name   | Container ID |          Image          | Kind  |  State  |  IPv4 Address  |     IPv6 Address     |
+---+---------+--------------+-------------------------+-------+---------+----------------+----------------------+
| 1 | client1 | 52757a04756a | ghcr.io/srl-labs/alpine | linux | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
+---+---------+--------------+-------------------------+-------+---------+----------------+----------------------+
```

</div>

We can `cat topo.clab.yaml` to see the subset of a topology that containerlab started in this pod.
///details | `topo.clab.yaml`

```
[*]─[client1]─[/clabernetes]
└──> cat topo.clab.yaml
name: clabernetes-client1
prefix: ""
topology:
    defaults:
        ports:
            - 60000:21/tcp
            - 60001:22/tcp
            - 60002:23/tcp
            - 60003:80/tcp
            - 60000:161/udp
            - 60004:443/tcp
            - 60005:830/tcp
            - 60006:5000/tcp
            - 60007:5900/tcp
            - 60008:6030/tcp
            - 60009:9339/tcp
            - 60010:9340/tcp
            - 60011:9559/tcp
            - 60012:57400/tcp
    nodes:
        client1:
            kind: linux
            image: ghcr.io/srl-labs/alpine
            exec:
                - ash -c '/config.sh 1'
            binds:
                - configs/client.sh:/config.sh
            ports: []
    links:
        - endpoints:
            - client1:eth1
            - host:client1-eth1
debug: false

```

///

It is worth reiterating, that unmodified containerlab runs inside a pod as if it would've run on a Linux system in a standalone mode. It has access to the Docker API and schedules nodes in exactly the same way as if no k8s exists.

## Accessing the nodes

There are two common ways to access the lab nodes deployed with clabernetes:

1. Using external address provided by the Load Balancer service.
2. Entering the pod's shell and from there log in the running lab node. No load balancer required.

We are going to show you both options and you can choose the one that suits you best.

### Load Balancer

Adding a Load Balancer to the k8s cluster makes accessing the nodes almost as easy as when working with containerlab. The kube-vip load balancer that we added before is going to provide an external IP address for a LoadBalancer k8s service that clabernetes creates for each deployment under its control.

By default, clabernetes exposes[^3] the following ports for each lab node:

| Protocol | Ports                                                                             |
| -------- | --------------------------------------------------------------------------------- |
| tcp      | `21`, `80`, `443`, `830`, `5000`, `5900`, `6030`, `9339`, `9340`, `9559`, `57400` |
| udp      | `161`                                                                             |

Let's list the services in the `c9s-vlan` namespace (exluding the VXLAN services[^6]):

```{.bash .no-select}
kubectl get -n c9s-vlan svc | grep -iv vx
```

<div class="embed-result">
```
NAME              TYPE           CLUSTER-IP      EXTERNAL-IP                               PORT(S)                                                                                                                                                                                                   AGE
client1           ExternalName   <none>          vlan-client1.c9s-vlan.svc.cluster.local   <none>                                                                                                                                                                                                    15h
client2           ExternalName   <none>          vlan-client2.c9s-vlan.svc.cluster.local   <none>                                                                                                                                                                                                    15h
srl1              ExternalName   <none>          vlan-srl1.c9s-vlan.svc.cluster.local      <none>                                                                                                                                                                                                    15h
srl2              ExternalName   <none>          vlan-srl2.c9s-vlan.svc.cluster.local      <none>                                                                                                                                                                                                    15h
vlan-client1      LoadBalancer   10.96.232.165   172.18.1.10                               161:32442/UDP,21:32059/TCP,22:32030/TCP,23:30920/TCP,80:31205/TCP,443:32489/TCP,830:31231/TCP,5000:31769/TCP,5900:30902/TCP,6030:31583/TCP,9339:32089/TCP,9340:30311/TCP,9559:30974/TCP,57400:31386/TCP   15h
vlan-client2      LoadBalancer   10.96.164.37    172.18.1.11                               161:30025/UDP,21:31127/TCP,22:30779/TCP,23:30542/TCP,80:31104/TCP,443:32142/TCP,830:30102/TCP,5000:31116/TCP,5900:31559/TCP,6030:30734/TCP,9339:32250/TCP,9340:31922/TCP,9559:30745/TCP,57400:30817/TCP   15h
vlan-srl1         LoadBalancer   10.96.221.110   172.18.1.12                               161:30581/UDP,21:32591/TCP,22:31752/TCP,23:30164/TCP,80:32272/TCP,443:32365/TCP,830:30360/TCP,5000:30618/TCP,5900:30454/TCP,6030:32155/TCP,9339:32736/TCP,9340:32268/TCP,9559:31412/TCP,57400:30100/TCP   15h
vlan-srl2         LoadBalancer   10.96.64.176    172.18.1.13                               161:32109/UDP,21:30903/TCP,22:31495/TCP,23:32174/TCP,80:31128/TCP,443:30720/TCP,830:32017/TCP,5000:30708/TCP,5900:32520/TCP,6030:31586/TCP,9339:31917/TCP,9340:31631/TCP,9559:32731/TCP,57400:32076/TCP   15h
```
</div>

We see the two service types in the `c9s-vlan` namespace: `ExternalName` and `LoadBalancer`.

The LoadBalancer services (implemented by the `kube-vip`) provide external IPs for the lab nodes. The long list of ports are the ports clabernetes exposes by default which spans both regular SSH and other common management interfaces.

For instance, we see that `srl1` node has been assigned `172.18.1.12` IP and we can immediately SSH into it from the outside world using the following command:

```{.text .no-select}
ssh admin@172.18.1.12
```

<div class="embed-result">
```
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
</div>

Other services, like gNMI, JSON-RPC, SNMP are available as well since those ports are already exposed.

///details | "gNMI access"
    type: example

```{.bash .no-select}
gnmic -a 172.18.1.12 -u admin -p 'NokiaSrl1!' --skip-verify -e json_ietf \
  get --path /system/information/version
```

<div class="embed-result">
```
[
  {
    "source": "172.18.1.12",
    "timestamp": 1707828542585726740,
    "time": "2024-02-13T14:49:02.58572674+02:00",
    "updates": [
      {
        "Path": "srl_nokia-system:system/srl_nokia-system-info:information/version",
        "values": {
          "srl_nokia-system:system/srl_nokia-system-info:information/version": "v23.10.1-218-ga3fc1bea5a"
        }
      }
    ]
  }
]
```
</div>
///

The `ExternalName` services are used to provide DNS resolution for the lab nodes. They are not accessible from outside the cluster, but they can be used by other pods in the same namespace to resolve the lab nodes' names to their IPs.

For instance, the pods in the `c9s-vlan` namespace can resolve the `srl1` node's name to its IP. This enables name resolution workflow similar to what you'd have in a regular containerlab deployment.

### Pod Shell

Load Balancer makes it easy to get external access to the lab nodes, but don't panic if for whatever reason you can't install one. It is still possible to access the nodes without LB!

For example, to access `srl1` lab node in our k8s cluster we may leverage `kubectl exec` command to get to the shell of the pod that runs `srl1` node.

/// note
You may have a stellar experience with [`k9s` project](https://k9scli.io/) that offers a terminal UI to interact with k8s clusters. It is a great tool to have in your toolbox.

If Terminal UI is not your jam, take a look at `kubectl` [shell completions](https://kubernetes.io/docs/reference/kubectl/quick-reference/#kubectl-autocomplete). They come in super handy, install them if you haven't yet.
///

Since all pods are named after the nodes they are running, we can find the right one by listing all pods in a namespace:

```bash
kubectl get pods -n c9s-vlan
```

<div class="embed-result">
```
NAME                            READY   STATUS    RESTARTS   AGE
vlan-client1-699dbcfd8b-r2fgc   1/1     Running   0          16h
vlan-client2-7db5d589c6-pb8pd   1/1     Running   0          16h
vlan-srl1-868f9858cb-xqkbf      1/1     Running   0          16h
vlan-srl2-676784b5cb-7gt22      1/1     Running   0          16h
```
</div>

Looking at the pod named `vlan-srl1-868f9858cb-xqkbf` we understand that it runs `srl1` node we specified in the topology. To get shell access to this node we can run:

```{.bash .no-select}
kubectl -n c9s-vlan exec -it vlan-srl1-868f9858cb-xqkbf -- ssh admin@srl1
```

We essentially execute `ssh admin@srl1` command inside the pod, as you'd normally do with containerlab.

## Datapath stitching

One of the challenges associated with distributed labs is to enable connectivity between the nodes as per user's intent.

Thanks to k8s and accompanying Load Balancer service, the management network access is taken care of. You get access to the management interfaces of each pod out of the box. But what about the non-management links we defined in the original topology file?

In containerlab the links defined in the topology most often represented by the veth pairs between the nodes, but things are a bit more complicated in distributed environments like k8s.

Remember our manifest file we deployed in the beginning of this quickstart? It had a single link between two nodes defined in the same way you'd do it in containerlab:

```yaml title=""
# snip
links:
  - endpoints: ["srl1:e1-10", "srl2:e1-10"]
```

How does clabernetes layout this link when the lab nodes srl1 and srl2 can be scheduled on different worker nodes? Well, clabernetes takes the original link definition as provided by a user and transforms it into a set of point-to-point VXLAN tunnels[^4] that stitch the nodes together.

Two nodes appear to be connected to each other as if they were connected with a veth pair. We can check that LLDP neighbors are discovered on either other side of the link:

```{.bash .no-select}
kubectl -n c9s-vlan exec -it vlan-srl1-868f9858cb-xqkbf -- \
    ssh admin@srl1 #(1)!
```

1. Logging to `srl1` node

<div class="embed-result">
```
Last login: Fri Sep 22 23:07:30 2023 from 2001:172:20:20::1
Using configuration file(s): []
Welcome to the srlinux CLI.
Type 'help' (and press <ENTER>) if you need any help using this.
```
</div>
<div class="embed-result">
```srl
--{ running }--[  ]--
A:srl1# show system lldp neighbor
```
</div>
<div class="embed-result">
```
A:srl1# show system lldp neighbor
  +---------------+----------------+----------------+---------------+---------------+---------------+---------------+
  |     Name      |    Neighbor    |    Neighbor    |   Neighbor    |   Neighbor    | Neighbor Last | Neighbor Port |
  |               |                |  System Name   |  Chassis ID   | First Message |    Update     |               |
  +===============+================+================+===============+===============+===============+===============+
  | ethernet-1/10 | 1A:00:00:FF:00 | srl2           | 1A:00:00:FF:0 | 16 hours ago  | 3 seconds ago | ethernet-1/10 |
  |               | :00            |                | 0:00          |               |               |               |
  +---------------+----------------+----------------+---------------+---------------+---------------+---------------+
```
</div>

We can also make sure that our startup-configuration that was provided in [external files](https://github.com/srl-labs/srlinux-vlan-handling-lab/blob/main/configs) in original topology is applied in good order and we can perform the ping between two clients

```bash
kubectl exec -it -n c9s-vlan pod/vlan-client1-699dbcfd8b-r2fgc -- \
docker exec -it client1 ping -c 2 10.1.0.2
```

<div class="embed-result">
```text
PING 10.1.0.2 (10.1.0.2) 56(84) bytes of data.
64 bytes from 10.1.0.2: icmp_seq=1 ttl=64 time=2.08 ms
64 bytes from 10.1.0.2: icmp_seq=2 ttl=64 time=1.04 ms

--- 10.1.0.2 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1001ms
rtt min/avg/max/mdev = 1.040/1.557/2.075/0.517 ms

```

</div>

With the command above we:

1. connected to the `vlan-client1-699dbcfd8b-r2fgc` that runs the `client1` node
2. executed `ping` command inside the `client1` node to ping the `client2` node
3. Ensured that the datapath stitching is working as expected

/// details | VXLAN and MTU
    type: warning
VXLAN tunnels are susceptible to MTU issues. Check the MTU value for `vx-*` link in your pod to see what value has been set by the kernel and adjust your node's link/IP MTU accordingly.

```bash
[*]─[srl1]─[/clabernetes]
└──> ip l | grep vx
11: vx-srl1-e1-1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
12: vx-srl1-e1-10: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
```

In our kind cluster that has a single network attached, the VXLAN tunnel is routed through the management network interface of the pod. It is possible to configure kind nodes to have more than one network and therefore have a dedicated network for the VXLAN tunnels with a higher MTU value.
///

## VM-based nodes?

In this quickstart we used native containerized Network OS - [SR Linux](https://learn.srlinux.dev) - as it is lightweight and publicly available. But what if you want to use a VM-based Network OS like Nokia SR OS, Cisco IOS-XRv or Juniper vMX? Can you do that with clabernetes?

Short answer is yes. Clabernetes should be able to run VM-based nodes as well, but your cluster nodes must support nested virtualization, same as you would need to run VM-based nodes in containerlab.

Also you need to ensure that your VM-based container image is accessible to your cluster nodes, either via a public registry or a private one.

When these considerations are taken care of, you can use the same topology file as you would use with containerlab. The only difference is that you need to specify the image in the topology file as a fully qualified image name, including the registry name.

[^1]: In general there are no requirements for clabernetes from a kubernetes cluster perspective, however, many device types may have requirements for nested virtualization or specific CPU flags that your nodes would need to support in order to run the device.
[^2]: They may run on the same node, this is up to the kubernetes scheduler whose job it is to schedule pods on the nodes it deems most appropriate.
[^3]: Default exposed ports can be overwritten by a user via Topology CR.
[^4]: Using containerlab's [vxlan tunneling workflow](../../manual/multi-node.md#vxlan-tunneling) to create tunnels.
[^5]: The namespace name is derived from the name of the lab in the `.clab.yml` file.
[^6]: VXLAN services are used for datapath stitching and are not meant to be accessed from outside the cluster.
