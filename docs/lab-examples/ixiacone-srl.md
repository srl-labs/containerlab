|                               |                                                                                        |
| ----------------------------- | -------------------------------------------------------------------------------------- |
| **Description**               | A Keysight ixia-c-one node connected with Nokia SR Linux                               |
| **Components**                | [Keysight ixia-c-one][ixia-c], [Nokia SR Linux][srl]                                   |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB                   |
| **Topology file**             | [ixiacone-srl.clab.yaml][topofile]                                                     |
| **Name**                      | ixiac01                                                                                |
| **Version information**[^2]   | `containerlab:0.46.2`, `ixia-c-one:0.1.0-84`, `srlinux:21.11.2`, `docker-ce:20.10.2`   |

## Description

This lab consists of a [Keysight ixia-c-one](../manual/kinds/keysight_ixia-c-one.md) node with 2 ports connected to 2 ports on a Nokia SR Linux node via two point-to-point ethernet links. Both nodes are also connected with their management interfaces to the `containerlab` docker network.

Keysight ixia-c-one is a single-container distribution of [ixia-c][ixia-c], which in turn is Keysight's reference implementation of [Open Traffic Generator API][otg]. This example will demonstrate how test case designers can leverage Go SDK client [gosnappi][gosnappi] to configure ixia-c traffic generator and execute a test verifying IPv4 forwarding.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/ixiac&quot;}"></div>

## Use cases

This lab allows users to validate an IPv4 traffic forwarding scenario between Keysight ixia-c-one and Nokia SR Linux.

### IPv4 Traffic forwarding

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/ixiac&quot;}"></div>

This lab demonstrates a simple IPv4 traffic forwarding scenario where

- One Keysight ixia-c-one port acts as a transmit port (IP `1.1.1.1`) and the other as receive port (IP `2.2.2.1`)
- Nokia SR Linux is configured to forward the traffic destined for `20.20.20.0/24` to `2.2.2.1` using static route configuration in the default network instance

#### Configuration

During the lab deployment and test execution the following configuration is applied to the lab nodes to forward and receive traffic.

=== "SR Linux"
    SR Linux node comes up pre-configured with the commands listed in [srl.cfg][srlcfg] file which configure IPv4 addresses on both interfaces and install a static route to forward the traffic coming from ixia-c.
=== "Keysight ixia-c-one"
    IPv4 addresses for `ixia-c-one` node interfaces are configured via the OTG API as part of the [`ipv4_forwarding.go`](ipv4_forwarding) script.

#### Execution

The test case is written in Go language. To run it, [Go >= 1.21](https://go.dev/doc/install) needs to be installed first.

Once installed, change into the lab directory:

```Shell
cd /etc/containerlab/lab-examples/ixiac01
```

Deploy the lab:

```Shell
sudo containerlab deploy
```

Run the test:

```Shell
go run ipv4_forwarding.go
```

The test is configured to send 100 IPv4 packets with a rate 10pps from `10.10.10.1` to `10.20.20.x`, where `x` is changed from 1 to 5. Once 100 packets are sent, the test script checks that we received all the sent packets.

During the test run you will see flow metrics reported each second with the current flow data such as:

```yaml
2023/12/18 11:14:12 Metrics Response:
choice: flow_metrics
flow_metrics:
- bytes_rx: "44032"
  bytes_tx: "44032"
  frames_rx: "86"
  frames_rx_rate: 9
  frames_tx: "86"
  frames_tx_rate: 9
  name: r1.v4.r2
  transmit: started
```

#### Verification

The test that we ran above will continuously keep checking flow metrics to ensure packet count received on rx port of ixia-c-one are as expected.
If the condition is not met in 10 seconds, the test will timeout, hence indicating failure.

Upon success, last flow metrics output will indicate the latest status with `transmit` set to `stopped`.

```yaml
2023/12/18 11:14:13 Metrics Response:
choice: flow_metrics
flow_metrics:
- bytes_rx: "51200"
  bytes_tx: "51200"
  frames_rx: "100"
  frames_rx_rate: 9
  frames_tx: "100"
  frames_tx_rate: 10
  name: r1.v4.r2
  transmit: stopped
```

#### Cleanup

To stop the lab, use:

```Shell
sudo containerlab destroy --cleanup
```


[ixia-c]: https://github.com/open-traffic-generator/ixia-c
[otg]: https://redocly.github.io/redoc/?url=https://raw.githubusercontent.com/open-traffic-generator/models/master/artifacts/openapi.yaml
[gosnappi]: https://github.com/open-traffic-generator/snappi/tree/main/gosnappi
[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ixiac01/ixiac01.clab.yml
[srlcfg]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ixiac01/srl.cfg
[ipv4_forwarding]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ixiac01/ipv4_forwarding.go

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
