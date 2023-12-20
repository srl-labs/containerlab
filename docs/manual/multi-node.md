# Multi-node labs

Containerlab is a perfect tool of choice when all the lab components/nodes fit into one VM or bare metal server. Unfortunately, sometimes it is hard to satisfy this requirement and fit a big and sophisticated lab on a single host.

Although containerlab is not (yet) capable of deploying topologies over a number of container hosts, we have embedded some capabilities that can help you to workaround the single-host resources constraint.

## Exposing services

Sometimes all that is needed is to make certain services running inside the nodes launched with containerlab available to a system running outside of the container host. For example, you might have an already running telemetry stack somewhere in your lab and you want to use it with the routing systems deployed with containerlab.

In that case, the simple solution would be to expose the nodes' ports which are used to collect telemetry information. Take a look the following example where two nodes are defined in the topology file and get their gNMI port exposed to a host under a user-defined host-port.

```yaml
name: telemetry

topology:
  nodes:
    ceos:
      kind: ceos
      image: ceos:latest
      ports:
        # host port 57401 is mapped to port 57400 of ceos node
        - 57401:57400
    srl:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux
      ports:
        - 57402:57400
  links:
    - endpoints: ["ceos:eth1", "srl:e1-1"]
```

Once the container's ports/services are exposed to a host under host-port, the telemetry collector running outside of the container host system can reach each node gNMI service.

If container host has IP address of `$IP`, then telemetry collector can reach `ceos` telemetry service by `$IP:57401` address and `srl` gNMI service will be reachable via `$IP:57402`.

## Exposing management network

Exposing services on a per-port basis as shown above is a quick and easy way to make a certain service available via a host port, likely being the most common way of exposing services with containerlab. Unfortunately, not every use case can be covered with such approach.

Imagine if you want to integrate an NMS system running elsewhere with a lab you launched with containerlab. Typically you would need to expose the entire management network for an NMS to start managing the nodes with management protocols required.  
In this scenario you wouldn't get far with exposing services via host-ports, as NMS would expect to have IP connectivity with the node it is about to adopt for managing.

For integration tasks like this containerlab users can leverage static routing towards [containerlab management network](network.md#management-network). Consider the following diagram:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/multinode.drawio&quot;}"></div>
<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>

This solution requires to set up routing between the host which runs the NMS and the container host that has containerlab nodes inside. Since containers are always attached to a common management network, we can make this network reachable by installing, for example, a static route on the NMS host. This will provision the datapath between the NMS and the containerlab management network.

By default, containerlab management network is addressed with `172.20.20./0` IPv4 address, but this can be easily [changed](network.md#configuring-management-network) to accommodate for network environment.

## Bridging

Previous examples were aiming management network access, but what if we need to rather connect a network interfaces of a certain node with a system running outside of the container host? An example for such connectivity requirement could be a traffic generator connected to a containerized node port.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/multinode.drawio&quot;}"></div>

In this case we can leverage the bridge kind[^1] that containerlab offers to connect container' interface to a pre-created bridge and slice the network with VLANs to create a L2 connectivity between the ports:

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:2,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/multinode.drawio&quot;}"></div>

## VxLAN Tunneling

Sometimes VLAN bridging is not possible, for example when the other end of the virtual wire is reachable via routing, not bridging. We have developed a semi-automated solution for this case as well.

The idea is to create unicast VxLAN tunnels between the VMs hosting nodes requiring connectivity.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:10,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/multinode.drawio&quot;}"></div>

Refer to the [multinode](../lab-examples/multinode.md) lab that goes deep in details on how to create this tunneling and explains the technicalities of such dataplane.

[^1]: Both regular linux [bridge](kinds/bridge.md) and [ovs-bridge](kinds/ovs-bridge.md) kinds can be used, depending on the requirements.
