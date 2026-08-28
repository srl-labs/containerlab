package core

import (
	"context"
	"strings"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

type failingExecLabRuntime struct {
	clablabruntime.LabRuntime
	returnCode int
}

func (r *failingExecLabRuntime) Inspect(
	context.Context,
	clablabruntime.InspectRequest,
) (*clablabruntime.LabState, error) {
	return &clablabruntime.LabState{
		Name: "lab1", Namespace: "lab-ns", Nodes: []clablabruntime.NodeState{{Name: "node1"}},
	}, nil
}

func (r *failingExecLabRuntime) Exec(
	_ context.Context,
	req clablabruntime.ExecRequest,
) (*clabexec.ExecResult, error) {
	return &clabexec.ExecResult{Cmd: req.Command, ReturnCode: r.returnCode, Stderr: "failed"}, nil
}

func TestExecWithLabRuntimeReturnsNestedFailureAndPreservesResult(t *testing.T) {
	t.Parallel()

	runtime := &failingExecLabRuntime{returnCode: 7}
	c := &CLab{
		Config:            &Config{Name: "lab1", Topology: clabtypes.NewTopology()},
		TopoPaths:         &clabtypes.TopoPaths{},
		LabRuntime:        runtime,
		globalRuntimeName: clablabruntime.ClabernetesRuntimeName,
	}

	results, err := c.execWithLabRuntime(context.Background(), []string{"false"})
	if err == nil || !strings.Contains(err.Error(), "exit code 7") {
		t.Fatalf("execWithLabRuntime() error = %v, want exit code 7", err)
	}
	if results == nil {
		t.Fatal("execWithLabRuntime() discarded the failed command result")
	}
	dumped, dumpErr := results.Dump("json")
	if dumpErr != nil {
		t.Fatal(dumpErr)
	}
	if !strings.Contains(dumped, `"return-code": 7`) {
		t.Fatalf("failed result was not retained: %s", dumped)
	}
}

func TestResolveRuntimeNameLowercasesValue(t *testing.T) {
	t.Setenv("CLAB_RUNTIME", "C9S")

	if got := resolveRuntimeName(""); got != "c9s" {
		t.Fatalf("resolveRuntimeName() = %q, want c9s", got)
	}
	if got := resolveRuntimeName("c9S"); got != "c9s" {
		t.Fatalf("resolveRuntimeName(c9S) = %q, want c9s", got)
	}
}

func TestDestroyWithLabRuntimeRejectsNodeFilterBeforeDeletion(t *testing.T) {
	t.Parallel()

	c := &CLab{
		Config:            &Config{Name: "lab1"},
		LabRuntime:        &failingExecLabRuntime{},
		globalRuntimeName: clablabruntime.ClabernetesRuntimeName,
	}

	err := c.destroyWithLabRuntime(context.Background(), &DestroyOptions{nodeFilter: []string{"node1"}})
	if err == nil || !strings.Contains(err.Error(), "no resources were deleted") {
		t.Fatalf("destroyWithLabRuntime() error = %v, want safe node-filter rejection", err)
	}
}

func TestSanitizeLabRuntimeNames(t *testing.T) {
	t.Parallel()

	topology := clabtypes.NewTopology()
	topology.Nodes["R1"] = &clabtypes.NodeDefinition{Kind: "linux"}

	c := &CLab{Config: &Config{Name: "SRv6_Lab", Topology: topology}}
	if err := c.sanitizeLabRuntimeNames(); err != nil {
		t.Fatal(err)
	}
	if c.Config.Name != "srv6-lab" {
		t.Fatalf("lab name = %q, want srv6-lab", c.Config.Name)
	}
	// The node keeps the name the topology file wrote; the runtime renames it on the way to
	// Kubernetes, and everything containerlab renders locally stays readable.
	if _, ok := topology.Nodes["R1"]; !ok {
		t.Fatalf("topology nodes = %v, want R1 to be left alone", topology.Nodes)
	}

	topology.Nodes["r1"] = &clabtypes.NodeDefinition{Kind: "linux"}
	if err := c.sanitizeLabRuntimeNames(); err == nil {
		t.Fatal("sanitizeLabRuntimeNames() error = nil, want a colliding node name error")
	}
}

func TestContainerFromLabNodeResolvesSanitizedNodeNames(t *testing.T) {
	t.Parallel()

	topology := clabtypes.NewTopology()
	topology.Nodes["R1"] = &clabtypes.NodeDefinition{
		Kind:  "nokia_srsim",
		Image: "sros:25.7",
		Group: "spine",
	}

	c := &CLab{
		Config:    &Config{Name: "lab1", Topology: topology},
		TopoPaths: &clabtypes.TopoPaths{},
	}

	container := c.containerFromLabNode(
		&clablabruntime.LabState{Name: "lab1", Namespace: "c9s-lab1"},
		clablabruntime.NodeState{Name: "r1"},
	)
	if container.Image != "sros:25.7" ||
		container.Labels[clabconstants.NodeKind] != "nokia_srsim" ||
		container.Labels[clabconstants.NodeGroup] != "spine" {
		t.Fatalf("container = %+v, want the R1 node definition behind the sanitized name",
			container)
	}
}
