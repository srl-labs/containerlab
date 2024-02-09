package types

import "fmt"

const (
	// WaitForCreate is the wait phase name for a node creation stage.
	WaitForCreate WaitForPhase = "create"
	// WaitForCreateLinks is the wait phase name for a node create-links stage.
	WaitForCreateLinks WaitForPhase = "create-links"
	// WaitForConfigure is the wait phase name for a node configure stage.
	WaitForConfigure WaitForPhase = "configure"
	// WaitForHealthy is the wait phase name for a node healthy stage.
	WaitForHealthy WaitForPhase = "healthy"
	// WaitForExit is the wait phase name for a node exit stage.
	WaitForExit WaitForPhase = "exit"
)

// Stages represents a configuration of a given node deployment stage.
type Stages struct {
	Create      *StageCreate      `yaml:"create"`
	CreateLinks *StageCreateLinks `yaml:"create-links"`
	Configure   *StageConfigure   `yaml:"configure"`
	Healthy     *StageHealthy     `yaml:"healthy"`
	Exit        *StageExit        `yaml:"exit"`
}

// NewStages returns a new Stages instance.
func NewStages() *Stages {
	return &Stages{
		Create: &StageCreate{
			StageBase: StageBase{},
		},
		CreateLinks: &StageCreateLinks{
			StageBase: StageBase{},
		},
		Configure: &StageConfigure{
			StageBase: StageBase{},
		},
		Healthy: &StageHealthy{
			StageBase: StageBase{},
		},
		Exit: &StageExit{
			StageBase: StageBase{},
		},
	}
}

// GetWaitFor returns lists of nodes that need to be waited for in a map
// that is indexed by the state for which this dependency is to be evaluated.
func (s *Stages) GetWaitFor() map[WaitForPhase]WaitForList {
	result := map[WaitForPhase]WaitForList{}

	result[WaitForConfigure] = s.Configure.WaitFor
	result[WaitForCreate] = s.Create.WaitFor
	result[WaitForCreateLinks] = s.CreateLinks.WaitFor
	result[WaitForHealthy] = s.Healthy.WaitFor
	result[WaitForExit] = s.Exit.WaitFor

	return result
}

// Merge merges stage other into stage s.
// WaitFor merge strategy is done by appending WaitFor from other to s,
// instead of overwriting the WaitFor list in s. This is done to ensure
// that WaitFor dependencies can be augmented by more specific stages.
func (s *Stages) Merge(other *Stages) error {
	var err error
	if other.Configure != nil {
		err = s.Configure.Merge(&other.Configure.StageBase)
		if err != nil {
			return err
		}
	}
	if other.Create != nil {
		err = s.Create.Merge(&other.Create.StageBase)
		if err != nil {
			return err
		}
	}
	if other.CreateLinks != nil {
		err = s.CreateLinks.Merge(&other.CreateLinks.StageBase)
		if err != nil {
			return err
		}
	}
	if other.Healthy != nil {
		err = s.Healthy.Merge(&other.Healthy.StageBase)
		if err != nil {
			return err
		}
	}
	if other.Exit != nil {
		err = s.Exit.Merge(&other.Exit.StageBase)
		if err != nil {
			return err
		}
	}
	return err
}

// StageCreate represents a creation stage of a given node.
type StageCreate struct {
	StageBase `yaml:",inline"`
}

// StageCreateLinks represents a stage of a given node when links are getting added to it.
type StageCreateLinks struct {
	StageBase `yaml:",inline"`
}

// StageConfigure represents a stage of a given node when it enters configuration workflow.
type StageConfigure struct {
	StageBase `yaml:",inline"`
}

// StageHealthy represents a stage of a given node when it reaches healthy status.
type StageHealthy struct {
	StageBase `yaml:",inline"`
}

// StageExit represents a stage of a given node when the node reaches exit state.
type StageExit struct {
	StageBase `yaml:",inline"`
}

// StageBase represents a common configuration stage.
// Other stages embed this type to inherit its configuration options.
// A particular member of the StageBase type is a list of WaitFor configurations.
type StageBase struct {
	WaitFor WaitForList `yaml:"wait-for,omitempty"`
}

// WaitForList is a list of WaitFor configurations.
type WaitForList []*WaitFor

// contains returns true if the WaitForList contains the given WaitFor.
func (wfl WaitForList) contains(wf *WaitFor) bool {
	for _, entry := range wfl {
		if entry.Equals(wf) {
			return true
		}
	}

	return false
}

// Merge merges base stage from sc into s by
// appending WaitFor from sc to s.
func (s *StageBase) Merge(sc *StageBase) error {
	if sc == nil {
		return nil
	}

	for _, wf := range sc.WaitFor {
		// prevent adding the same dependency twice
		if s.WaitFor.contains(wf) {
			continue
		}
		s.WaitFor = append(s.WaitFor, wf)
	}

	return nil
}

// WaitForPhase defines the phases that nodes go through
// during the deployment process. They are used to define and enforce
// dependencies between nodes.
type WaitForPhase string

// WaitFor represents the wait-for configuration for a node deployment stage.
type WaitFor struct {
	Node  string       `json:"node"`            // the node that is to be waited for
	State WaitForPhase `json:"state,omitempty"` // the state that the node must have completed
}

// Equals returns true if the Node and the State of the WaitFor structs are value equal.
func (w *WaitFor) Equals(other *WaitFor) bool {
	if w.Node == other.Node && w.State == other.State {
		return true
	}

	return false
}

// GetWaitForPhases returns list of wait for phases that are used to init Waitgroups
// for all the states.
func GetWaitForPhases() []WaitForPhase {
	return []WaitForPhase{
		WaitForCreate, WaitForCreateLinks,
		WaitForConfigure, WaitForHealthy, WaitForExit,
	}
}

func WaitForPhaseFromString(s string) (WaitForPhase, error) {
	for _, val := range GetWaitForPhases() {
		if s == string(val) {
			return val, nil
		}
	}

	return "", fmt.Errorf("unknown phase %q", s)
}
