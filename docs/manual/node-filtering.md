# Node filtering

With the node filtering feature containerlab enables users to perform operations on a subset of nodes defined in the topology file. A typical use case is to deploy only a subset of nodes in a lab to save deployment time and host resources.

Consider the following topology file that defines 4 interconnected nodes:

```yaml
name: filter

topology:
  defaults:
    image: alpine:3
  nodes:
    node1:
    node2:
    node3:
    node4:

  links:
    - endpoints: ["node1:eth1", "node2:eth1"]
    - endpoints: ["node1:eth2", "node3:eth2"]
    - endpoints: ["node1:eth3", "node4:eth3"]
    - endpoints: ["node2:eth2", "node4:eth2"]
    - endpoints: ["node3:eth1", "node4:eth1"]
```

A graphical representation of the topology forms a ring with 4 nodes and 5 links between them:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/node-filter.drawio&quot;}"></div>

## Deploying a subset of nodes

By default, all nodes and their links are deployed when the `deploy` command is executed. But what if each node is a massive VM that consumes a lot of RAM and you want to launch just a few of them? This use case becomes even more relevant when you have a large lab with many nodes and isolated domains.

For the sake of this example, let's assume that we want to deploy only nodes `node1`, `node2`, and `node4`, representing a subring. To do that, we can use the [`--node-filter` flag](../cmd/deploy.md#node-filter) and provide a comma-separated list of nodes names to deploy:

```bash
clab deploy --node-filter node1,node2,node4
```

As a result of this command, only nodes `node1`, `node2`, and `node4` will be deployed, and the links between them will be created. The remaining nodes and links will be ignored.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/node-filter.drawio&quot;}"></div>

When filtering the nodes to `node1` and `node2` the topology becomes a linear chain:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:2,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/node-filter.drawio&quot;}"></div>

## Destroying a subset of nodes

The `destroy` command can also be scoped to a subset of nodes. The same [`--node-filter` flag](../cmd/destroy.md#node-filter) can be used to specify the nodes to destroy. For example, to destroy only nodes `node1` and `node2` from the previous example, we can run:

```bash
clab destroy --node-filter node1,node2
```

And only these two nodes will be destroyed (with all links connected to them), leaving the rest of the lab intact.

## Other commands

The following commands have support for `--node-filter` flag:

* `graph`
* `save`
* `config`

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
