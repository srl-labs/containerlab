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
	otgHost  = "https://clab-ixiac01-ixia-c"
	otgPort1 = "eth1"
	otgPort2 = "eth2"
)

var (
	dstMac   = "ff:ff:ff:ff:ff:ff"
	srcMac   = "00:00:00:00:00:aa"
	pktCount = 100
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
	f1.Rate().SetPps(10)
	f1.Duration().FixedPackets().SetPackets(int32(pktCount))

	// configure headers for all packets in the flow
	eth := f1.Packet().Add().Ethernet()
	ip := f1.Packet().Add().Ipv4()
	tcp := f1.Packet().Add().Tcp()

	eth.Src().SetValue(srcMac)
	eth.Dst().SetValue(dstMac)

	ip.Src().SetValue("10.10.10.1")
	ip.Dst().Increment().SetStart("10.20.20.1").SetStep("0.0.0.1").SetCount(5)

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
