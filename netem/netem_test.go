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

func TestNetemTime64BitBoundary(t *testing.T) {
	tickInUSec := core.GetTickInUSec()
	if tickInUSec <= 0 {
		t.Fatalf("tick rate = %f, want a positive value", tickInUSec)
	}

	maxMicros := uint64(float64(math.MaxUint32) / tickInUSec)
	for float64(maxMicros)*tickInUSec > math.MaxUint32 {
		maxMicros--
	}
	atBoundary := time.Duration(maxMicros) * time.Microsecond
	aboveBoundary := atBoundary + time.Microsecond

	ticks, nanoseconds := netemTime(atBoundary)
	wantTicks := uint32(float64(maxMicros) * tickInUSec)
	if ticks != wantTicks || nanoseconds != nil {
		t.Fatalf(
			"netemTime(%s) = (%d, %v), want (%d, nil)",
			atBoundary,
			ticks,
			nanoseconds,
			wantTicks,
		)
	}

	ticks, nanoseconds = netemTime(aboveBoundary)
	if ticks != math.MaxUint32 || nanoseconds == nil ||
		*nanoseconds != aboveBoundary.Nanoseconds() {
		t.Fatalf(
			"netemTime(%s) = (%d, %v), want capped ticks and 64-bit nanoseconds",
			aboveBoundary,
			ticks,
			nanoseconds,
		)
	}
}

func TestSetRate64BitBoundary(t *testing.T) {
	const bytesPerKbit = 1000 / 8
	max32BitRate := uint64(math.MaxUint32) / bytesPerKbit

	for _, test := range []struct {
		name   string
		rate   uint64
		want64 bool
	}{
		{name: "zero", rate: 0},
		{name: "largest 32-bit byte rate", rate: max32BitRate},
		{name: "first 64-bit byte rate", rate: max32BitRate + 1, want64: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			qdisc := tc.Object{Attribute: tc.Attribute{Netem: &tc.Netem{}}}
			setRate(&qdisc, test.rate)

			got := qdisc.Attribute.Netem
			wantBytes := test.rate * bytesPerKbit
			want32 := uint32(min(wantBytes, uint64(math.MaxUint32)))
			if got.Rate == nil || got.Rate.Rate != want32 {
				t.Fatalf("32-bit rate = %+v, want %d", got.Rate, want32)
			}
			if test.want64 {
				if got.Rate64 == nil || *got.Rate64 != wantBytes {
					t.Fatalf("64-bit rate = %v, want %d", got.Rate64, wantBytes)
				}
			} else if got.Rate64 != nil {
				t.Fatalf("64-bit rate = %d, want nil", *got.Rate64)
			}
			if rate := RateKbit(got); rate != test.rate {
				t.Fatalf("RateKbit() = %d, want %d", rate, test.rate)
			}
		})
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
	for _, d := range [][2]time.Duration{{-time.Millisecond, 0}, {
		time.Millisecond, -time.Millisecond,
	}} {
		if err := ReplaceImpairments(nil, nil, d[0], d[1], 0, 0, 0); err == nil {
			t.Fatalf("delay %s and jitter %s must be rejected", d[0], d[1])
		}
	}
}
