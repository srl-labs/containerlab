package utils

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func SetDelayJitterLoss(nodeName string, nsFd int, link *net.Interface, delay, jitter time.Duration, loss float64, rate uint64 /*in kbit*/) error {

	if link == nil {
		return fmt.Errorf("no link provided")
	}

	adjustments := []string{}

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
	if delay == 0 && loss == 0 && rate == 0 {
		log.Warn("non of the netem parameters (delay, jitter, loss, rate) was set")
		return nil
	}

	// open tc session
	tcnl, err := tc.Open(&tc.Config{
		NetNS: nsFd,
	})
	if err != nil {
		return err
	}

	defer func() {
		if err := tcnl.Close(); err != nil {
			log.Errorf("could not close rtnetlink socket: %v\n", err)
		}
	}()

	err = tcnl.SetOption(netlink.ExtendedAcknowledge, true)
	if err != nil {
		return fmt.Errorf("could not set option ExtendedAcknowledge: %v", err)
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
			Kind:  "netem",
			Netem: &tc.Netem{},
		},
	}

	// if loss is set, propagate to qdisc
	// if loss != 0 {
	// 	adjustments = append(adjustments, toEntry("loss", fmt.Sprintf("%.3f%%", loss)))
	// 	qdisc.Attribute.Netem.Qopt = tc.NetemQopt{
	// 		Loss: uint32(math.Round(math.MaxUint32 * (loss / float64(100)))),
	// 	}
	// }
	// if latency is set propagate to qdisc
	if delay != 0 {
		adjustments = append(adjustments, toEntry("delay", delay.String()))
		delay64 := delay.Milliseconds()
		qdisc.Attribute.Netem.Latency64 = &delay64
		// if jitter is set propagate to qdisc
		// if jitter != 0 {
		// 	adjustments = append(adjustments, toEntry("jitter", jitter.String()))
		// 	jit64 := (jitter * time.Millisecond).Milliseconds()
		// 	qdisc.Attribute.Netem.Jitter64 = &jit64
		// }
	}
	// is rate is set propagate to qdisc
	// if rate != 0 {
	// 	adjustments = append(adjustments, toEntry("rate", fmt.Sprintf("%d kbit/s", rate)))
	// 	byteRate := rate / 8
	// 	qdisc.Attribute.Netem.Rate64 = &byteRate
	// }

	log.Infof("Adjusting qdisc for Node: %q, Interface: %q - Settings: [ %s ]", nodeName, link.Name, strings.Join(adjustments, ", "))
	// replace the tc qdisc
	err = tcnl.Qdisc().Replace(&qdisc)
	if err != nil {
		return err
	}

	return nil
}

func toEntry(k, v string) string {
	return fmt.Sprintf("%s: %s", k, v)
}
