|                               |                                                                                     |
| ----------------------------- | ------------------------------------------------------------------------------------|
| **Description**               | A Ixia-c-one node connected back-to-back with Arista cEOS                           |
| **Components**                | [Ixia-c-one][ixia-c], [Arista cEOS][ceos]                                           |
| **Resource requirements**[^1] | :fontawesome-solid-microchip: 2 <br/>:fontawesome-solid-memory: 2 GB                |
| **Topology file**             | [ixiaconeceos.clab.yaml][topofile]                                                  |
| **Name**                      | srlceos01                                                                           |
| **Version information**[^2]   | `containerlab:0.24.2`, `ixia-c-one:0.0.1-2738`, `ceos:4.26.0F`, `docker-ce:20.10.12`|

## Description
This lab consists of an Ixia-c-one node with 2 ports connected to 2 ports on an Arista cEOS node via two point-to-point ethernet links. Both nodes are also connected with their management interfaces to the `containerlab` docker network.  
`<TBD:Image might have to moved to diagram folder to be viewable correctly from the web page. It is currently kept in the ../images folder>`

<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:0,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;../images/ixia-c-one-ceos.drawio&quot;}"></div>

## Use cases
This lab allows users to test an IPv4 traffic forwarding scenario between Ixia-c-one and Arista cEOS.


### Layer 3 Traffic forwarding.
<div class="mxgraph" style="max-width:100%;border:1px solid transparent;margin:0 auto; display:block;" data-mxgraph="{&quot;page&quot;:1,&quot;zoom&quot;:1.5,&quot;highlight&quot;:&quot;#0000ff&quot;,&quot;nav&quot;:true,&quot;check-visible-state&quot;:true,&quot;resize&quot;:true,&quot;url&quot;:&quot;../images/ixia-c-one-ceos.drawio&quot;}"></div>

This lab demonstrates a simple Layer 3 traffic forwarding scenario where the 2 Ixia-c-one ports act as the transmit and recieve ports and Arista cEOS is configured to forward the traffic using static route configuration.

#### Configuration
Once the lab is deployed with containerlab, use the following configuration instructions to make interfaces configuration on Arista cEOS and configure the ixia-c-one ports to forward and receive traffic from the Device Under Test.  


===Arista cEOS

Enter the Arista cEOS cli by running following command:
```bash
docker exec -it clab-ixia-c-ceos Cli
```

Now execute the following commands to configure the Arista cEOS to forward IPv4 data traffic:
```bash
config terminal
interface Ethernet1
   no switchport
   ip address 1.1.1.2/24
!
interface Ethernet2
   no switchport
   ip address 2.2.2.1/24
!
ip route 20.20.20.0/24 2.2.2.2
!
ip routing
!
```
=== "ixia-c-one"

Two workarounds are needed that will be removed in a future version of ixia-c-one once it supports ARP/ND:  
1: To set an IPv4 address on the clab data interface on the ixia-c-one Rx port ( so that is responds to ARP requests)
```bash
docker exec -it clab-ixia-c-ixia-c-one bash
bash-5.1# bash set ipv4 eth2 2.2.2.2 24

[Note: Use bash unset ipv4 eth2 2.2.2.2 24 if you want to remove the IP e.g. to change the IP]
```
2: To get the DUT MAC to set as the Dst MAC of data packets :  
(In an automation environment , it can also be programmatically fetched using ssh or gnmi)
```bash
docker exec -it clab-ixia-c-ceos Cli
show interfaces Ethernet1 | incl Hardware
```

ixia-c-one is configured by using Rest APIs. It can be configured using multiple language sdks.  
In this example, steps are provided to setup the test with gosnappi sdk.

Ensure go is installed on the system and confirm using `go version` that the version is at least `go1.17.3` . 
1. Set up a go module :
```bash
$ go mod init example/test
```

2. This test needs a gosnappi module , the version of which should match the ixia-c-one version being used.
The correct matching version can be found at https://github.com/open-traffic-generator/ixia-c/releases
```bash
go get github.com/open-traffic-generator/snappi/gosnappi@v0.7.18
```

3. Create any go file with suffix in the name as 'test', example l3_forward_test.go 
Copy the contents of the below go code into the file. 

4. Now , run the test using the command :
`go test -run=TestL3Traffic -dmac="<DUT MAC>" -v| tee out.log`  
Note that the DUT MAC should be in format `"aa:bb:cc:dd:ee:ff"` 
```bash
e.g. 
$ go test -run=TestL3Traffic -dmac="aa:c1:ab:fe:3a:c2" -v  | tee out.txt
```



Test contents:
```go
/* Test L3 Traffic
Topology:
IXIA (1.1.1.1/24) -----> (1.1.1.2)ARISTA(2.2.2.1) ------> IXIA (2.2.2.2/24)
Flows:
- tcp: 10.10.10.1 -> 20.20.20.1+
*/

package tests

import (
        "testing"
        "log"
        "fmt"
        "flag"
        "time"
        "os"
        "os/exec"
        "runtime"
        "strings"

        /* go get github.com/open-traffic-generator/snappi/gosnappi@v0.7.18 ,
           this version much match with the ixia-c-one version in the clab file i.e ixia-c-one:0.0.1-2738
           Matching gosnappi version @ https://github.com/open-traffic-generator/ixia-c/releases */
        "github.com/open-traffic-generator/snappi/gosnappi"
)

var dmac string
/* Set the MAC of Arista eth1 interface here or pass with with parameter -dmac="<MAC>" when running go test */
func init() {
        flag.StringVar(&dmac, "dmac", "00:00:00:00:00:00", "Connected DUT MAC")
}

const (
        otgHttpLocation  = "https://clab-ixia-c-ixia-c-one"  //Name of ixia-c-one node in the clab topo file
        otgPort1Location = "ixia-c-port-eth1:5555"           //ixia-c-port-<clab topo link name>:5555
        otgPort2Location = "ixia-c-port-eth2:5555"
        optsClearPrevious = false                             //true: Clear stats on every iteration
)

/* Run as: go test -run=TestL3Traffic -dmac="00:2f:04:02:34:34" -v where dmac is the MAC of connected DUT.
   To automate the test completely, DUT MAC can be fetched using ssh or gnmi libraries from the DUT.
   And DUT config can also be set using ssh or gnmi library */
func TestL3Traffic(t *testing.T) {

        log.Printf("Dest MAC to be used when sending data packets to Arista :[%s]", dmac)
        /* Connect to ixia-c-one controller */
        client, err := NewClient(otgHttpLocation)
        if err != nil {
                t.Fatal(err)
        }

        /* Create test config with one L3 traffic flow */
        config, expected := trafficConfigL3(client)

        /* Apply config on ixia-c-one ports */
        res,err := client.Api().SetConfig(config)
        if err != nil {
                t.Fatal(err)
        }
        LogWarnings(res.Warnings())

        /* Start traffic */
        res,err = client.Api().SetTransmitState(client.Api().NewTransmitState().
                                                         SetState(gosnappi.TransmitStateState.START))
        if err != nil {
                t.Fatal(err)
        }
        LogWarnings(res.Warnings())

        /* Check for 20s if traffic is successfully forwarded by DUT.
           Test will FAIL if condition is not achieved within that time */
        waitForOpts := WaitForOpts{"for data traffic to be forwarded", /* Wait Condition */
                                   1000 * time.Millisecond,            /* Check after every Interval  ms */
                                   20 * time.Second}                   /* Fail test if not true after Timeout s*/
        WaitFor(t, func() (bool, error) { return client.FlowMetricsOk(expected) }, &waitForOpts)

        /* Stop traffic */
        res,err = client.Api().SetTransmitState(client.Api().NewTransmitState().
                                                         SetState(gosnappi.TransmitStateState.STOP))
                if err != nil {
                t.Fatal(err)
        }
        LogWarnings(res.Warnings())
        /* Test is successful. Same setup can be used for next test e.g. in a batch run */
}

type ApiClient struct {
        api gosnappi.GosnappiApi
}

func trafficConfigL3(client *ApiClient) (gosnappi.Config, ExpectedState) {
        config := client.Api().NewConfig()

        port1 := config.Ports().Add().SetName("ixia-c-port1").SetLocation(otgPort1Location)
        port2 := config.Ports().Add().SetName("ixia-c-port2").SetLocation(otgPort2Location)

        // OTG traffic configuration
        f1 := config.Flows().Add().SetName("p1.tcp.v4.p2")
        f1.Metrics().SetEnable(true)
        f1.TxRx().Port().
                SetTxName(port1.Name()).
                SetRxName(port2.Name())
        f1.Size().SetFixed(512)
        f1.Rate().SetPps(500)
        f1.Duration().FixedPackets().SetPackets(1000)
        e1 := f1.Packet().Add().Ethernet()
        e1.Src().SetValue("00:00:00:00:00:AA")
        e1.Dst().SetValue(dmac) // Provided by user. Must match with connected DUT MAC for present topology.
        v4 := f1.Packet().Add().Ipv4()
        v4.Src().SetValue("10.10.10.1")
        v4.Dst().Increment().SetStart("20.20.20.1").SetStep("0.0.0.1").SetCount(5)
        tc := f1.Packet().Add().Tcp()
        tc.SrcPort().SetValue(3250)
        tc.DstPort().Decrement().SetStart(8070).SetStep(2).SetCount(10)

        expected := ExpectedState{  //helpers.ExpectedState
                Flow: map[string]ExpectedFlowMetrics{
                        f1.Name(): {FramesRx: 1000, FramesRxRate: 0},
                },
        }

        return config, expected
}


func NewClient(location string) (*ApiClient, error) {
        client := &ApiClient{
                api: gosnappi.NewApi(),
        }

        log.Printf("Creating gosnappi client for HTTP server %s ...\n", location)
        client.api.NewHttpTransport().
                SetVerify(false).
                SetLocation(location)
        return client, nil
}

func (client *ApiClient) Api() gosnappi.GosnappiApi {
        return client.api
}

type ExpectedFlowMetrics struct {
        FramesRx     int64
        FramesRxRate float32
}
type ExpectedState struct {
        Flow map[string]ExpectedFlowMetrics
}

func (client *ApiClient) FlowMetricsOk(expectedState ExpectedState) (bool, error) {
        dNames := []string{}
        for name := range expectedState.Flow {
                dNames = append(dNames, name)
        }

        req := client.Api().NewMetricsRequest()
        req.Flow().SetFlowNames(dNames)

        res, err := client.Api().GetMetrics(req)
        if err != nil {
                fmt.Errorf("could not GetMetrics: %v", err)
                return false, err
        }
        fMetrics := res.FlowMetrics()

        /* If ClearPrevious: true , clear the screen after every iteration*/
        PrintMetricsTable(&MetricsTableOpts{
                ClearPrevious: optsClearPrevious,
                FlowMetrics: fMetrics,
        })

        expected := true
        for _, f := range fMetrics.Items() {
                expectedMetrics := expectedState.Flow[f.Name()]
                if f.FramesRx() != expectedMetrics.FramesRx || f.FramesRxRate() != expectedMetrics.FramesRxRate {
                        expected = false
                }
        }

        return expected, nil
}



type WaitForOpts struct {
        Condition string
        Interval  time.Duration
        Timeout   time.Duration
}

/* Check for condition to be true every opts.Interval ms (default 500ms)
   and fail the test is condition is false even after opts.Timeout s (default 30s)*/
func WaitFor(t *testing.T, fn func() (bool, error), opts *WaitForOpts) error {
        if opts == nil {
                opts = &WaitForOpts{
                        Condition: "condition to be true",
                }
        }
        defer Timer(time.Now(), fmt.Sprintf("Waiting for %s", opts.Condition))

        if opts.Interval == 0 {
                opts.Interval = 500 * time.Millisecond
        }
        if opts.Timeout == 0 {
                opts.Timeout = 30 * time.Second
        }

        start := time.Now()
        log.Printf("Waiting for %s ...\n", opts.Condition)

        for {
                done, err := fn()
                if err != nil {
                        t.Fatal(fmt.Errorf("error waiting for %s: %v", opts.Condition, err))
                }
                if done {
                        log.Printf("Done waiting for %s\n", opts.Condition)
                        return nil
                }

                if time.Since(start) > opts.Timeout {
                        t.Fatal(fmt.Errorf("timeout occurred while waiting for %s", opts.Condition))
                }
                time.Sleep(opts.Interval)
        }
}

type MetricsTableOpts struct {
        ClearPrevious bool
        FlowMetrics   gosnappi.MetricsResponseFlowMetricIter
}

/* if optsClearPrevious is true (default false), clear the screen after every iteration */
func PrintMetricsTable(opts *MetricsTableOpts) {
        if opts == nil {
                return
        }
        out := "\n"

        if opts.FlowMetrics != nil {
                border := strings.Repeat("-", 25*4+5)
                out += "\nFlow Metrics\n" + border + "\n"
                out += fmt.Sprintf("%-25s%-25s%-25s%-25s%-25s\n", "Name","Frames Tx", "Frames Rx","FPS Tx", "FPS Rx")
                                for _, m := range opts.FlowMetrics.Items() {
                        if m != nil {
                                name := m.Name()
                                tx := m.FramesTx()
                                rx := m.FramesRx()
                                txRate := m.FramesTxRate()
                                rxRate := m.FramesRxRate()
                                out += fmt.Sprintf("%-25v%-25v%-25v%-25v%-25v\n", name, tx, rx, txRate, rxRate)
                        }
                }
                out += border + "\n\n"
        }

        if opts.ClearPrevious {
                ClearScreen()
        }
        log.Println(out)
}

func ClearScreen() {
        switch runtime.GOOS {
        case "darwin":
                fallthrough
        case "linux":
                cmd := exec.Command("clear")
                cmd.Stdout = os.Stdout
                cmd.Run()
        case "windows":
                cmd := exec.Command("cmd", "/c", "cls")
                cmd.Stdout = os.Stdout
                cmd.Run()
        default:
                return
        }
}

func Timer(start time.Time, name string) {
        elapsed := time.Since(start)
        log.Printf("%s took %d ms", name, elapsed.Milliseconds())
}


func LogWarnings(warnings []string) {
        for _, w := range warnings {
                log.Printf("WARNING: %v", w)
        }
}

```

   
===

#### Verification
The success/failure of the test will be based on ixia-c-one traffic flow stats.
The test program will check whether all transmitted packets are recieved on eth2 of ixia-c-one.
If the packets are recieved, the test will indicate Pass status like below:
```bash
PASS
ok      example/test    3.099s
```
If the packets have not been transmitted within 20s ( timeout in the test) , it will indicate Fail status like below:
```bash
FAIL    example/test    20.146s
```
Note: If the clab is destroyed after the test, please use `docker volume prune` to remove 500MB+ of persistent volume storage
which is left behind after ixia-c-one docker container is removed.

<TBD: link for ixia-c-one below>
[ixia-c]: https://github.com/open-traffic-generator/ixia-c
[ceos]: https://www.arista.com/en/products/software-controlled-container-networking
[topofile]: https://github.com/srl-labs/containerlab/tree/master/lab-examples/ixiac/ixiaconeceos.clab.yaml

[^1]: Resource requirements are provisional. Consult with the installation guides for additional information.
[^2]: The lab has been validated using these versions of the required tools/components. Using versions other than stated might lead to a non-operational setup process.

<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/hellt/drawio-js@main/embed2.js" async></script>