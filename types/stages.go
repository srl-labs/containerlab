package types

import (
	"fmt"

	clabexec "github.com/srl-labs/containerlab/exec"
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

var (
	// the defauts we need as pointers, so assign them to vars, such that we can acquire the pointer.
	defaultCommandExecutionPhase = CommandExecutionPhaseEnter
	defaultCommandTarget         = CommandTargetContainer
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
			StageBase: StageBase{
				Execs: Execs{},
			},
		},
		CreateLinks: &StageCreateLinks{
			StageBase: StageBase{
				Execs: Execs{},
			},
		},
		Configure: &StageConfigure{
			StageBase: StageBase{
				Execs: Execs{},
			},
		},
		Healthy: &StageHealthy{
			StageBase: StageBase{
				Execs: Execs{},
			},
		},
		Exit: &StageExit{
			StageBase: StageBase{
				Execs: Execs{},
			},
		},
	}
}

// InitDefaults set defaults for the stages.
func (s *Stages) InitDefaults() {
	s.Configure.Execs.InitDefaults()
	s.Create.Execs.InitDefaults()
	s.CreateLinks.Execs.InitDefaults()
	s.Healthy.Execs.InitDefaults()
	s.Exit.Execs.InitDefaults()
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

// Execs represents a list of Exec configurations.
type Execs []*Exec

// HasCommands returns true if the Execs contains at least one command.
func (c Execs) HasCommands() bool {
	return len(c) > 0
}

// InitDefaults sets default values for Execs.
func (execs Execs) InitDefaults() {
	for _, e := range execs {
		e.InitDefaults()
	}
}

type Exec struct {
	Command string     `yaml:"command,omitempty"`
	Target  ExecTarget `yaml:"target,omitempty"`
	Phase   ExecPhase  `yaml:"phase,omitempty"`
}

// InitDefaults sets default values for Exec.
func (c *Exec) InitDefaults() {
	// default the phase to on-enter
	if c.Phase == "" {
		c.Phase = defaultCommandExecutionPhase
	}
	// default target to container
	if c.Target == "" {
		c.Target = defaultCommandTarget
	}
}

func (c *Exec) GetExecCmd() (*clabexec.ExecCmd, error) {
	return clabexec.NewExecCmdFromString(c.Command)
}

func (c *Exec) String() string {
	return fmt.Sprintf("phase: %s, command: %s, target: %s", c.Phase, c.Command, c.Target)
}

type ExecPhase string

const (
	// CommandExecutionPhaseEnter represents a command to be executed when the node enters the stage.
	CommandExecutionPhaseEnter ExecPhase = "on-enter"
	// CommandExecutionPhaseExit represents a command to be executed when the node exits the stage.
	CommandExecutionPhaseExit ExecPhase = "on-exit"
)

type ExecTarget string

const (
	// CommandTargetContainer determines that the commands are meant to be executed within the container.
	CommandTargetContainer ExecTarget = "container"
	// CommandTargetHost determines that the commands are meant to be executed on the host system.
	CommandTargetHost ExecTarget = "host"
)

// StageCreate represents a creation stage of a given node.
type StageCreate struct {
	StageBase `yaml:",inline"`
}

func (s *StageCreate) Merge(other *StageCreate) error {
	err := s.StageBase.Merge(&other.StageBase)
	if err != nil {
		return err
	}

	return nil
}

// StageCreateLinks represents a stage of a given node when links are getting added to it.
type StageCreateLinks struct {
	StageBase `yaml:",inline"`
}

func (s *StageCreateLinks) Merge(other *StageCreateLinks) error {
	err := s.StageBase.Merge(&other.StageBase)
	if err != nil {
		return err
	}

	return nil
}

// StageConfigure represents a stage of a given node when it enters configuration workflow.
type StageConfigure struct {
	StageBase `yaml:",inline"`
}

func (s *StageConfigure) Merge(other *StageConfigure) error {
	err := s.StageBase.Merge(&other.StageBase)
	if err != nil {
		return err
	}

	return nil
}

// StageHealthy represents a stage of a given node when it reaches healthy status.
type StageHealthy struct {
	StageBase `yaml:",inline"`
}

func (s *StageHealthy) Merge(other *StageHealthy) error {
	err := s.StageBase.Merge(&other.StageBase)
	if err != nil {
		return err
	}

	return nil
}

// StageExit represents a stage of a given node when the node reaches exit state.
type StageExit struct {
	StageBase `yaml:",inline"`
}

func (s *StageExit) Merge(other *StageExit) error {
	err := s.StageBase.Merge(&other.StageBase)
	if err != nil {
		return err
	}

	return nil
}

// StageBase represents a common configuration stage.
// Other stages embed this type to inherit its configuration options.
type StageBase struct {
	WaitFor WaitForList `yaml:"wait-for,omitempty"`
	Execs   Execs       `yaml:"exec,omitempty"`
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

// Merge merges base stage from sc into s.
// Merging for WaitFor and Exec commands is done by appending from sc to s without duplicates.
func (s *StageBase) Merge(sb *StageBase) error {
	if sb == nil {
		return nil
	}

	for _, wf := range sb.WaitFor {
		// prevent adding the same dependency twice
		if s.WaitFor.contains(wf) {
			continue
		}
		s.WaitFor = append(s.WaitFor, wf)
	}

	s.Execs = append(s.Execs, sb.Execs...)

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
