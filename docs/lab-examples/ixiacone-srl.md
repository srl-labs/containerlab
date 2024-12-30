|                               |                                                                                        |
| ----------------------------- | -------------------------------------------------------------------------------------- |
| **Description**               | Keysight Ixia-c-one node connected with Nokia SR Linux                                 |
| **Components**                | [Keysight Ixia-c-one][ixia-c-one], [Nokia SR Linux][srl]                               |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB                   |
| **Topology file**             | [ixiacone-srl.clab.yaml][topofile]                                                     |
| **Name**                      | ixiac01                                                                                |
| **Version information**[^2]   | `containerlab:0.46.2`, `ixia-c-one:1.19.0-5`, `srlinux:23.10.1`, `docker-ce:20.10.2`   |

## Description

This lab consists of a [Keysight Ixia-c-one](../manual/kinds/keysight_ixia-c-one.md) node with 2 ports connected to 2 ports on a Nokia SR Linux node via two point-to-point ethernet links. Both nodes are also connected with their management interfaces to the `containerlab` docker network.

Keysight Ixia-c-one is a single-container distribution of [Ixia-c][ixia-c], a software traffic generator and protocol emulator with [Open Traffic Generator (OTG) API][otg]. This example will demonstrate how test case designers can leverage Go SDK client [gosnappi][gosnappi] to create an OTG configuration and execute a test verifying IPv4 forwarding.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/ixiac&quot;}"></div>

## Deployment

Change into the lab directory:

```Shell
cd /etc/containerlab/lab-examples/ixiac01
```

Deploy the lab:

```Shell
sudo containerlab deploy
```

## Use cases

This lab allows users to validate an IPv4 traffic forwarding scenario between Keysight Ixia-c-one and Nokia SR Linux.

### IPv4 Traffic forwarding

<div class='mxgraph' style='max-width:100%;border:1px solid transparent;margin:0 auto; display:block;' data-mxgraph='{"page":1,"zoom":2,"highlight":"#0000ff","nav":true,"resize":true,"edit":"_blank","url":"https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/ixiac"}'></div>

This lab demonstrates a simple IPv4 traffic forwarding scenario where

- Keysight Ixia-c-one with two test ports `eth1` and `eth2` connected to Nokia SR Linux with ports `e1-1` and `e1-2` respectively.
- An OTG configuration applied to Ixia-c-one that emulates a router behind each test port: `r1` with IP `1.1.1.1/24` behind `eth1`, and `r2` with IP `2.2.2.1/24` behind `eth2`.
- The test is configured to send 100 IPv4 packets with a rate 10pps from `10.10.10.1` behind `r1` to `10.20.20.x`, where `x` is changed from 1 to 5.
- SR Linux interfaces are configured with `1.1.1.2/24` and `2.2.2.2/24` IPv4 addresses.
- SR Linux is configured to forward the traffic destined for `20.20.20.0/24` to `2.2.2.1` using a static route in the default network instance.

Logical IP topology of the lab is shown below:

<div class='mxgraph' style='max-width:100%;border:1px solid transparent;margin:0 auto; display:block;' data-mxgraph='{"page":2,"zoom":2,"highlight":"#0000ff","nav":true,"resize":true,"edit":"_blank","url":"https://raw.githubusercontent.com/srl-labs/containerlab/diagrams/ixiac"}'></div>

#### Configuration

During the lab deployment and test execution the following configuration is applied to the lab nodes to forward and receive traffic.

- **SR Linux**  
    SR Linux node comes up pre-configured with the commands listed in [srl.cfg][srlcfg] file which configure IPv4 addresses on both interfaces and install a static route to forward the traffic coming from ixia-c.

- **Keysight ixia-c-one**  
    IPv4 addresses for `ixia-c-one` node interfaces are configured via the OTG API as part of the [`ipv4_forwarding.go`][ipv4_forwarding] script.

#### Execution

The test case is written in Go language. To run it, [Go >= 1.21](https://go.dev/doc/install) needs to be installed first.

Once installed, run the test:

```Shell
go run ipv4_forwarding.go
```

Once 100 packets are sent, the test script checks that we received all the sent packets.

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

## Cleanup

To stop the lab, use:

```Shell
sudo containerlab destroy --cleanup
```

[ixia-c]: https://ixia-c.dev/
[ixia-c-one]: https://ixia-c.dev/deployments-containerlab/
[otg]: https://otg.dev/
[gosnappi]: https://github.com/open-traffic-generator/snappi/tree/main/gosnappi
[srl]: https://www.nokia.com/networks/products/service-router-linux-NOS/
[topofile]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ixiac01/ixiac01.clab.yml
[srlcfg]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ixiac01/srl.cfg
[ipv4_forwarding]: https://github.com/srl-labs/containerlab/blob/main/lab-examples/ixiac01/ipv4_forwarding.go

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://viewer.diagrams.net/js/viewer-static.min.js" async></script>
