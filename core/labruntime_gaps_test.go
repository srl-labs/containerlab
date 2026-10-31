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

// TestContainerFromLabNodeReportsTheTopologyNode checks the labels a lab runtime node reports
// without its topology file: the node name the topology spells, the type and group the
// topology set, the recorded topology file, and its ports as unpublished container ports.
func TestContainerFromLabNodeReportsTheTopologyNode(t *testing.T) {
	t.Parallel()

	c := &CLab{Config: &Config{Name: "lab1"}, TopoPaths: &clabtypes.TopoPaths{}}
	container := c.containerFromLabNode(
		&clablabruntime.LabState{
			Name: "lab1", Namespace: "c9s-lab1", TopologyPath: "/home/alice/lab1.clab.yml",
		},
		clablabruntime.NodeState{
			Name: "r1", TopologyName: "R1", Kind: "nokia_srlinux", Type: "ixr-h2",
			Group: "spines", ContainerState: "running", Health: "healthy",
			ExposedPorts: []clablabruntime.ExposedPort{
				{Port: 57400, ExposePort: 57400, Protocol: "tcp"},
			},
		},
	)

	want := map[string]string{
		"clab-node-name":     "R1",
		"clab-node-longname": "lab1-r1",
		"clab-node-kind":     "nokia_srlinux",
		"clab-node-type":     "ixr-h2",
		"clab-node-group":    "spines",
		"clab-topo-file":     "/home/alice/lab1.clab.yml",
		"containerlab":       "lab1",
	}
	for key, value := range want {
		if container.Labels[key] != value {
			t.Fatalf("label %s = %q, want %q (%v)", key, container.Labels[key], value,
				container.Labels)
		}
	}
	if container.State != "running" || container.Status != "healthy" ||
		container.Names[0] != "lab1-r1" {
		t.Fatalf("container = %s %s %v", container.State, container.Status, container.Names)
	}
	if len(container.Ports) != 1 || container.Ports[0].ContainerPort != 57400 ||
		container.Ports[0].HostPort != 0 || container.Ports[0].Protocol != "tcp" {
		t.Fatalf("ports = %+v", container.Ports)
	}
}
