package core

import (
	"context"
	"testing"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

type recordingDeployLabRuntime struct {
	recordingDestroyLabRuntime
	deployed bool
}

func (r *recordingDeployLabRuntime) Deploy(
	_ context.Context,
	req clablabruntime.DeployRequest,
) (*clablabruntime.LabState, error) {
	r.deployed = true

	return &clablabruntime.LabState{Name: req.Name, Namespace: "c9s-" + req.Name}, nil
}

func TestContainerFromLabNodeReportsWorkload(t *testing.T) {
	t.Parallel()

	c := &CLab{Config: &Config{Name: "lab1"}, TopoPaths: &clabtypes.TopoPaths{}}
	ports := []clablabruntime.ExposedPort{{Port: 22, ExposePort: 22, Protocol: "tcp"}}

	container := c.containerFromLabNode(
		&clablabruntime.LabState{Name: "lab1", Namespace: "c9s-lab1"},
		clablabruntime.NodeState{
			Name:                "r1",
			Pod:                 "lab1-r1-abc",
			Container:           "node-r1-x",
			NodeUID:             "uid-1",
			ExposedPorts:        ports,
			LoadBalancerAddress: "192.0.2.10",
		},
	)

	if container.ID != "c9s-lab1/lab1/r1" || container.ShortID != container.ID {
		t.Fatalf("ID = %q, ShortID = %q; want the unique <namespace>/<lab>/<node> id",
			container.ID, container.ShortID)
	}
	w := container.Workload
	if w == nil || w.Namespace != "c9s-lab1" || w.Pod != "lab1-r1-abc" ||
		w.Container != "node-r1-x" || w.NodeUID != "uid-1" ||
		w.LoadBalancerAddress != "192.0.2.10" || len(w.ExposedPorts) != 1 {
		t.Fatalf("workload = %+v", w)
	}

	other := c.containerFromLabNode(
		&clablabruntime.LabState{Name: "lab1", Namespace: "c9s-lab1"},
		clablabruntime.NodeState{Name: "r2"},
	)
	if other.ShortID == container.ShortID {
		t.Fatal("two nodes share a container ID")
	}
}

func TestDestroyWithLabRuntimePassesCleanup(t *testing.T) {
	t.Parallel()

	for _, cleanup := range []bool{false, true} {
		runtime := &recordingDestroyLabRuntime{}
		c := &CLab{
			Config:            &Config{Name: "lab1"},
			LabRuntime:        runtime,
			TopoPaths:         &clabtypes.TopoPaths{},
			globalRuntimeName: clablabruntime.ClabernetesRuntimeName,
		}

		if err := c.destroyWithLabRuntime(
			context.Background(),
			&DestroyOptions{cleanup: cleanup},
		); err != nil {
			t.Fatal(err)
		}
		if runtime.request.Cleanup != cleanup {
			t.Fatalf("destroy cleanup = %t, want %t", runtime.request.Cleanup, cleanup)
		}
	}
}

func TestDeployReconfigureWithLabRuntimeCleansUp(t *testing.T) {
	t.Parallel()

	runtime := &recordingDeployLabRuntime{}
	c := &CLab{
		Config:            &Config{Name: "lab1"},
		LabRuntime:        runtime,
		TopoPaths:         &clabtypes.TopoPaths{},
		renderedTopology:  []byte("name: lab1\n"),
		globalRuntimeName: clablabruntime.ClabernetesRuntimeName,
	}

	if _, err := c.deployWithLabRuntime(
		context.Background(),
		&DeployOptions{reconfigure: true},
	); err != nil {
		t.Fatal(err)
	}
	if !runtime.request.Cleanup || !runtime.deployed {
		t.Fatalf("reconfigure destroy = %+v, deployed = %t; want a cleanup destroy",
			runtime.request, runtime.deployed)
	}
}

func TestCheckConnectivitySkipsLocalRuntimesForLabRuntimes(t *testing.T) {
	t.Parallel()

	c := &CLab{LabRuntime: &recordingDestroyLabRuntime{}}
	if err := c.CheckConnectivity(context.Background()); err != nil {
		t.Fatalf("CheckConnectivity() = %v, want nil for a lab runtime", err)
	}
}
