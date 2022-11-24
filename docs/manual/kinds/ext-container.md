---
search:
  boost: 4
---
<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
# External Container
Containerlab can connect its nodes to externally created containers. This connectivity option is enabled with `ext-container` kind. Nodes of this kind are used as placeholder nodes for the external containers.



## Using ext-container kind
Containerlab doesn't create nodes of type `ext-container`, it will as part of the deployment phase check that a container with the name of the ext-container is in the running state. It will check every second for 15 min. If no container with the given name shows up in the running state, the deployment of the topology will stop and thereby fail.


```yaml
# topology documentation: http://containerlab.dev/lab-examples/externalContainer01/
name: externalContainer01

topology:
  kinds:
    srl:
      type: ixrd3
      image: ghcr.io/nokia/srlinux
  nodes:
    srl:
      kind: srl
    node1:
      kind: ext-container
    node2:
      kind: ext-container

  links:
    - endpoints: ["srl:e1-1", "node1:eth1"]
    - endpoints: ["srl:e1-2", "node2:eth1"]
```

This configuration will start an SRLinux node with the name of "srl" as a containerlab managed container.
Further it defines two additional nodes (node1 and node2) of type `ext-container`. These containers will not be created by containerlab.
It will try to find these nodes in the runtime and add the links as defined in the links section of the configuration.

## Enabled node parameters
With k8s-kind nodes it is possible to use the following configuration parameters:

* [exec](../nodes.md#exec) - to execute commands within the external container as part of the deployment