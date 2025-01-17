---
search:
  boost: 4
---
<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

# Linux container

Labs deployed with containerlab are endlessly flexible, mostly because containerlab can spin up and wire regular containers as part of the lab topology.

Nowadays more and more workloads are packaged into containers, and containerlab users can nicely integrate them in their labs following a familiar docker' compose-like syntax. As long as the networking domain is considered, the most common use case for bare linux containers is to introduce "clients" or traffic generators which are connected to the network nodes or host telemetry/monitoring stacks.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/index.md&quot;}"></div>

But, of course, you are free to choose which container to add into your lab, there is not restriction to that!

## Using linux containers

As with any other node, the linux container is a node of a specific kind, `linux` in this case.

```yaml
# a simple topo of two alpine containers connected with each other
name: demo

topology:
  nodes:
    n1:
      kind: linux
      image: alpine:latest
    n2:
      kind: linux
      image: alpine:latest
  links:
    - endpoints: ["n1:eth1","n2:eth1"]
```

With a topology file like that, the nodes will start and both containers will have `eth1` link available.

Containerlab tries to deliver the same level of flexibility in container configuration as docker-compose has. With linux containers it is possible to use the following node configuration parameters:

* [image](../nodes.md#image) - to set an image source for the container
* [binds](../nodes.md#binds) - to mount files from the host to a container
* [ports](../nodes.md#ports) - to expose services running in the container to a host
* [env](../nodes.md#env) - to set environment variables
* [user](../nodes.md#user) - to set a user that will be used inside the container system
* [cmd](../nodes.md#cmd) - to provide a command that will be executed when the container is started

!!!note
    Nodes of `linux` kind will have a `on-failure` restart policy when run with docker runtime. This means that if container fails/exits with a non zero return code, docker will restart this container automatically.  
    When restarted, the container will loose all non-`eth0` interfaces. These can be re-added manually with [tools veth](../../cmd/tools/veth/create.md) command.
