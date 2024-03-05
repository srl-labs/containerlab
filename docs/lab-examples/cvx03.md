|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | Cumulus Linux in Docker runtime with leaf and spine topology         |
| **Components**                | [Cumulus Linux][cvx]                                                 |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 4 <br/>:fontawesome-solid-memory: 5 GB |
| **Topology file**             | [topo.clab.yml][topofile]                                            |
| **Name**                      | cvx03                                                                |
| **Version information**[^2]   | `cvx:5.3.0` `Docker version 25.0.3, build 4debf41`                   |

## Description

The lab consists of Cumulus Linux fabric composed of 2 borders, 2 spines and 2 leafs. The topology is additionally equipped with a Linux container connected to leaves to facilitate use cases which require access side emulation.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/cvx3.drawio&quot;}"></div>

## Configuration

The custom docker image need to be built locally before running the deployment
```
docker build \
--force-rm=true \
-t cx_ebtables:5.3.0 \
-f cx_ebtables.Dockerfile .
```

All nodes have been provided with a startup configuration and should come up with all their interfaces fully configured.

Once the lab is started, the nodes will be able to ping each other on their vlan interfaces:

```
# ping leaf interface
root@border-1:/# ping 10.162.0.14
PING 10.162.0.14 (10.162.0.14) 56(84) bytes of data.
64 bytes from 10.162.0.14: icmp_seq=1 ttl=64 time=0.262 ms
64 bytes from 10.162.0.14: icmp_seq=2 ttl=64 time=0.256 ms
^C
--- 10.162.0.14 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 34ms
```

Logs of the NVUE process are placed in `/root/nvue.log`.

## Use cases

* Demonstrate how a `cvx` can run with a EVPN VXLAN BGP fabric
* Demonstrate Cumulus Linux Leaf and spine
* Verify vlan trunking and access on connected host to a leaf

[cvx]: https://www.nvidia.com/en-gb/networking/ethernet-switching/cumulus-vx/
[topofile]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/cvx03/topo.clab.yml

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
