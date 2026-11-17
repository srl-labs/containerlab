package netem

import (
	"math"
	"testing"
	"time"

	"github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
)

func TestSetDelay(t *testing.T) {
	qdisc := tc.Object{Attribute: tc.Attribute{Netem: &tc.Netem{}}}

	setDelay(&qdisc, time.Hour, 50*time.Millisecond)

	netem := qdisc.Attribute.Netem
	if netem.Qopt.Latency != math.MaxUint32 || netem.Latency64 == nil ||
		*netem.Latency64 != time.Hour.Nanoseconds() {
		t.Fatalf("a delay of an hour must cap the ticks and set the nanoseconds, got %+v", netem)
	}
	if netem.Qopt.Jitter != core.Time2Tick(50000) || netem.Jitter64 != nil {
		t.Fatalf("a jitter of 50ms must only set the ticks, got %+v", netem)
	}
}
