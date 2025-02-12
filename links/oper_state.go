package links

import "github.com/charmbracelet/log"

type OperState string

const (
	up   OperState = "up"
	down OperState = "down"
)

// Map of string values to OperState enum
var operStateMap = map[string]OperState{
	"up":      up,
	"down":    down,
	"enable":  up,
	"disable": down,
}

// Reverse map for string conversion
var operStateToString = map[OperState]string{
	up:   "up",
	down: "down",
}

// UnmarshalYAML Implement UnmarshalYAML for OperState
func (s *OperState) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var operStateStr string
	if err := unmarshal(&operStateStr); err != nil {
		return err
	}

	if state, exists := operStateMap[operStateStr]; exists {
		*s = state
	} else {
		log.Errorf("oper-state value of %s not supported, defaulting to \"up\"", operStateStr)
		*s = up // Default to up
	}
	return nil
}

// MarshalYAML Implement MarshalYAML for OperState
func (s *OperState) MarshalYAML() (interface{}, error) {
	return operStateToString[*s], nil
}
