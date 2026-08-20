package core

import (
	"context"
	"strings"
	"testing"

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
