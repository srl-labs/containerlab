/*
Test IPv4 Forwarding with
- Endpoints: OTG 1.1.1.1 -----> 1.1.1.2 DUT 2.2.2.2 ------> OTG 2.2.2.1
- Static Route on DUT: 20.20.20.0/24 -> 2.2.2.2
- TCP flow from OTG: 10.10.10.1 -> 20.20.20.1+

To run: go run ipv4_forwarding.go
*/

package main

import (
	"log"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
)

// hostname and interfaces of ixia-c-one node from containerlab topology.
const (
	otgHost  = "https://clab-ixiac01-ixia-c:8443"
	otgPort1 = "eth1"
	otgPort2 = "eth2"
)

var (
	r1Mac      = "02:00:00:00:01:aa"
	r2Mac      = "02:00:00:00:02:aa"
	r1Ip       = "1.1.1.1"
	r2Ip       = "2.2.2.1"
	r1IpPrefix = uint32(24)
	r2IpPrefix = uint32(24)
	r1IpGw     = "1.1.1.2"
	r2IpGw     = "2.2.2.2"
	pktCount   = 100
)

func main() {
	// create a new API handle to make API calls against otgHost
	api := gosnappi.NewApi()
	api.NewHttpTransport().SetLocation(otgHost).SetVerify(false)

	config := newConfig()

	// push traffic configuration to otgHost
	res, err := api.SetConfig(config)
	checkResponse(res, err)

	// start transmitting configured flows
	ts := gosnappi.NewControlState()
	ts.Traffic().FlowTransmit().SetState(gosnappi.StateTrafficFlowTransmitState.START)
	res, err = api.SetControlState(ts)
	checkResponse(res, err)

	// fetch flow metrics and wait for received frame count to be correct
	mr := gosnappi.NewMetricsRequest()
	mr.Flow()
	waitFor(
		func() bool {
			res, err := api.GetMetrics(mr)
			checkResponse(res, err)

			fm := res.FlowMetrics().Items()[0]
			return fm.Transmit() == gosnappi.FlowMetricTransmit.STOPPED && fm.FramesRx() == uint64(pktCount)
		},
		10*time.Second,
	)
}

func checkResponse(res interface{}, err error) {
	if err != nil {
		log.Fatal(err) // skipcq: RVV-A0003
	}
	switch v := res.(type) {
	case gosnappi.MetricsResponse:
		log.Printf("Metrics Response:\n%s\n", v)
	case gosnappi.Warning:
		for _, w := range v.Warnings() {
			log.Println("WARNING:", w)
		}
	default:
		log.Fatal("Unknown response type:", v) // skipcq: RVV-A0003
	}
}

func newConfig() gosnappi.Config {
	// create an empty traffic configuration
	config := gosnappi.NewConfig()
	// create traffic endpoints
	p1 := config.Ports().Add().SetName("p1").SetLocation(otgPort1)
	p2 := config.Ports().Add().SetName("p2").SetLocation(otgPort2)

	// create emulated devices (routers) – needed for ARP protocol to work
	r1 := config.Devices().Add().SetName("r1")
	r2 := config.Devices().Add().SetName("r2")

	// device ethernets
	r1Eth := r1.Ethernets().Add().SetName("r1Eth").SetMac(r1Mac)
	r2Eth := r2.Ethernets().Add().SetName("r2Eth").SetMac(r2Mac)

	// connections to test ports
	r1Eth.Connection().SetPortName(p1.Name())
	r2Eth.Connection().SetPortName(p2.Name())

	// device IP configuration
	r1Ip := r1Eth.Ipv4Addresses().Add().
		SetName("r1Ip").
		SetAddress(r1Ip).
		SetPrefix(r1IpPrefix).
		SetGateway(r1IpGw)

	r2Ip := r2Eth.Ipv4Addresses().Add().
		SetName("r2Ip").
		SetAddress(r2Ip).
		SetPrefix(r2IpPrefix).
		SetGateway(r2IpGw)

	// create a flow between r1 and r2
	f1 := config.Flows().Add().SetName("r1.v4.r2")
	f1.TxRx().Device().SetTxNames([]string{r1Ip.Name()})
	f1.TxRx().Device().SetRxNames([]string{r2Ip.Name()})

	// enable per flow metrics tracking
	f1.Metrics().SetEnable(true)
	// set size, count and transmit rate for all packets in the flow
	f1.Size().SetFixed(512)
	f1.Rate().SetPps(10)
	f1.Duration().FixedPackets().SetPackets(uint32(pktCount))

	// configure headers for all packets in the flow
	eth := f1.Packet().Add().Ethernet()
	ip := f1.Packet().Add().Ipv4()
	tcp := f1.Packet().Add().Tcp()

	eth.Src().SetValue(r1Mac)
	eth.Dst().Auto()

	ip.Src().SetValue("10.10.10.1")
	ip.Dst().Increment().SetStart("20.20.20.1").SetStep("0.0.0.1").SetCount(5)

	tcp.SrcPort().SetValue(3250)
	tcp.DstPort().Decrement().SetStart(8070).SetStep(2).SetCount(10)

	log.Printf("OTG configuration:\n%s\n", config)
	return config
}

func waitFor(fn func() bool, timeout time.Duration) {
	start := time.Now()
	for {
		if fn() {
			return
		}
		if time.Since(start) > timeout {
			log.Fatal("Timeout occurred !") // skipcq: RVV-A0003
		}

		time.Sleep(500 * time.Millisecond)
	}
}
