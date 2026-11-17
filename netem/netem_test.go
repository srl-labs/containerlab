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

func TestRateKbit(t *testing.T) {
	rate64 := uint64(1_250_000_000)
	tests := map[string]struct {
		netem tc.Netem
		want  uint64
	}{
		"no rate":     {netem: tc.Netem{}, want: 0},
		"32-bit rate": {netem: tc.Netem{Rate: &tc.NetemRate{Rate: 1_250_000}}, want: 10_000},
		"64-bit rate": {
			netem: tc.Netem{Rate: &tc.NetemRate{Rate: math.MaxUint32}, Rate64: &rate64},
			want:  10_000_000,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := RateKbit(&test.netem); got != test.want {
				t.Fatalf("RateKbit() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestReplaceImpairmentsRejectsNegativeDurations(t *testing.T) {
	for _, d := range [][2]time.Duration{{-time.Millisecond, 0}, {time.Millisecond, -time.Millisecond}} {
		if err := ReplaceImpairments(nil, nil, d[0], d[1], 0, 0, 0); err == nil {
			t.Fatalf("delay %s and jitter %s must be rejected", d[0], d[1])
		}
	}
}
