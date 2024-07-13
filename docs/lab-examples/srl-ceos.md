# Nokia SR Linux and Arista cEOS

|                               |                                                                             |
| ----------------------------- | --------------------------------------------------------------------------- |
| **Description**               | A Nokia SR Linux connected back-to-back with Arista cEOS                    |
| **Components**                | [Nokia SR Linux][srl], Arista cEOS                                          |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 4 GB        |
| **Topology file**             | [srlceos01.clab.yml][topofile]                                              |
| **Name**                      | srlceos01                                                                   |
| **Version information**[^2]   | `containerlab:0.56.0`, `srlinux:24.3.3`, `ceos:4.32.0F`, `docker-ce:26.0.0` |

## Description

A lab consists of an SR Linux node connected with Arista cEOS via a point-to-point ethernet link. Both nodes are also connected with their management interfaces to the `containerlab` docker network.

<div class='mxgraph' style='max-width:100%;border:1px solid transparent;margin:0 auto; display:block;' data-mxgraph='{"page":0,"zoom":1.5,"highlight":"#0000ff","nav":true,"resize":true,"edit":"_blank","url":"https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlceos01.drawio"}'></div>

## Deployment

The deployment process of this lab is explained in the [quickstart](../quickstart.md#deploying-a-lab).

## Use cases

This lab allows users to launch basic interoperability scenarios between Nokia SR Linux and Arista cEOS operating systems.

### BGP

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/srlceos01.drawio&quot;}"></div>

This lab demonstrates a simple iBGP peering scenario between Nokia SR Linux and Arista cEOS. Both nodes exchange NLRI with their loopback prefix making it reachable.

#### Configuration

Once the lab is deployed with containerlab, use the following configuration instructions to make interfaces configuration and enable BGP on both nodes.

/// tab | SR Linux
Get into SR Linux CLI with `ssh clab-srlceos01-srl` and start configuration. You can configure the node by typing in commands using the snippet below, or copy it entirely and paste it into the CLI.

```{.srl .code-scroll-lg}
# enter the candidate datastore
enter candidate

# configure physical interface
/ interface ethernet-1/1 {
    admin-state enable
    subinterface 0 {
        ipv4 {
            admin-state enable
            address 192.168.1.1/24 {
            }
        }
    }
}

# configure loopback interface
/ interface lo0 {
    subinterface 0 {
        ipv4 {
            admin-state enable
            address 10.10.10.1/32 {
            }
        }
    }
}

# configure routing policy to import/export routes via BGP
/ routing-policy {
    policy loopbacks-policy {
        statement 1 {
            match {
                protocol local
            }
            action {
                policy-result accept
            }
        }
    }
}

/ network-instance default {
    # add physical and logical interface to the network instance
    interface ethernet-1/1.0 {
    }
    interface lo0.0 {
    }
    protocols {
        bgp {
            autonomous-system 65001
            router-id 10.10.10.1
            afi-safi ipv4-unicast {
                admin-state enable
            }
            group ibgp {
                export-policy loopbacks-policy
                import-policy loopbacks-policy
            }
            neighbor 192.168.1.2 {
                admin-state enable
                peer-as 65001
                peer-group ibgp
            }
        }
    }
}

commit now
```

///
/// tab | cEOS
Get into cEOS CLI with `ssh clab-srlceos01-ceos`[^3] and start configuration

```bash
# enter configuration mode
enable
configure
ip routing

# configure loopback and data interfaces
interface Ethernet1
  no switchport
  ip address 192.168.1.2/24
exit
interface Loopback0
  ip address 10.10.10.2/32
exit

# configure BGP
router bgp 65001
  router-id 10.10.10.2
  neighbor 192.168.1.1 remote-as 65001
  network 10.10.10.2/32
exit
exit
```

///

#### Verification

Once BGP peering is established, the routes can be seen in GRT of both nodes:

/// tab | SR Linux

```srl
A:srl# show / network-instance default route-table ipv4-unicast prefix 10.*2/32
--------------------------------------------------------------------------------------------------------------------------------------------------
IPv4 unicast route table of network instance default
--------------------------------------------------------------------------------------------------------------------------------------------------
+----------------+------+-----------+--------------------+---------+---------+--------+-----------+----------+----------+----------+-------------+
|     Prefix     |  ID  |   Route   |    Route Owner     | Active  | Origin  | Metric |   Pref    | Next-hop | Next-hop |  Backup  |   Backup    |
|                |      |   Type    |                    |         | Network |        |           |  (Type)  | Interfac | Next-hop |  Next-hop   |
|                |      |           |                    |         | Instanc |        |           |          |    e     |  (Type)  |  Interface  |
|                |      |           |                    |         |    e    |        |           |          |          |          |             |
+================+======+===========+====================+=========+=========+========+===========+==========+==========+==========+=============+
| 10.10.10.2/32  | 0    | bgp       | bgp_mgr            | True    | default | 0      | 170       | 192.168. | ethernet |          |             |
|                |      |           |                    |         |         |        |           | 1.0/24 ( | -1/1.0   |          |             |
|                |      |           |                    |         |         |        |           | indirect |          |          |             |
|                |      |           |                    |         |         |        |           | /local)  |          |          |             |
+----------------+------+-----------+--------------------+---------+---------+--------+-----------+----------+----------+----------+-------------+
```

///
/// tab | cEOS

```bash
ceos>show ip route

VRF: default
Codes: C - connected, S - static, K - kernel,
    O - OSPF, IA - OSPF inter area, E1 - OSPF external type 1,
    E2 - OSPF external type 2, N1 - OSPF NSSA external type 1,
    N2 - OSPF NSSA external type2, B - BGP, B I - iBGP, B E - eBGP,
    R - RIP, I L1 - IS-IS level 1, I L2 - IS-IS level 2,
    O3 - OSPFv3, A B - BGP Aggregate, A O - OSPF Summary,
    NG - Nexthop Group Static Route, V - VXLAN Control Service,
    DH - DHCP client installed default route, M - Martian,
    DP - Dynamic Policy Route, L - VRF Leaked,
    RC - Route Cache Route

Gateway of last resort:
K        0.0.0.0/0 [40/0] via 172.20.20.1, Management0

B I      10.10.10.1/32 [200/0] via 192.168.1.1, Ethernet1
C        10.10.10.2/32 is directly connected, Loopback0
C        172.20.20.0/24 is directly connected, Management0
C        192.168.1.0/24 is directly connected, Ethernet1
```

///
Data plane confirms that routes have been programmed to FIB:

```
A:srl# ping 10.10.10.2 network-instance default
Using network instance default
PING 10.10.10.2 (10.10.10.2) 56(84) bytes of data.
64 bytes from 10.10.10.2: icmp_seq=1 ttl=64 time=3.47 ms
```

[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/srlceos01/srlceos01.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
[^3]: Credentials `admin:admin`
<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
