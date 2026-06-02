|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | A 5-stage CLOS topology based on Nokia SR Linux                      |
| **Components**                | [Nokia SR Linux][srl]                                                |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 4 <br/>:fontawesome-solid-memory: 8 GB |
| **Topology file**             | [clos02.clab.yml][topofile]                                          |
| **Name**                      | clos02                                                               |

## Description

This labs provides a lightweight folded 5-stage CLOS fabric with Super Spine level bridging two PODs.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:7,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/containerlab.drawio&quot;}"></div>

The topology is additionally equipped with the Linux containers connected to leaves to facilitate use cases which require access side emulation.

## Use cases

With this lightweight CLOS topology a user can exhibit the following scenarios:

* perform configuration tasks applied to the 5-stage CLOS fabric
* demonstrate fabric behavior leveraging the user-emulating linux containers attached to the leaves

## Configuration setup

To help you get faster to the provisioning of the services on this mini fabric we added an auto-configuration script to this example.

In order to make a fully deterministic lab setup we added another topology file called [setup.clos02.clab.yml][setup-topofile] where the management interfaces of each network node and clients are statically addressed with [`mgmt-ipv4/6` config option](../manual/nodes.md#mgmt-ipv4). Other than that, the topology files does not have any changes.

### Prerequisites

The configuration of the fabric elements is carried out with [`gnmic` client](https://gnmic.openconfig.net/install/), therefore it needs to be installed on the machine where you run the lab.

### Run instructions

First deploy this topology as per usual:

```
containerlab deploy -t setup.clos02.clab.yml
```

Once the lab is deployed, execute the [configuration script][setup-script]:

```
bash setup.sh
```

### Configuration schema

The [setup script][setup-script] will use the following IP addresses across the nodes of the lab:

The script will configure the following:

1. IP addresses for Management, System and Link interfaces of leaves and spines.
2. IP addresses for Clients eth0 (Management) and eth1 interfaces.
3. BGP, ISIS & OSPF protocols.

The following table outlines the addressing plan used in this lab:

| Source      | Interface      | Towards     | IPv4                | IPv6                      |
| ----------- | -------------- | ----------- | ------------------- | ------------------------- |
| leaf1       | mgmt0.0        | -           | `172.100.100.2/24`  | `3fff:172:100:100::2/64`  |
|             | system0.0      | -           | `30.0.0.1/32`       | `3000:30:0:0::1/128`      |
|             | ethernet-1/1.0 | spine1      | `10.0.0.0/31`       | `1000:10:0:0::0/127`      |
|             | ethernet-1/2.0 | spine2      | `10.0.0.2/31`       | `1000:10:0:0::2/127`      |
|             | ethernet-1/3.0 | client1     | `10.0.0.24/31`      | `1000:10:0:0::24/127`     |
| leaf2       | mgmt0.0        | -           | `172.100.100.3/24`  | `3fff:172:100:100::3/64`  |
|             | system0.0      | -           | `30.0.0.2/32`       | `3000:30:0:0::2/128`      |
|             | ethernet-1/1.0 | spine1      | `10.0.0.4/31`       | `1000:10:0:0::4/127`      |
|             | ethernet-1/2.0 | spine2      | `10.0.0.6/31`       | `1000:10:0:0::6/127`      |
|             | ethernet-1/3.0 | client2     | `10.0.0.26/31`      | `1000:10:0:0::26/127`     |
| leaf3       | mgmt0.0        | -           | `172.100.100.4/24`  | `3fff:172:100:100::4/64`  |
|             | system0.0      | -           | `30.0.0.3/32`       | `3000:30:0:0::3/128`      |
|             | ethernet-1/1.0 | spine3      | `10.0.0.12/31`      | `1000:10:0:0::12/127`     |
|             | ethernet-1/2.0 | spine4      | `10.0.0.14/31`      | `1000:10:0:0::14/127`     |
|             | ethernet-1/3.0 | client3     | `10.0.0.28/31`      | `1000:10:0:0::28/127`     |
| leaf4       | mgmt0.0        | -           | `172.100.100.5/24`  | `3fff:172:100:100::5/64`  |
|             | system0.0      | -           | `30.0.0.4/32`       | `3000:30:0:0::4/128`      |
|             | ethernet-1/1.0 | spine3      | `10.0.0.16/31`      | `1000:10:0:0::16/127`     |
|             | ethernet-1/2.0 | spine4      | `10.0.0.18/31`      | `1000:10:0:0::18/127`     |
|             | ethernet-1/3.0 | client4     | `10.0.0.30/31`      | `1000:10:0:0::30/127`     |
| spine1      | mgmt0.0        | -           | `172.100.100.6/24`  | `3fff:172:100:100::6/64`  |
|             | system0.0      | -           | `30.0.0.5/32`       | `3000:30:0:0::5/128`      |
|             | ethernet-1/1.0 | leaf1       | `10.0.0.1/31`       | `1000:10:0:0::1/127`      |
|             | ethernet-1/2.0 | leaf2       | `10.0.0.5/31`       | `1000:10:0:0::5/127`      |
|             | ethernet-1/3.0 | superspine1 | `10.0.0.8/31`       | `1000:10:0:0::8/127`      |
| spine2      | mgmt0.0        | -           | `172.100.100.7/24`  | `3fff:172:100:100::7/64`  |
|             | system0.0      | -           | `30.0.0.6/32`       | `3000:30:0:0::6/128`      |
|             | ethernet-1/1.0 | leaf1       | `10.0.0.3/31`       | `1000:10:0:0::3/127`      |
|             | ethernet-1/2.0 | leaf2       | `10.0.0.7/31`       | `1000:10:0:0::7/127`      |
|             | ethernet-1/3.0 | superspine2 | `10.0.0.10/31`      | `1000:10:0:0::10/127`     |
| spine3      | mgmt0.0        | -           | `172.100.100.8/24`  | `3fff:172:100:100::8/64`  |
|             | system0.0      | -           | `30.0.0.7/32`       | `3000:30:0:0::7/128`      |
|             | ethernet-1/1.0 | leaf3       | `10.0.0.13/31`      | `1000:10:0:0::13/127`     |
|             | ethernet-1/2.0 | leaf4       | `10.0.0.17/31`      | `1000:10:0:0::17/127`     |
|             | ethernet-1/3.0 | superspine1 | `10.0.0.20/31`      | `1000:10:0:0::20/127`     |
| spine4      | mgmt0.0        | -           | `172.100.100.9/24`  | `3fff:172:100:100::9/64`  |
|             | system0.0      | -           | `30.0.0.8/32`       | `3000:30:0:0::8/128`      |
|             | ethernet-1/1.0 | leaf3       | `10.0.0.15/31`      | `1000:10:0:0::15/127`     |
|             | ethernet-1/2.0 | leaf4       | `10.0.0.19/31`      | `1000:10:0:0::19/127`     |
|             | ethernet-1/3.0 | superspine2 | `10.0.0.22/31`      | `1000:10:0:0::22/127`     |
| superspine1 | mgmt0.0        | -           | `172.100.100.10/24` | `3fff:172:100:100::10/64` |
|             | system0.0      | -           | `30.0.0.9/32`       | `3000:30:0:0::9/128`      |
|             | ethernet-1/1.0 | spine1      | `10.0.0.9/31`       | `1000:10:0:0::9/127`      |
|             | ethernet-1/2.0 | spine3      | `10.0.0.21/31`      | `1000:10:0:0::21/127`     |
| superspine2 | mgmt0.0        | -           | `172.100.100.11/24` | `3fff:172:100:100::11/64` |
|             | system0.0      | -           | `30.0.0.10/32`      | `3000:30:0:0::10/128`     |
|             | ethernet-1/1.0 | spine2      | `10.0.0.11/31`      | `1000:10:0:0::11/127`     |
|             | ethernet-1/2.0 | spine4      | `10.0.0.23/31`      | `1000:10:0:0::23/127`     |
| client1     | eth0           | -           | `172.100.100.12/24` | `3fff:172:100:100::12/64` |
|             | eth1           | leaf1       | `10.0.0.25/31`      | `1000:10:0:0::25/127`     |
| client2     | eth0           | -           | `172.100.100.13/24` | `3fff:172:100:100::13/64` |
|             | eth1           | leaf2       | `10.0.0.27/31`      | `1000:10:0:0::27/127`     |
| client3     | eth0           | -           | `172.100.100.14/24` | `3fff:172:100:100::14/64` |
|             | eth1           | leaf3       | `10.0.0.29/31`      | `1000:10:0:0::29/127`     |
| client4     | eth0           | -           | `172.100.100.15/24` | `3fff:172:100:100::15/64` |
|             | eth1           | leaf4       | `10.0.0.31/31`      | `1000:10:0:0::31/127`     |

Configuration snippets that are used to provision the nodes are contained within the [`configs`](https://github.com/srl-labs/containerlab/tree/main/lab-examples/clos02/configs) subdirectory.

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/clos02/clos02.clab.yml
[setup-topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/clos02/setup.clos02.clab.yml
[setup-script]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/clos02/setup.sh

[^1]: Resource requirements are provisional. Consult with SR Linux Software Installation guide for additional information.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
