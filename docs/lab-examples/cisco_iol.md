|                               |                                                                      |
| ----------------------------- | -------------------------------------------------------------------- |
| **Description**               | Two Cisco IOL routers connected via a Cisco IOL L2 switch            |
| **Components**                | [Cisco IOL][iol]                                                     |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 1 <br/>:fontawesome-solid-memory: 2 GB |
| **Topology file**             | [cisco_iol.clab.yml][topofile]                                       |
| **Name**                      | cisco_iol                                                            |
| **Version information**[^2]   | `cisco_iol:17.12.01`                                                 |

## Description

This lab consists of two Cisco IOL routers connected through a Cisco IOL L2 switch.

```
r1<---->sw<---->r2
```

The [IOL images](../manual/kinds/cisco_iol.md) are built with [vrnetlab](../manual/vrnetlab.md). The routers use the regular IOL image, the switch uses the L2 image and is marked with `type: l2` in the topology file.

## Configuration

On boot every node gets the containerlab startup configuration applied: the `Ethernet0/0` management interface is placed in its own management VRF and receives its IP addressing from docker, and SSH is enabled. Data-plane interfaces start at `Ethernet0/1` and come up unnumbered.

Log in with `admin`/`admin` and configure the router interfaces:

=== "r1"
    ```bash
    ssh admin@clab-cisco_iol-r1
    ```
    ```
    configure terminal
    interface Ethernet0/1
     ip address 192.168.1.1 255.255.255.0
    end
    ```
=== "r2"
    ```bash
    ssh admin@clab-cisco_iol-r2
    ```
    ```
    configure terminal
    interface Ethernet0/1
     ip address 192.168.1.2 255.255.255.0
    end
    ```

The switch forwards both router ports in VLAN 1 by default, so no configuration is needed on `sw` for this lab.

## Verification

Ping between the routers across the switch:

```
r1#ping 192.168.1.2
Type escape sequence to abort.
Sending 5, 100-byte ICMP Echos to 192.168.1.2, timeout is 2 seconds:
.!!!!
Success rate is 80 percent (4/5), round-trip min/avg/max = 1/1/2 ms
```

## Multinode Labs and Cisco IOL L2

Labs running on different hosts can be interconnected with containerlab by stitching an interface from each topology together with a VXLAN tunnel:

```
topology1.clab.yml <--> vxlan-stitch <--> topology2.clab.yml
```

Read more about this in the [`vxlan-stitch` link documentation](../manual/topo-def-file.md#vxlan-stitched) and the [multi-node labs](multinode.md) example.

When interconnecting labs this way, extra configuration is necessary to ensure crossfunctionality between topologies utilizing Cisco IOL L2 images. Due to system internals of how the NODE ID is generated and used for the images, an ID OFFSET needs to be configured to avoid duplicate/overlapping Bridge IDs and STP issues in the supertopology. 

The OFFSET can be controlled by utilizing the `CLAB_IOL_PID_OFFSET` environment variable:

```yaml
topology:
  kinds:
    cisco_iol:
      env:
        CLAB_IOL_PID_OFFSET: "64" # topology2 only; topology1 stays unset or vice versa
```

For example, setting the offset to `64` would skew the starting bridge ID for the topology from `aabb.cc00.0100` to `aabb.cc00.4100`.

Refer to the [multinode-site-a][site-a-topofile] and [multinode-site-b][site-b-topofile] examples for more information.

[iol]: ../manual/kinds/cisco_iol.md
[topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/cisco_iol/cisco_iol.clab.yml
[site-a-topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/cisco_iol/multinode-site-a.clab.yml
[site-b-topofile]: https://github.com/srl-labs/containerlab/tree/main/lab-examples/cisco_iol/multinode-site-b.clab.yml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
