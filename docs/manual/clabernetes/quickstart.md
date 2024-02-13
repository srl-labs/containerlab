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
kubectl get -n c9s pods -o wide #(1)!
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
└──> containerlab inspect
```

con <div class="embed-result">

```
INFO[0000] Parsing & checking topology file: topo.clab.yaml
+---+---------+--------------+-------------------------+-------+---------+----------------+----------------------+
| # |  Name   | Container ID |          Image          | Kind  |  State  |  IPv4 Address  |     IPv6 Address     |
+---+---------+--------------+-------------------------+-------+---------+----------------+----------------------+
| 1 | client1 | 52757a04756a | ghcr.io/srl-labs/alpine | linux | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
+---+---------+--------------+-------------------------+-------+---------+----------------+----------------------+
```

</div>

If you do not see any nodes in the `inspect` output give it a few minutes, as containerlab is pulling the image and starting the nodes. The logs of this process can be seen by running `tail -f clab.log`.

We can `cat topo.clab.yaml` to see the subset of a topology that containerlab started in this pod.

!!!note
    It is worth repeating that unmodified containerlab runs inside a pod as if it would've run on a Linux system in a standalone mode. It has access to the Docker API and schedules nodes in exactly the same way as if no k8s exists.

## Accessing the nodes

There are two common ways to access the lab nodes deployed by clabernetes:

1. External access using Load Balancer service.
2. Entering the pod's shell and from there login to the running NOS. No LB is required.

We are going to show you both options.

### Load Balancer

Adding a Load Balancer to the k8s cluster makes accessing the nodes almost as easy as when working with containerlab. The kube-vip load balancer that we added a few steps before is going to create a LoadBalancer k8s service for each exposed port.

By default, clabernetes exposes[^3] the following ports for each lab node:

| Protocol | Ports                                                                             |
| -------- | --------------------------------------------------------------------------------- |
| tcp      | `21`, `80`, `443`, `830`, `5000`, `5900`, `6030`, `9339`, `9340`, `9559`, `57400` |
| udp      | `161`                                                                             |

The good work that LB is doing can be listing services in the `clabernetes` namespace:

```{.bash .no-select}
kubectl get -n clabernetes svc | grep -iv vx
```

<div class="embed-result">
```
NAME            TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)                                                                                                                                                                                                   AGE
srl02-srl1      LoadBalancer   10.96.120.2     172.18.1.10   161:30700/UDP,21:32307/TCP,22:31202/TCP,23:32412/TCP,80:31940/TCP,443:31832/TCP,830:30576/TCP,5000:30702/TCP,5900:31502/TCP,6030:31983/TCP,9339:31113/TCP,9340:30835/TCP,9559:32702/TCP,57400:32037/TCP   125m
srl02-srl2      LoadBalancer   10.96.175.237   172.18.1.11   161:30810/UDP,21:32208/TCP,22:31701/TCP,23:31177/TCP,80:31229/TCP,443:31872/TCP,830:32395/TCP,5000:31799/TCP,5900:30292/TCP,6030:31442/TCP,9339:32298/TCP,9340:30475/TCP,9559:32595/TCP,57400:31253/TCP   125m
```
</div>

The two LoadBalancer services provide external IPs (`172.18.1.10` and `172.18.1.11`) for the lab nodes. The long list of ports are the ports clabernetes exposes by default which spans both regular SSH as well as other common automation interfaces.

You can immediately SSH into one of the nodes using its External-IP:

```{.text .no-select}
ssh admin@172.18.1.10
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

admin@172.18.1.10's password:
Using configuration file(s): []
Welcome to the srlinux CLI.
Type 'help' (and press <ENTER>) if you need any help using this.
--{ running }--[  ]--
A:srl1#  

```
</div>

Other services, like gNMI, JSON-RPC, SNMP are available as well since those ports are already exposed.

???example "gNMI access"
    ```{.bash .no-select}
    gnmic -a 172.18.1.10 -u admin -p 'NokiaSrl1!' --skip-verify -e json_ietf \
      get --path /system/information/version
    ```
    <div class="embed-result">
    ```
    [
      {
        "source": "172.18.1.10",
        "timestamp": 1695423561467118891,
        "time": "2023-09-23T01:59:21.467118891+03:00",
        "updates": [
          {
            "Path": "srl_nokia-system:system/srl_nokia-system-info:information/version",
            "values": {
              "srl_nokia-system:system/srl_nokia-system-info:information/version": "v23.7.1-163-gd408df6a0c"
            }
          }
        ]
      }
    ]
    ```
    </div>

### Pod Shell

Load Balancer makes it easy to get external access to the lab nodes, but don't panic if for whatever reason you can't install one.
It is still possible to access the nodes without LB, it will just be less convenient.

For example, to access `srl1` lab node in our k8s cluster we just need to figure out which pod runs this node.

Since all pods are named after the nodes they are running, we can find the right one by listing all pods in a namespace:

```bash
kubectl get pods -n clabernetes | grep -iv manager 
```

<div class="embed-result">
```
NAME                                   READY   STATUS    RESTARTS   AGE
srl02-srl1-646dbff599-c65gw            1/1     Running   0          8m12s
srl02-srl2-d654ffbcd-4l2q7             1/1     Running   0          8m12s
```
</div>

Looking at the pod named `srl02-srl1-56675cdbfd-7tbk2` we understand that it runs `srl1` node we specified in the topology. To get shell access to this node we can run:

```{.bash .no-select}
kubectl -n clabernetes exec -it srl02-srl1-646dbff599-c65gw -- ssh admin@srl1
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
  - endpoints: ["srl1:e1-1", "srl2:e1-1"]
```

How does clabernetes layout this link when the lab nodes srl1 and srl2 can be scheduled on different worker nodes? Well, clabernetes takes the original link definition as provided by a user and transforms it into a set of point-to-point VXLAN tunnels[^4] that stitch the nodes together.

Two nodes appear to be connected to each other as if they were connected with a veth pair. We can check that LLDP neighbors are discovered on either other side of the link:

```{.bash .no-select}
kubectl -n clabernetes exec -it srl02-srl1-56675cdbfd-7tbk2 -- \
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
  +--------------+-------------------+----------------------+---------------------+------------------------+----------------------+---------------+
  |     Name     |     Neighbor      | Neighbor System Name | Neighbor Chassis ID | Neighbor First Message | Neighbor Last Update | Neighbor Port |
  +==============+===================+======================+=====================+========================+======================+===============+
  | ethernet-1/1 | 1A:48:00:FF:00:00 | srl2                 | 1A:48:00:FF:00:00   | 2 hours ago            | 2 seconds ago        | ethernet-1/1  |
  +--------------+-------------------+----------------------+---------------------+------------------------+----------------------+---------------+
```
</div>

We can also make sure that our startup-configuration that was provided in [external files](https://github.com/srl-labs/containerlab/tree/main/lab-examples/srl02) in original topology is applied in good order and we can perform ping between two nodes:

```text
--{ running }--[  ]--
A:srl1# ping 192.168.0.1 network-instance default -c 2 
Using network instance default
PING 192.168.0.1 (192.168.0.1) 56(84) bytes of data.
64 bytes from 192.168.0.1: icmp_seq=1 ttl=64 time=74.8 ms
64 bytes from 192.168.0.1: icmp_seq=2 ttl=64 time=8.82 ms

--- 192.168.0.1 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1002ms
rtt min/avg/max/mdev = 8.823/41.798/74.773/32.975 ms
```

???warning "VXLAN and MTU"
    VXLAN tunnels are susceptible to MTU issues. Check the MTU value for `vx-*` link in your pod to see what value has been set by the kernel and adjust your node's link/IP MTU accordingly.

    ```bash
    root@clab-srl02-srl1-55477468c4-vprj4:/clabernetes# ip l | grep vx
    9: vx-srl1-e1-1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    ```
    In our kind cluster that has a single network attached, the VXLAN tunnel is routed through the management network interface of the pod. It is possible to configure kind nodes to have more than one network and therefore have a dedicated network for the VXLAN tunnels with a higher MTU value.

## VM-based nodes?

In this quickstart we used native containerized Network OS - SR Linux - as it is lightweight and publicly available. But what if you want to use a VM-based Network OS like Nokia SR OS, Cisco IOS-XRv or Juniper vMX? Can you do that with clabernetes?

Short answer is yes. Clabernetes should be able to run VM-based nodes as well, but your cluster nodes must support nested virtualization, same as you would need to run VM-based nodes in containerlab.

Also you need to ensure that your VM-based container image is accessible to your cluster nodes, either via a public registry or a private one.

When these considerations are taken care of, you can use the same topology file as you would use with containerlab. The only difference is that you need to specify the image in the topology file as a fully qualified image name, including the registry name.

[^1]: In general there are no requirements for clabernetes from a kubernetes cluster perspective, however, many device types may have requirements for nested virtualization or specific CPU flags that your nodes would need to support in order to run the device.
[^2]: They may run on the same node, this is up to the kubernetes scheduler whose job it is to schedule pods on the nodes it deems most appropriate.
[^3]: Default exposed ports can be overwritten by a user via Containerlab CR.
[^4]: Using containerlab's [vxlan tunneling workflow](../../manual/multi-node.md#vxlan-tunneling) to create tunnels.
[^5]: The namespace name is derived from the name of the lab in the `.clab.yml` file.
