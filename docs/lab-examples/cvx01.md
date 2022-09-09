|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | Cumulus Linux connected back-to-back with FRR                        |
| **Components**                | [Cumulus Linux][cvx]                                                 |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 1 GB |
| **Topology file**             | [topo.clab.yml][topofile]                                            |
| **Name**                      | cvx01                                                                |
| **Version information**[^2]   | `cvx:4.3.0` `docker-ce:19.03.13`                                     |

## Description

The lab consists of Cumulus Linux and FRR nodes connected back-to-back over a point-to-point ethernet link. Both nodes are also connected to the clab docker network over their management `eth0` interfaces.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/cvx.drawio&quot;}"></div>

## Configuration

Both nodes have been provided with a startup configuration and should come up with all their interfaces fully configured.

Once the lab is started, the nodes will be able to ping each other:

```
$ ssh lab-cvx01-sw1
Warning: Permanently added 'clab-cvx01-sw1,192.168.223.2' (ECDSA) to the list of known hosts.
root@clab-cvx01-sw1's password:
Linux 1c3f259f31872500 4.19.0-cl-1-amd64 #1 SMP Cumulus 4.19.149-1+cl4.3u1 (2021-01-28) x86_64
root@1c3f259f31872500:mgmt:~# ping 12.12.12.2 -c 1
vrf-wrapper.sh: switching to vrf "default"; use '--no-vrf-switch' to disable
PING 12.12.12.2 (12.12.12.2) 56(84) bytes of data.
64 bytes from 12.12.12.2: icmp_seq=1 ttl=64 time=0.400 ms

--- 12.12.12.2 ping statistics ---
1 packets transmitted, 1 received, 0% packet loss, time 0ms
rtt min/avg/max/mdev = 0.400/0.400/0.400/0.000 ms
```

## Use cases

* Demonstrate how a `cvx` node, running in its default `ignite` runtime, can connect to nodes running in other runtimes, e.g. `docker`
* Demonstrate how to inject startup configuration into a `cvx` node.
* Verify basic control plane and data plane operations

[cvx]: https://www.nvidia.com/en-gb/networking/ethernet-switching/cumulus-vx/
[topofile]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/cvx01/topo.clab.yml

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
