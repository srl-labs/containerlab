package labruntime

import (
	"context"
	"fmt"
	"time"
)

const (
	ClabernetesRuntimeName = "clabernetes"
)

type Config struct {
	Debug   bool
	Timeout time.Duration
}

type DeployRequest struct {
	Name               string
	Namespace          string
	TopologyDefinition []byte
	Wait               bool
	Timeout            time.Duration
}

type DestroyRequest struct {
	Name      string
	Namespace string
	Wait      bool
	Timeout   time.Duration
}

type InspectRequest struct {
	Name      string
	Namespace string
}

type ListRequest struct {
	Namespace     string
	AllNamespaces bool
}

type RuntimeCapabilities struct {
	Deploy  bool
	Destroy bool
	Inspect bool
	List    bool
}

type NodeState struct {
	Name                string
	Kind                string
	Image               string
	State               string
	Ready               bool
	LoadBalancerAddress string
}

type LabState struct {
	Name         string
	Namespace    string
	TopologyPath string
	State        string
	Ready        bool
	Nodes        []NodeState
}

type LabRuntime interface {
	Deploy(context.Context, DeployRequest) (*LabState, error)
	Destroy(context.Context, DestroyRequest) error
	Inspect(context.Context, InspectRequest) (*LabState, error)
	List(context.Context, ListRequest) ([]*LabState, error)
	Capabilities() RuntimeCapabilities
}

type Initializer func(Config) (LabRuntime, error)

var LabRuntimes = map[string]Initializer{}

func Register(name string, init Initializer) {
	LabRuntimes[name] = init
}

func IsLabRuntimeName(name string) bool {
	_, ok := LabRuntimes[name]
	return ok
}

func Init(name string, cfg Config) (LabRuntime, error) {
	init, ok := LabRuntimes[name]
	if !ok {
		return nil, fmt.Errorf("unknown lab runtime %q", name)
	}

	return init(cfg)
}
