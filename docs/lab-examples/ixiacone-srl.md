|                               |                                                                                        |
| ----------------------------- | -------------------------------------------------------------------------------------- |
| **Description**               | A Keysight ixia-c-one node connected with Nokia SR Linux                               |
| **Components**                | [Keysight ixia-c-one][ixia-c], [Nokia SR Linux][srl]                                   |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB                   |
| **Topology file**             | [ixiacone-srl.clab.yaml][topofile]                                                     |
| **Name**                      | ixiac01                                                                                |
| **Version information**[^2]   | `containerlab:0.26.0`, `ixia-c-one:0.0.1-2738`, `srlinux:21.11.2`, `docker-ce:20.10.2` |

## Description
This lab consists of a [Keysight ixia-c-one](../manual/kinds/keysight_ixia-c-one.md) node with 2 ports connected to 2 ports on an Arista cEOS node via two point-to-point ethernet links. Both nodes are also connected with their management interfaces to the `containerlab` docker network.  

Keysight ixia-c-one is a single-container distribution of [ixia-c][ixia-c], which in turn is Keysight's reference implementation of [Open Traffic Generator API][otg]. This example will demonstrate how test case designers can leverage Go SDK client [gosnappi][gosnappi] to configure ixia-c traffic generator and execute a test verifying IPv4 forwarding.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/ixiac&quot;}"></div>

## Use cases
This lab allows users to validate an IPv4 traffic forwarding scenario between Keysight ixia-c-one and Nokia SR Linux.


### IPv4 Traffic forwarding
<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/ixiac&quot;}"></div>

This lab demonstrates a simple IPv4 traffic forwarding scenario where
- One Keysight ixia-c-one port acts as a transmit port (IP `1.1.1.1`) and the other as receive port (IP `2.2.2.2`)
- Nokia SR Linux is configured to forward the traffic destined for `20.20.20.0/24` to `2.2.2.2` using static route configuration in the default network instance

#### Configuration
Once the lab is deployed with containerlab, users need to configure the lab nodes to forward and receive traffic.


=== "SR Linux"
    SR Linux node comes up pre-configured with the commands listed in [srl.cfg][srlcfg] file which configure IPv4 addresses on both interfaces and install a static route to route the traffic coming from ixia-c.
=== "Keysight ixia-c-one"
    IPv4 addresses for data ports eth1/2 of ixia-c node are configured with `./ifcfg` scripts executed by containerlab on successful deployment[^3]. These commands are listed in the topology file under `exec` node property.


When a lab boots up, containerlab will also execute a command on SR Linux node to fetch MAC address of its `e1-1` interface which is connected to tx port of ixia-c-one. Write down this MAC address[^4] as it will serve as an argument in the test script we will run afterwards.

```bash
# partial output of `containerlab deploy` cmd that lists fetched MAC address
INFO[0019] Executed command 'bash -c "ip l show e1-1 | grep -o -E '([[:xdigit:]]{1,2}:){5}[[:xdigit:]]{1,2}' | head -1"' on clab-ixiac01-srl. stdout:
1a:b0:01:ff:00:01 
```

#### Execution
The test case is written in Go language hence [Go >= 1.17](https://go.dev/doc/install) needs to be installed first.

Once installed, change into the lab directory:
```
cd /etc/containerlab/lab-examples/ixiac01
```

Run the test with MAC address obtained in previous step:
```
go run ipv4_forwarding.go -dstMac="<MAC address>"
```

The test is configured to send 100 IPv4 packets with a rate 10pps from `10.10.10.1` to `10.20.20.x`, where `x` is changed from 1 to 5. Once 100 packets are sent, the test script checks that we received all the sent packets.

During the test run you will see flow metrics reported each second with the current flow data such as:

```
2022/04/12 16:28:10 Metrics Response:
choice: flow_metrics
flow_metrics:
- bytes_rx: "44032"
  bytes_tx: "0"
  frames_rx: "86"
  frames_rx_rate: 10
  frames_tx: "86"
  frames_tx_rate: 9
  name: p1.v4.p2
  transmit: started
```

#### Verification
The test that we ran above will continuously keep checking flow metrics to ensure packet count received on rx port of ixia-c-one are as expected.
If the condition is not met in 10 seconds, the test will timeout, hence indicating failure.

Upon success, last flow metrics output will indicate the latest status with `transmit` set to `stopped`.

```yaml
2022/04/12 16:28:11 Metrics Response:
choice: flow_metrics
flow_metrics:
- bytes_rx: "51200"
  bytes_tx: "0"
  frames_rx: "100"
  frames_rx_rate: 9
  frames_tx: "100"
  frames_tx_rate: 10
  name: p1.v4.p2
  transmit: stopped
```


[ixia-c]: https://github.com/open-traffic-generator/ixia-c
[otg]: https://redocly.github.io/redoc/?url=https://raw.githubusercontent.com/open-traffic-generator/models/master/artifacts/openapi.yaml
[gosnappi]: https://github.com/open-traffic-generator/snappi/tree/main/gosnappi
[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ixiac01/ixiac01.clab.yml
[srlcfg]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ixiac01/srl.cfg

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.  
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.
[^3]: Replace `add` with `del` to undo.
[^4]: The docker commands above shall not be required for upcoming releases of ixia-c-one with added ARP/ND capability.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>