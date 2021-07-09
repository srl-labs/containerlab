|                               |                                                                                          |
| ----------------------------- | ---------------------------------------------------------------------------------------- |
| **Description**               | Cumulus In The Cloud                                                                 |
| **Components**                | [Cumulus Linux][cvx]                                                                     |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 4 GB                     |
| **Topology file**             | [symm-mh.yml][topo-mh] <br/>[symm-mlag.yml][topo-mlag]                                                                     |
| **Name**                      | cvx04                                                                                    |
| **Version information**[^2]   | `cvx:4.3.0` `docker-ce:19.03.13`                                                         |

## Description
The lab is a multi-node topology that consists of two racks with two dual-homed servers connected with a leaf-spine network.


<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:2,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/cvx.drawio&quot;}"></div>

## Use cases
This is a "Cumulus In The Cloud" topology designed to demonstrate some of the advanced features of Cumulus Linux. It is based on the [original CITC demo environment](https://www.nvidia.com/en-gb/networking/network-simulation/) with the only exception being the reduced number of spine switches (2 instead of 4). The topology can be spun up fully provisioned with the following two configuration options:

1. [EVPN Multi-Homing](topo-mh) -- an EVPN-VXLAN environment with layer 2 extension, layer 3 VXLAN routing and VRFs for multi-tenancy that uses a multicast underlay for VXLAN packet replication and does not use MLAG or CLAG.
2. [EVPN Symmetric Mode](topo-mlag) -- an EVPN-VXLAN environment with layer 2 extension, layer 3 VXLAN routing, VRFs for multi-tenancy and MLAG/CLAG for server multi-homing.

## Instructions

Each configuration option is provided in its own configuration file -- [`symm-mh.yml`](topo-mh) or [`symm-mlag.yml`](topo-mlag). See [instructions](/lab-examples/lab-examples/#how-to-deploy-a-lab-from-the-lab-catalog) for how to deploy a topology. 

Once up, each node can be accessed via ssh using its hostname (automatically populated in your `/etc/hosts` file) and the default credentials `root/root`:

```
ssh root@clab-citc-leaf01
Warning: Permanently added 'clab-citc-leaf01,192.168.223.3' (ECDSA) to the list of known hosts.
root@clab-citc-leaf01's password:
Linux 94992c82719f1172 4.19.0-cl-1-amd64 #1 SMP Cumulus 4.19.149-1+cl4.3u1 (2021-01-28) x86_64
Last login: Fri Jul  9 13:35:48 2021 from 192.168.223.1
root@94992c82719f1172:mgmt:~# 
```

!!!note
    Due to the different boot order inside a container, BGPd may come up stuck waiting for IPv6 LLA of the peer. This issue only appears on the initial boot and can be fixed with the `vtysh -c 'clear ip bgp *` command.


[cvx]: https://www.nvidia.com/en-gb/networking/ethernet-switching/cumulus-vx/
[topo-mh]: https://github.com/srl-labs/containerlab/tree/master/lab-examples/cvx04/symm-mh.yml
[topo-mlag]: https://github.com/srl-labs/containerlab/tree/master/lab-examples/cvx03/symm-mlag.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>