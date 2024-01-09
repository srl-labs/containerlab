# Clabernetes Quickstart

The best way to understand how clabernetes works is to walk through a short example where we create a three-node k8s
cluster and deploy a lab there.

This quickstart uses [kind](https://kind.sigs.k8s.io/) to create a local kubernetes cluster and
then deploys clabernetes into. Once clabernetes is installed we deploy a small
[topology with two SR Linux nodes](../../lab-examples/two-srls.md) connected back to back together.

Once the lab is deployed, we explain how clabverter & clabernetes work in unison to to make the original topology files deployable onto the cluster
with tunnels stitching lab nodes together to form point to point connections between the nodes.  

Buckle up!

## Creating a cluster

Clabernetes goal is to allow users to run networking labs with containerlab's simplicity and ease of use, but with the scaling powers of kubernetes. To simulate the scaling aspect, we'll use [`kind`](https://kind.sigs.k8s.io/) to create a local multi-node kubernetes cluster. If you already have a k8s cluster, feel free to use it instead -- clabernetes can run in any kubernetes cluster[^1]!

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

A successful installation will result in a `clabernetes-manager` deployment of three pods running in
the cluster:

```{.bash .no-select}
kubectl get pods -o wide #(1)!
```

1. Note, that `clabernetes-manager` is installed as a 3-node deployment, and you can see that two pods might be in Init stay for a little while until the leader election is completed.

<div class="embed-result">
```
NAME                                   READY   STATUS     RESTARTS   AGE   IP           NODE          NOMINATED NODE   READINESS GATES
clabernetes-manager-7d8d9b4785-9xbnn   0/1     Init:0/1   0          27s   10.244.2.2   c9s-worker    <none>           <none>
clabernetes-manager-7d8d9b4785-hpc6h   0/1     Init:0/1   0          27s   10.244.1.3   c9s-worker2   <none>           <none>
clabernetes-manager-7d8d9b4785-z47dr   1/1     Running    0          27s   10.244.1.2   c9s-worker2   <none>           <none>
```
</div>

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

Clabernetes biggest advantage is that it uses the same topology file format as containerlab; as much as possible. Understandably though, the original [Containerlab's topology file](../../manual/topo-def-file.md) is not something you can deploy on k8s as is.  
We've created a converter tool called `clabverter` that takes containerlab topology file and converts it to kubernetes manifests. The manifests can then be deployed on a k8s cluster.

So how do we do that? Just enter the directory where original `clab.yml` file is located; for the [Two SR Linux nodes](../../lab-examples/two-srls.md) lab this would look like this:

```bash title="Entering the lab directory"
❯ cd lab-examples/srl02/ #(1)!

❯ ls
srl02.clab.yml  srl1.cfg  srl2.cfg
```

1. The path is relative to containerlab repository root.

And let `clabverter` do its job:

```{.bash .no-select title="Converting the containerlab topology to clabernetes manifests and applying it"}
clabverter --stdout | kubectl apply -f - #(1)!
```

1. `clabverter` converts the original containerlab topology to a set of k8s manifests and applies them to the cluster.

    We will cover what `clabverter` does in more details in the user manual some time later, but if you're curious, you can check the manifests it generates by running `clabverter --stdout > manifests.yml` and inspecting the `manifests.yml` file.

In the background, `clabverter` created `Containerlab` custom resource (CR) in the `clabernetes` namespace that defines our topology and also created a set of config maps for each startup config used in the lab.

## Verifying the deployment

Once clabverter is done, clabernetes controller casts its spell which is called reconciliation in k8s world. It takes the spec of the `Containerlab` CR (custom resource) and creates a set of deployments, config maps and services that are required to deploy the lab.

Let's run some verification commands to see what we have in our cluster so far.

Starting with listing `Containerlab` CRs in the `clabernetes` namespace:

``` {.bash .no-select}
kubectl get --namespace clabernetes Containerlab
```

<div class="embed-result">
```
NAME    AGE
srl02   3m27s
```
</div>

Looking in the Containerlab CR we can see that clabverter put original topology under the `spec.config` field. Clabernetes controller on its turn took the original topology and split it to sub-topologies that are outlined in the `status.configs` section of the resource:

``` {.bash .no-select}
kubectl get --namespace clabernetes Containerlabs srl02 -o yaml
```

<div class="embed-result" markdown>
=== "spec.config"
    ```yaml
    spec:
      config: |-
        # topology documentation: http://containerlab.dev/lab-examples/two-srls/
        name: srl02

        topology:
          nodes:
            srl1:
              kind: nokia_srlinux
              image: ghcr.io/nokia/srlinux
              startup-config: srl1.cfg
            srl2:
              kind: nokia_srlinux
              image: ghcr.io/nokia/srlinux
              startup-config: srl2.cfg

          links:
            - endpoints: ["srl1:e1-1", "srl2:e1-1"]
    ```
=== "status.configs"
    ```yaml
    # --snip--
    status:
      configs: |
        srl1:
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
                        startup-config: srl1.cfg
                        image: ghcr.io/nokia/srlinux
                links:
                    - endpoints:
                        - srl1:e1-1
                        - host:srl1-e1-1
            debug: false
        srl2:
            name: clabernetes-srl2
            prefix: ""
            topology:
                defaults:
                    ports:
                        - 60000:21/tcp
                        # here goes a list of exposed ports
                nodes:
                    srl2:
                        kind: nokia_srlinux
                        startup-config: srl2.cfg
                        image: ghcr.io/nokia/srlinux
                links:
                    - endpoints:
                        - srl2:e1-1
                        - host:srl2-e1-1
    ```
</div>

The sub-topologies are then deployed as deployments (which in their turn create pods) in the cluster, and containerlab is then run inside each pod deploying the topology as it would normally do on a single node:

``` {.bash .no-select title="Listing pods in srl02 namespace"}
kubectl get pods --namespace clabernetes -o wide
```

<div class="embed-result">
```
NAME                          READY   STATUS    RESTARTS   AGE    IP           NODE          NOMINATED NODE   READINESS GATES
clabernetes-manager-77bcc9484c-fn2gq   1/1     Running   0          11m   10.244.2.10   c9s-worker    <none>           <none>
clabernetes-manager-77bcc9484c-hs9c7   1/1     Running   0          11m   10.244.2.9    c9s-worker    <none>           <none>
clabernetes-manager-77bcc9484c-tvr42   1/1     Running   0          11m   10.244.1.10   c9s-worker2   <none>           <none>
srl02-srl1-646dbff599-c65gw            1/1     Running   0          43s   10.244.1.11   c9s-worker2   <none>           <none>
srl02-srl2-d654ffbcd-4l2q7             1/1     Running   0          43s   10.244.2.11   c9s-worker    <none>           <none>
```
</div>

Besides the `clabernetes-manager` pods, we see that two pods running (one per each lab node our original topology had) on different worker nodes[^2].
These pods run containerlab inside in a docker-in-docker mode and each node deploys a subset of the original topology. We can enter the pod and use containerlab CLI to verify the topology:

```{.bash .no-select}
kubectl exec -n clabernetes -it srl02-srl1-646dbff599-c65gw -- bash
```

And in the pod's shell we swim in the familiar containerlab waters:

```{.bash .no-select}
root@srl02-srl1-56675cdbfd-7tbk2:/clabernetes# clab inspect
```

<div class="embed-result">
```
INFO[0000] Parsing & checking topology file: topo.clab.yaml
+---+------+--------------+-----------------------+------+---------+----------------+----------------------+
| # | Name | Container ID |         Image         | Kind |  State  |  IPv4 Address  |     IPv6 Address     |
+---+------+--------------+-----------------------+------+---------+----------------+----------------------+
| 1 | srl1 | 80fae9ccf43b | ghcr.io/nokia/srlinux | srl  | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
+---+------+--------------+-----------------------+------+---------+----------------+----------------------+
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
