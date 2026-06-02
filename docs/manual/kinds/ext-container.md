---
search:
  boost: 4
kind_code_name: ext-container
kind_display_name: External Container
---

# -{{ kind_display_name }}-
-{{ kind_display_name }}- is identified with `-{{ kind_code_name }}-` kind in the [topology file](../topo-def-file.md).

Regular containerlab-managed nodes can be connected to externally managed containers. For instance, users may want to connect Network OS nodes launched by containerlab to some containers that are managed by other container orchestration tools to create advanced topologies.

This connectivity option is enabled by adding nodes of `-{{ kind_code_name }}-` kind to the topology.

## Using `-{{ kind_code_name }}-` kind

Containerlab doesn't create nodes of type `-{{ kind_code_name }}-`, it uses those nodes to let users create links to those externally managed containers from the nodes scheduled by containerlab.

The below topology demonstrates how the node named `srl` created by containerlab can be connected to the external container named `external-node1` that is created by some other tool.

```yaml
name: ext-cont

topology:
  nodes:
    srl:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux
    external-node1: #(1)!
      kind: -{{ kind_code_name }}-

  links:
    - endpoints: ["srl:e1-1", "external-node1:eth1"]
```

1. The name of the node of `-{{ kind_code_name }}-` kind should match the container name as displayed by the `docker ps` command.

By specifying the node `external-node1` as part of the containerlab topology, users can use this node name in the links section of the file and create links between containerlab-managed and externally-managed nodes.

## Interacting with External Container nodes

Even though -{{ kind_display_name }}- nodes are not scheduled by containerlab, it is possible to configure or interact with them using containerlab topology definition options.

For example, when deploying containerlab topology, users can execute commands in the external containers using [`exec`](../nodes.md#exec) configuration option:

```yaml
topology:
  nodes:
    srl:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux
    external-node1: #(1)!
      kind: -{{ kind_code_name }}-
      exec:
        - ip address add 192.168.0.1/24 dev eth1
```

1. `external-node1` is the name of a container launched outside of containerlab. In the case of a Docker runtime, this is a name displayed by `docker ps` command.
