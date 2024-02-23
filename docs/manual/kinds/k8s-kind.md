---
search:
  boost: 4
---

# Kubernetes in docker (kind) cluster

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

Since more and more applications (including network management systems and network functions) are being deployed in the k8s clusters, it is important to be able to test the network connectivity between the k8s workloads and the underlay network.

[Kind][kind-url] is a tool for running local Kubernetes clusters using Docker container “nodes”. By integrating kind clusters via a new kind `k8s-kind` with containerlab, it is possible to spin-up kind clusters as part of the containerlab topology.

This deployment model unlocks the possibility to integrate network underlay created by containerlab with the workloads running in the kind clusters in a single YAML file. The integration between kind clusters and containerlab topology makes it easy to deploy and interconnect k8s clusters and the underlay network.

## Using `k8s-kind`

Integration between Kind and Containerlab is a mix of two kinds:

1. `k8s-kind` - to manage the creation of the kind clusters
2. `ext-container` - to allow for the interconnection between the nodes of a kind cluster and the network nodes that are part of the same containerlab topology

The lab depicted below incorporates two kind clusters, one with a control plane and a worker node, and the other with an all-in-one node.

By defining the clusters with `k8s-kind` nodes we let containerlab manage the lifecycle (deployment/destroy) of the kind clusters. But this is not all. We can use the `ext-container` nodes to define actual kind cluster containers that run the control plane and worker nodes.

The name of the `ext-container` node is known upfront as it is computed as `<k8s-kind-node-name>-control-plane` for the control plane node and `<k8s-kind-node-name>-worker[worker-node-index]` for the worker nodes.

By defining the `ext-container` nodes we unlock the possibility to define the links between the kind cluster nodes and the network nodes that are part of the same containerlab topology.

```yaml
--8<-- "lab-examples/k8s_kind01/k8s_kind01.clab.yml"
```

This is exactly how you use the integration between containerlab and kind to create a topology that includes kind clusters and network nodes.

Once the lab pictured above is deployed, we can see the two clusters created:

///tab | get clusters

```bash
❯ kind get clusters
k01
k02
```

///

///tab | k01 nodes

```
❯ kind get nodes --name k01
k01-worker
k01-control-plane
```

///

///tab | k02 nodes

```
❯ kind get nodes --name k02
k02-control-plane
```

///

## Cluster config

It is possible to provide original kind cluster configuration via `startup-configuration` parameter of the `k8s-kind` node. Due to the kind cluster config provided to `k01` node above, kind will spin up 2 containers, one control-plane and one worker node. `k02` cluster that doesn't have a `startup-configuration` defined will spin up a single container with an all-in-one control-plane and a worker node.

Contents of `k01-config.yaml`:

```yaml
--8<-- "lab-examples/k8s_kind01/k01-config.yaml"
```

## Cluster interfaces

When containerlab orchestrates the kind clusters creation it relies on kind API to handle the actual deployment process. When kind creates a cluster it uses a docker network to connect the kind cluster nodes together.

In order to connect cluster nodes to the network underlay created by containerlab, we use `ext-container` kind of nodes per each control-plane and worker node of a cluster and connect them with their `eth1+` interfaces to the network nodes.

Since the `eth1+` interfaces come up unconfigured, we may configure them using the `exec` property and set the IP addresses.

Given the lab above, we configure `eth1` interface on all nodes. For example, we can check that worker node of cluster `k01` got its `eth1` inteface configured with the IP address:

```bash
❯ docker exec -it k01-worker ip a show eth1
12229: eth1@if12230: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 9500 qdisc noqueue state UP group default 
    link/ether aa:c1:ab:7e:22:6f brd ff:ff:ff:ff:ff:ff link-netnsid 1
    inet 192.168.11.1/24 scope global eth1
       valid_lft forever preferred_lft forever
```

## Node parameters

With `k8s-kind`` nodes it is possible to use the following configuration parameters:

- [image](../nodes.md#image) - to define the kind container image to use for the kind cluster
- [startup-config](../nodes.md#startup-config) - to provide a kind cluster configuration (optional, kind defaults apply otherwise)

### Extra parameters

In addition to the generic node parameters, `k8s-kind` can take following extra parameters from `extras` field.

```
topology:
  nodes:
    kind0:
      kind: k8s_kind
      extras:
        k8s_kind:
          deploy:
            # Corresponds to --wait option. Wait given duration until the cluster becomes ready.
            wait: 0s
```

## Known issues

### Duplication of nodes in the output

When you deploy a lab with `k8s-kind` nodes you may notice that the output of the `deploy` command contains more nodes than you have defined in the topology file. This is a known visual issue that is caused by the fact that `k8s-kind` nodes are merely a placeholder for a kind cluster configuration, and the actual nodes of the kind cluster are defined by the `ext-container` nodes.

### `inspect --all` command output

When you run `clab inspect --all` command you may notice that the output doesn't list the `k8s-kind` nodes nor the `ext-container` nodes.

For now, use `clab inspect -t <topology-file>` to see the full topology output.

[kind-url]: https://kind.sigs.k8s.io/
