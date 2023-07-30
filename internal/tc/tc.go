package tc

import (
	"fmt"
	"net"
	"time"

	"github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

// NewTC returns a new tc client opened for a given network namespace.
// Must be closed after use.
func NewTC(ns int) (*tc.Tc, error) {
	tcnl, err := tc.Open(&tc.Config{
		NetNS: ns,
	})
	if err != nil {
		return nil, err
	}

	return tcnl, nil
}

// SetImpairments sets the impairments on the given interface of a node.
func SetImpairments(tcnl *tc.Tc, nodeName string, link *net.Interface, delay, jitter time.Duration, loss float64, rate uint64) (*tc.Object, error) {
	err := tcnl.SetOption(netlink.ExtendedAcknowledge, true)
	if err != nil {
		return nil, fmt.Errorf("could not set option ExtendedAcknowledge: %v", err)
	}

	qdisc := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(link.Index),
			Handle:  core.BuildHandle(0x1, 0x0),
			Parent:  tc.HandleRoot,
			Info:    0,
		},
		Attribute: tc.Attribute{
			Kind: "netem",
			Netem: &tc.Netem{
				Qopt: tc.NetemQopt{
					Limit: 10000, // max number of packets netem can hold during delay
				},
			},
		},
	}

	err = setDelay(&qdisc, delay, jitter)
	if err != nil {
		return nil, err
	}

	// if loss is set, propagate to qdisc
	// if loss != 0 {
	// 	adjustments = append(adjustments, toEntry("loss", fmt.Sprintf("%.3f%%", loss)))
	// 	qdisc.Attribute.Netem.Qopt = tc.NetemQopt{
	// 		Loss: uint32(math.Round(math.MaxUint32 * (loss / float64(100)))),
	// 	}
	// }

	// is rate is set propagate to qdisc
	// if rate != 0 {
	// 	adjustments = append(adjustments, toEntry("rate", fmt.Sprintf("%d kbit/s", rate)))
	// 	byteRate := rate / 8
	// 	qdisc.Attribute.Netem.Rate64 = &byteRate
	// }

	// log.Infof("Adjusting qdisc for Node: %q, Interface: %q - Settings: [ %s ]", nodeName,
	// 	link.Name, strings.Join(impairments, ", "))
	// replace the tc qdisc
	err = tcnl.Qdisc().Replace(&qdisc)
	if err != nil {
		return nil, err
	}

	// get qdisc of an interface after we set it
	qdiscs, err := tcnl.Qdisc().Get()
	if err != nil {
		return nil, fmt.Errorf("could not get all qdiscs: %v\n", err)
	}

	for _, qdisc := range qdiscs {
		if qdisc.Ifindex == uint32(link.Index) {
			return &qdisc, nil
		}
	}

	return nil, fmt.Errorf("could not find qdisc for interface %q", link.Name)
}

// setDelay sets delay and jitter to the qdisc.
func setDelay(qdisc *tc.Object, delay, jitter time.Duration) error {

	delayTcTime, err := core.Duration2TcTime(delay)
	if err != nil {
		return err
	}

	delayTicks := core.Time2Tick(delayTcTime)

	qdisc.Attribute.Netem.Qopt.Latency = delayTicks

	jitterTcTime, err := core.Duration2TcTime(jitter)
	if err != nil {
		return err
	}

	jitterTicks := core.Time2Tick(jitterTcTime)

	qdisc.Attribute.Netem.Qopt.Jitter = jitterTicks

	return err
}

// func PrintImpairments(nodeName string, nsFd int, link *net.Interface) error {

// }
