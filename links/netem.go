package links

import (
	"fmt"
	"math"
	"time"
)

// maxNetemJitter is the highest jitter the kernel accepts without capping it.
const maxNetemJitter = math.MaxInt32 * time.Nanosecond

// maxNetemRate is the highest rate in kbit/s that can be converted to bytes/s.
const maxNetemRate = math.MaxUint64 / 1000

// NetemRaw is the link impairment of an endpoint as defined in the topology file.
// It applies to the packets the endpoint's interface sends.
type NetemRaw struct {
	Delay      string  `yaml:"delay,omitempty"`
	Jitter     string  `yaml:"jitter,omitempty"`
	Loss       float64 `yaml:"loss,omitempty"`
	Rate       uint64  `yaml:"rate,omitempty"`
	Corruption float64 `yaml:"corruption,omitempty"`
}

// Netem is the resolved link impairment of an endpoint.
type Netem struct {
	Delay      time.Duration
	Jitter     time.Duration
	Loss       float64
	Rate       uint64
	Corruption float64
}

// Resolve validates the impairment and returns nil if it sets nothing.
func (r *NetemRaw) Resolve() (*Netem, error) {
	if r == nil {
		return nil, nil
	}

	netem := &Netem{Loss: r.Loss, Rate: r.Rate, Corruption: r.Corruption}

	var err error
	if netem.Delay, err = parseNetemDuration("delay", r.Delay); err != nil {
		return nil, err
	}
	if netem.Jitter, err = parseNetemDuration("jitter", r.Jitter); err != nil {
		return nil, err
	}

	if netem.Jitter != 0 && netem.Delay == 0 {
		return nil, fmt.Errorf("netem jitter cannot be set without setting delay")
	}
	if netem.Jitter > maxNetemJitter {
		return nil, fmt.Errorf("netem jitter must be at most %s", maxNetemJitter)
	}
	if !(netem.Loss >= 0 && netem.Loss <= 100) {
		return nil, fmt.Errorf("netem loss must be in the range between 0 and 100")
	}
	if !(netem.Corruption >= 0 && netem.Corruption <= 100) {
		return nil, fmt.Errorf("netem corruption must be in the range between 0 and 100")
	}

	if netem.Rate > maxNetemRate {
		return nil, fmt.Errorf("netem rate must be at most %d kbit/s", uint64(maxNetemRate))
	}

	if *netem == (Netem{}) {
		return nil, nil
	}

	return netem, nil
}

func parseNetemDuration(name, value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("netem %s %q is not a duration, e.g. 100ms: %w", name, value, err)
	}
	if duration < 0 {
		return 0, fmt.Errorf("netem %s %q must not be negative", name, value)
	}

	return duration, nil
}
