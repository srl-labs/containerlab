package types

import (
	"fmt"

	"github.com/srl-labs/containerlab/clab/exec"
)

const (
	// WaitForCreate is the wait stage name for a node creation stage.
	WaitForCreate WaitForStage = "create"
	// WaitForCreateLinks is the wait stage name for a node create-links stage.
	WaitForCreateLinks WaitForStage = "create-links"
	// WaitForConfigure is the wait stage name for a node configure stage.
	WaitForConfigure WaitForStage = "configure"
	// WaitForHealthy is the wait stage name for a node healthy stage.
	WaitForHealthy WaitForStage = "healthy"
	// WaitForExit is the wait stage name for a node exit stage.
	WaitForExit WaitForStage = "exit"
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
func (s *Stages) GetWaitFor() map[WaitForStage]WaitForList {
	result := map[WaitForStage]WaitForList{}

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
		err = s.Configure.Merge(other.Configure)
		if err != nil {
			return err
		}
	}
	if other.Create != nil {
		err = s.Create.Merge(other.Create)
		if err != nil {
			return err
		}
	}
	if other.CreateLinks != nil {
		err = s.CreateLinks.Merge(other.CreateLinks)
		if err != nil {
			return err
		}
	}
	if other.Healthy != nil {
		err = s.Healthy.Merge(other.Healthy)
		if err != nil {
			return err
		}
	}
	if other.Exit != nil {
		err = s.Exit.Merge(other.Exit)
		if err != nil {
			return err
		}
	}
	return err
}

type ExecContainer struct {
	// list of commands to run on the host
	ExecCommandsHost []string `yaml:"exec,omitempty"`
}

func (e *ExecContainer) Merge(other *ExecContainer) {
	e.ExecCommandsHost = append(e.ExecCommandsHost, other.ExecCommandsHost...)
}

func (e *ExecContainer) GetExecs() ([]*exec.ExecCmd, error) {
	ex := []*exec.ExecCmd{}
	for _, c := range e.ExecCommandsHost {
		newCmd, err := exec.NewExecCmdFromString(c)
		if err != nil {
			return nil, err
		}
		ex = append(ex, newCmd)
	}
	return ex, nil
}

// StageCreate represents a creation stage of a given node.
type StageCreate struct {
	StageBase     `yaml:",inline"`
	ExecContainer `yaml:",inline"`
}

func (s *StageCreate) Merge(other *StageCreate) error {
	err := s.StageBase.Merge(&other.StageBase)
	if err != nil {
		return err
	}
	s.ExecContainer.Merge(&other.ExecContainer)
	return nil
}

// StageCreateLinks represents a stage of a given node when links are getting added to it.
type StageCreateLinks struct {
	StageBase     `yaml:",inline"`
	ExecContainer `yaml:",inline"`
}

func (s *StageCreateLinks) Merge(other *StageCreateLinks) error {
	err := s.StageBase.Merge(&other.StageBase)
	if err != nil {
		return err
	}
	s.ExecContainer.Merge(&other.ExecContainer)
	return nil
}

// StageConfigure represents a stage of a given node when it enters configuration workflow.
type StageConfigure struct {
	StageBase     `yaml:",inline"`
	ExecContainer `yaml:",inline"`
}

func (s *StageConfigure) Merge(other *StageConfigure) error {
	err := s.StageBase.Merge(&other.StageBase)
	if err != nil {
		return err
	}
	s.ExecContainer.Merge(&other.ExecContainer)
	return nil
}

// StageHealthy represents a stage of a given node when it reaches healthy status.
type StageHealthy struct {
	StageBase     `yaml:",inline"`
	ExecContainer `yaml:",inline"`
}

func (s *StageHealthy) Merge(other *StageHealthy) error {
	err := s.StageBase.Merge(&other.StageBase)
	if err != nil {
		return err
	}
	s.ExecContainer.Merge(&other.ExecContainer)
	return nil
}

// StageExit represents a stage of a given node when the node reaches exit state.
type StageExit struct {
	StageBase     `yaml:",inline"`
	ExecContainer `yaml:",inline"`
}

func (s *StageExit) Merge(other *StageExit) error {
	err := s.StageBase.Merge(&other.StageBase)
	if err != nil {
		return err
	}
	s.ExecContainer.Merge(&other.ExecContainer)
	return nil
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

// WaitForStage defines the stages that nodes go through
// during the deployment process. They are used to define and enforce
// dependencies between nodes.
type WaitForStage string

// WaitFor represents the wait-for configuration for a node deployment stage.
type WaitFor struct {
	Node  string       `json:"node"`            // the node that is to be waited for
	Stage WaitForStage `json:"stage,omitempty"` // the stage that the node must have completed
}

// Equals returns true if the Node and the State of the WaitFor structs are value equal.
func (w *WaitFor) Equals(other *WaitFor) bool {
	if w.Node == other.Node && w.Stage == other.Stage {
		return true
	}

	return false
}

// GetWaitForStages returns list of wait for stages that are used to init Waitgroups
// for all the states.
func GetWaitForStages() []WaitForStage {
	return []WaitForStage{
		WaitForCreate, WaitForCreateLinks,
		WaitForConfigure, WaitForHealthy, WaitForExit,
	}
}

func WaitForStageFromString(s string) (WaitForStage, error) {
	for _, val := range GetWaitForStages() {
		if s == string(val) {
			return val, nil
		}
	}

	return "", fmt.Errorf("unknown stage %q", s)
}
