package tc

import (
	"fmt"
	"math"
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
func SetImpairments(tcnl *tc.Tc, nodeName string, link *net.Interface, delay, jitter time.Duration,
	loss float64, rate uint64, probability float64,
) (*tc.Object, error) {
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

	setLoss(&qdisc, loss)

	setRate(&qdisc, rate)

	// Always set the corruption field (even if probability is 0) to allow resetting.
	setCorruption(&qdisc, probability)

	err = tcnl.Qdisc().Replace(&qdisc)
	if err != nil {
		return nil, err
	}

	// get qdisc of an interface after we set it
	qdiscs, err := tcnl.Qdisc().Get()
	if err != nil {
		return nil, fmt.Errorf("could not get all qdiscs: %v", err)
	}

	for idx := range qdiscs {
		if qdiscs[idx].Ifindex == uint32(link.Index) {
			return &qdiscs[idx], nil
		}
	}

	return nil, fmt.Errorf("could not find qdisc for interface %q", link.Name)
}

// DeleteImpairments deletes the netem impairments from the given interface.
func DeleteImpairments(tcnl *tc.Tc, link *net.Interface) error {
	qdisc := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(link.Index),
			Handle:  core.BuildHandle(0x1, 0x0),
			Parent:  tc.HandleRoot,
			Info:    0,
		},
		Attribute: tc.Attribute{
			Kind:  "netem",
			Netem: &tc.Netem{},
		},
	}
	return tcnl.Qdisc().Delete(&qdisc)
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

// setLoss sets the loss to the qdisc.
func setLoss(qdisc *tc.Object, loss float64) {
	qdisc.Attribute.Netem.Qopt.Loss = uint32(math.Round(math.MaxUint32 * (loss / float64(100))))
}

// setRate sets the rate to the qdisc.
// The rate is provided in kbit.
func setRate(qdisc *tc.Object, rate uint64) {
	// convert to bytes
	byteRate := rate * 1000 / 8
	qdisc.Attribute.Netem.Rate = &tc.NetemRate{
		Rate: uint32(byteRate),
	}
}

// setCorruption sets the corruption probability and correlation.
func setCorruption(qdisc *tc.Object, probability float64) {
	qdisc.Netem.Corrupt = &tc.NetemCorrupt{
		Probability: uint32(math.Round(math.MaxUint32 * (probability / float64(100)))),
	}
}

// Impairments returns all link impairments of a node.
func Impairments(tcnl *tc.Tc) ([]tc.Object, error) {
	qdiscs, err := tcnl.Qdisc().Get()
	if err != nil {
		return nil, fmt.Errorf("could not get all qdiscs: %v", err)
	}

	return qdiscs, nil
}
