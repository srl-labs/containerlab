|                               |                                                                                     |
| ----------------------------- | ------------------------------------------------------------------------------------|
| **Description**               | A Keysight ixia-c-one node connected back-to-back with Arista cEOS                  |
| **Components**                | [Keysight ixia-c-one][ixia-c], [Arista cEOS][ceos]                     |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB                |
| **Topology file**             | [ixiacone-ceos.clab.yaml][topofile]                                                  |
| **Name**                      | ixiacone-ceos                                                                           |
| **Version information**[^2]   | `containerlab:0.24.2`, `ixia-c-one:0.0.1-2738`, `ceos:4.26.1F`, `docker-ce:20.10.12`|

## Description
This lab consists of a Keysight ixia-c-one node with 2 ports connected to 2 ports on an Arista cEOS node via two point-to-point ethernet links. Both nodes are also connected with their management interfaces to the `containerlab` docker network.  

Keysight ixia-c-one is a single-container distribution of [ixia-c][ixia-c], which in turn is Keysight's reference implementation of [Open Traffic Generator API][otg].
We'll be using a Go client SDK [gosnappi][gosnappi] to configure ixia-c-one.

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;../images/ixia-c-one-ceos.drawio&quot;}"></div>

## Use cases
This lab allows users to:
- Validate an IPv4 traffic forwarding scenario between Keysight ixia-c-one and Arista cEOS.
- Validate a BGP forwarding plane between Keysight ixia-c-one and Arista cEOS.

### IPv4 Traffic forwarding.
<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;../images/ixia-c-one-ceos.drawio&quot;}"></div>

This lab demonstrates a simple IPv4 traffic forwarding scenario where
- One Keysight ixia-c-one port acts as a transmit port (IP 1.1.1.1) and the other as receive port (IP 2.2.2.2)
- Arista cEOS is configured to forward the traffic destined for 20.20.20.0/24 to 2.2.2.2 using static route configuration

#### Configuration
Once the lab is deployed with containerlab, use the following configuration instructions to configure interfaces on Arista cEOS and configure the Keysight ixia-c-one ports to forward and receive traffic from the Device Under Test.  

> If the topology is destroyed after the test, please use `docker volume prune` to remove 500MB+ of persistent volume storage which is left behind after ixia-c-one docker container is removed.

=== "Arista cEOS"
Get into cEOS CLI with `docker exec -it clab-ixia-c-ceos Cli` and start configuration
```bash
enable
configure terminal
interface Ethernet1
        no switchport
        ip address 1.1.1.2/24
exit
interface Ethernet2
        no switchport
        ip address 2.2.2.1/24
exit
ip route 20.20.20.0/24 2.2.2.2
ip routing
exit
exit
```
=== "Keysight ixia-c-one"
Setup to and run `ipv4_forwarding.go`
```bash
# configure IPv4 address on rx port of ixia-c-one (replace `add` with `del`
# to undo this)
docker exec -it clab-ixia-c-ixia-c-one bash -c "./ifcfg add eth2 2.2.2.2 24"
# note down MAC address of DUT interface connected to tx port of ixia-c-one
docker exec -it clab-ixia-c-ceos bash -c "ifconfig eth1 | grep ether"

# The docker commands above shall not be required for upcoming releases
# of ixia-c-one (which will have ARP/ND capability).

# install go from https://go.dev/doc/install since we'll need it to run the test
# and setup test environment
mkdir tests && cd tests
go mod init tests
# gosnappi version needs to be compatible to a given release of ixia-c-one and
# can be checked from https://github.com/open-traffic-generator/ixia-c/releases
go get github.com/open-traffic-generator/snappi/gosnappi@v0.7.18
# manually create a test file from the snippet or download it
curl -kLO https://raw.githubusercontent.com/open-traffic-generator/snappi-tests/main/scripts/ipv4_forwarding.go
# run the test with MAC address obtained in previous step
go run ipv4_forwarding.go -dstMac="<MAC address>"
```
=== "ipv4_forwarding.go"
```go
/*
Test IPv4 Forwarding with
- Endpoints: OTG 1.1.1.1 -----> 1.1.1.2 DUT 2.2.2.1 ------> OTG 2.2.2.2
- Static Route on DUT: 20.20.20.0/24 -> 2.2.2.2
- TCP flow from OTG: 10.10.10.1 -> 20.20.20.1+

To run: go run ipv4_forwarding.go -dstMac=<MAC of 1.1.1.2>
*/

package main

import (
        "flag"
        "log"
        "time"

        "github.com/open-traffic-generator/snappi/gosnappi"
)

// hostname and interfaces of ixia-c-one node from containerlab topology
const (
        otgHost  = "https://clab-ixia-c-ixia-c-one"
        otgPort1 = "eth1"
        otgPort2 = "eth2"
)

var (
        dstMac   = "ff:ff:ff:ff:ff:ff"
        srcMac   = "00:00:00:00:00:aa"
        pktCount = 1000
)

func main() {
        // replace value of dstMac with actual MAC of DUT interface connected to otgPort1
        flag.StringVar(&dstMac, "dstMac", dstMac, "Destination MAC address to be used for all packets")
        flag.Parse()

        api, config := newConfig()

        // push traffic configuration to otgHost
        res, err := api.SetConfig(config)
        checkResponse(res, err)

        // start transmitting configured flows
        ts := api.NewTransmitState().SetState(gosnappi.TransmitStateState.START)
        res, err = api.SetTransmitState(ts)
        checkResponse(res, err)

        // fetch flow metrics and wait for received frame count to be correct
        mr := api.NewMetricsRequest()
        mr.Flow()
        waitFor(
                func() bool {
                        res, err := api.GetMetrics(mr)
                        checkResponse(res, err)

                        fm := res.FlowMetrics().Items()[0]
                        return fm.Transmit() == gosnappi.FlowMetricTransmit.STOPPED && fm.FramesRx() == int64(pktCount)
                },
                10*time.Second,
        )
}

func checkResponse(res interface{}, err error) {
        if err != nil {
                log.Fatal(err)
        }
        switch v := res.(type) {
        case gosnappi.MetricsResponse:
                log.Printf("Metrics Response:\n%s\n", v)
        case gosnappi.ResponseWarning:
                for _, w := range v.Warnings() {
                        log.Println("WARNING:", w)
                }
        default:
                log.Fatal("Unknown response type:", v)
        }
}

func newConfig() (gosnappi.GosnappiApi, gosnappi.Config) {
        // create a new API handle to make API calls against otgHost
        api := gosnappi.NewApi()
        api.NewHttpTransport().SetLocation(otgHost).SetVerify(false)

        // create an empty traffic configuration
        config := api.NewConfig()
        // create traffic endpoints
        p1 := config.Ports().Add().SetName("p1").SetLocation(otgPort1)
        p2 := config.Ports().Add().SetName("p2").SetLocation(otgPort2)
        // create a flow and set the endpoints
        f1 := config.Flows().Add().SetName("p1.v4.p2")
        f1.TxRx().Port().SetTxName(p1.Name()).SetRxName(p2.Name())

        // enable per flow metrics tracking
        f1.Metrics().SetEnable(true)
        // set size, count and transmit rate for all packets in the flow
        f1.Size().SetFixed(512)
        f1.Rate().SetPps(500)
        f1.Duration().FixedPackets().SetPackets(int32(pktCount))

        // configure headers for all packets in the flow
        eth := f1.Packet().Add().Ethernet()
        ip := f1.Packet().Add().Ipv4()
        tcp := f1.Packet().Add().Tcp()

        eth.Src().SetValue(srcMac)
        eth.Dst().SetValue(dstMac)

        ip.Src().SetValue("10.10.10.1")
        ip.Dst().Increment().SetStart("20.20.20.1").SetStep("0.0.0.1").SetCount(5)

        tcp.SrcPort().SetValue(3250)
        tcp.DstPort().Decrement().SetStart(8070).SetStep(2).SetCount(10)

        log.Printf("OTG configuration:\n%s\n", config)
        return api, config
}

func waitFor(fn func() bool, timeout time.Duration) {
        start := time.Now()
        for {
                if fn() {
                        return
                }
                if time.Since(start) > timeout {
                        log.Fatal("Timeout occurred !")
                }

                time.Sleep(500 * time.Millisecond)
        }
}
```

#### Verification
The test that we ran above will continuously keep checking flow metrics to ensure packet count received on rx port of ixia-c-one are as expected.
If the condition is not met in 10 seconds, the test will timeout, hence indicating failure.  

Upon success, flow metrics should be as noted below.

```yaml
choice: flow_metrics
flow_metrics:
- bytes_rx: "512000"
  bytes_tx: "0"
  frames_rx: "1000"
  frames_rx_rate: 499
  frames_tx: "1000"
  frames_tx_rate: 500
  name: p1.v4.p2
  transmit: stopped
```

### BGPv4 Forwarding Plane

This section will soon be update with appropriate details.

> Support for protocols like BGP and IS-IS is not supported by free distribution of ixia-c-one. Please contact Keysight Support for more details.

[ixia-c]: https://github.com/open-traffic-generator/ixia-c  
[otg]: https://github.com/open-traffic-generator/models  
[gosnappi]: https://github.com/open-traffic-generator/snappi/tree/main/gosnappi  
[ceos]: https://www.arista.com/en/products/software-controlled-container-networking  
[topofile]: ../../lab-examples/ixiac/ixiacone-ceos.clab.yaml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.  
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>