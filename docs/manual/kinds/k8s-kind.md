---
search:
  boost: 4
---
<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

# k8s-kind container
kind is a tool for running local Kubernetes clusters using Docker container “nodes”.
kind was primarily designed for testing Kubernetes itself, but may be used for local development or CI.

The containerlab k8s-kind node kind allow for kind clusters to be deployed via containerlab.


## Using k8s-kind containers
As with any other node, the linux container is a node of a specific kind, `k8s-kind` in this case.

```yaml
# a simple topo of two alpine containers connected with each other
name: k8s_kind_demo

topology:
  kinds:
    srl:
     type: ixrd3
     image: ghcr.io/nokia/srlinux
  nodes:
    srl01:
      kind: srl
    k01:
      kind: k8s-kind
      startup-config: k01-config.yaml
    k02:
      kind: k8s-kind

    # k01 -> resulting nodes due to startup-config assigned config `k01-config.yaml`
    k01-control-plane:
      kind: ext-container
    k01-worker:
      kind: ext-container
    k01-worker2:
      kind: ext-container

    # k02 -> default kind cluster with just single node 
    k02-control-plane:
      kind: ext-container
  links: 
    - endpoints: ["srl01:e1-1", "k01-control-plane:eth1"]
    - endpoints: ["srl01:e1-2", "k01-worker:eth1"]
    - endpoints: ["srl01:e1-3", "k01-worker2:eth1"]
    - endpoints: ["srl01:e1-4", "k02-control-plane:eth1"]

    - endpoints: ["srl01:e1-10", "k01-control-plane:eth2"]
    - endpoints: ["srl01:e1-11", "k02-control-plane:eth2"]

```

Contents of `k01-config.yaml`:
```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
- role: control-plane
- role: worker
- role: worker
```

The example topology will spin-up two k8s-kind cluster and an srlinux (srl) node.
Due to the kind cluster config provided to k01, kind will spin up 3 containers, one control-plane and two worker nodes. k02 which is not specifically provided a configuration file, will fallback to kind defaults, which is a single control-plane node.
Since kind will spin-up these different nodes, the k01 and k02 nodes are solely there to configure the kind cluster creation.

For the definition of the wiring / linking the ext-container is being used. As said, the k01 and k02 nodes are just placeholders to define the cluster, which results in a single or multiple control-plane and worker nodes. To allow for the definition of the connectivity then, the `ext-container` kind needs to be defined and can then be utilized in the links -> endpoints section. 


## Enabled node parameters
With k8s-kind nodes it is possible to use the following configuration parameters:

* [image](../nodes.md#image) - to define the kind container image to use for the kind cluster
* [startup-config](../nodes.md#startup-config) - to provide a kind cluster configuration [optional, _kind defaults apply otherwise_]