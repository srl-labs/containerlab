package utils

import (
	"fmt"
	"math"
	"time"

	"github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func SetDelayJitterLoss(nsFd int, link netlink.Link, delay, jitter time.Duration, loss uint) error {

	if link == nil {
		return fmt.Errorf("no link provided")
	}

	// // check input is valid
	// loss betwenn 0 and 100
	if loss != 0 && loss > 100 {
		return fmt.Errorf("loss must be >= 0 and <= 100")
	}
	// jitter must not be set without delay
	if jitter != 0 && delay == 0 {
		return fmt.Errorf("cannot set jitter without delay")
	}
	// if delay and loss are nil, we have nothing to do
	if delay == 0 && loss == 0 {
		logrus.Warn("non of the netem parameters (delay, jitter, loss) was set")
		return nil
	}

	// open tc session
	tcnl, err := tc.Open(&tc.Config{
		NetNS: nsFd,
	})
	if err != nil {
		return err
	}

	qdisc := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(link.Attrs().Index),
			Handle:  core.BuildHandle(0xFFFF, 0x0000),
			Parent:  0xFFFFFFF1,
			Info:    0,
		},
		Attribute: tc.Attribute{
			Kind:  "netem",
			Netem: &tc.Netem{},
		},
	}

	// if loss is set, propagate to qdisc
	if loss != 0 {
		qdisc.Attribute.Netem.Qopt = tc.NetemQopt{
			Loss: uint32(math.Round(math.MaxUint32 * (float64(loss) / float64(100)))),
		}
	}
	// if latency is set propagate to qdisc
	if delay != 0 {
		lat64 := (delay * time.Millisecond).Milliseconds()
		qdisc.Attribute.Netem.Latency64 = &lat64
		// if jitter is set propagate to qdisc
		if jitter != 0 {
			jit64 := (jitter * time.Millisecond).Milliseconds()
			qdisc.Attribute.Netem.Jitter64 = &jit64
		}
	}

	// replace the tc qdisc
	err = tcnl.Qdisc().Replace(&qdisc)
	if err != nil {
		return err
	}

	return nil
}
