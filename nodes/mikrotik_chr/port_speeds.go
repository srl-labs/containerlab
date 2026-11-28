package mikrotik_chr

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	clabnodes "github.com/srl-labs/containerlab/nodes"
)

const (
	// portSpeedsEnv hands the port speeds to the launcher as "<port>:<Mb/s>" pairs, e.g.
	// "2:1000 3:1000". The kind always sets it, so only validated speeds reach QEMU.
	portSpeedsEnv = "CHR_PORT_SPEEDS"
	// maxPortSpeed is the highest speed QEMU's virtio-net accepts (an int32, in Mb/s).
	maxPortSpeed = math.MaxInt32
)

// KindSpecificConfig is the mikrotik_chr kind-specific config, set as keys on the node
// definition.
type KindSpecificConfig struct {
	// PortSpeeds sets the speed that data ports report, for one port or an inclusive range.
	PortSpeeds []PortSpeed `yaml:"port-speeds,omitempty" json:"port-speeds,omitempty"`
}

// PortSpeed sets the speed in Mb/s that data ports report. Port is a RouterOS port number, 2 for
// ether2, or an inclusive range such as 2..25.
type PortSpeed struct {
	Port  string `yaml:"port"  json:"port"`
	Speed int    `yaml:"speed" json:"speed"`
}

// UnmarshalYAML checks for a fraction before yaml.v2 can truncate it to an int.
func (p *PortSpeed) UnmarshalYAML(unmarshal func(any) error) error {
	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	if speed, ok := raw["speed"].(float64); ok && speed != math.Trunc(speed) {
		return fmt.Errorf("speed must be a whole number of Mb/s, got %v", speed)
	}

	type plainPortSpeed PortSpeed
	return unmarshal((*plainPortSpeed)(p))
}

var kindSpecificConfig clabnodes.KindSpecificConfigSpec[KindSpecificConfig]

// portSpeeds validates the port-speeds key and returns its "<port>:<Mb/s>" pairs in port order.
// lastPort is the highest data port the NIC layout holds (ether30 on arm64, ether64 on amd64).
// A port without a link is allowed: one port-speeds list in a group can serve nodes that do not
// link every port.
func portSpeeds(cfg *KindSpecificConfig, lastPort int) (string, error) {
	speeds := map[int]int{}

	for _, ps := range cfg.PortSpeeds {
		first, last, err := portRange(ps.Port, lastPort)
		if err != nil {
			return "", err
		}
		if ps.Speed < 1 || ps.Speed > maxPortSpeed {
			return "", fmt.Errorf(
				"port-speeds: port %q: speed must be between 1 and %d Mb/s, got %d",
				ps.Port, maxPortSpeed, ps.Speed,
			)
		}
		for p := first; p <= last; p++ {
			if _, ok := speeds[p]; ok {
				return "", fmt.Errorf("port-speeds: ether%d is set more than once", p)
			}
			speeds[p] = ps.Speed
		}
	}

	pairs := make([]string, 0, len(speeds))
	for _, p := range slices.Sorted(maps.Keys(speeds)) {
		pairs = append(pairs, fmt.Sprintf("%d:%d", p, speeds[p]))
	}

	return strings.Join(pairs, " "), nil
}

// portRange parses a data port number or an ascending inclusive range such as 2..25, within
// ether2 to ether<lastPort>.
func portRange(port string, lastPort int) (int, int, error) {
	firstText, lastText, isRange := strings.Cut(port, "..")
	if !isRange {
		lastText = firstText
	}

	first, firstErr := strconv.Atoi(firstText)
	last, lastErr := strconv.Atoi(lastText)
	if firstErr != nil || lastErr != nil ||
		strconv.Itoa(first) != firstText || strconv.Itoa(last) != lastText ||
		first < InterfaceOffset || last < first || last > lastPort {
		return 0, 0, fmt.Errorf(
			"port-speeds: port %q must be a port number or an ascending range within %d..%d "+
				"(ether%d to ether%d)",
			port, InterfaceOffset, lastPort, InterfaceOffset, lastPort,
		)
	}

	return first, last, nil
}
