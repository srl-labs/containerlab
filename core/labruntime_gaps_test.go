package core

import (
	"context"
	"strings"
	"testing"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clablinks "github.com/srl-labs/containerlab/links"
	clabtypes "github.com/srl-labs/containerlab/types"
	"gopkg.in/yaml.v2"
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

type requestRecordingLabRuntime struct {
	recordingDeployLabRuntime
	requested bool
}

func (r *requestRecordingLabRuntime) Plan(
	context.Context,
	clablabruntime.DeployRequest,
) (*clablabruntime.DeployPlan, error) {
	r.requested = true

	return &clablabruntime.DeployPlan{}, nil
}

func (r *requestRecordingLabRuntime) Manifests(
	context.Context,
	clablabruntime.DeployRequest,
) ([]clablabruntime.Manifest, error) {
	r.requested = true

	return nil, nil
}

func (r *requestRecordingLabRuntime) Validate(context.Context, clablabruntime.DeployRequest) error {
	r.requested = true

	return nil
}

func (r *requestRecordingLabRuntime) Start(context.Context, clablabruntime.NodeRequest) error {
	r.requested = true

	return nil
}

func (r *requestRecordingLabRuntime) Restart(context.Context, clablabruntime.NodeRequest) error {
	r.requested = true

	return nil
}

// labRuntimeDeployPaths run the commands that send the topology or the nodes to start or restart
// to the lab runtime, or check the topology before a redeploy, by name.
func labRuntimeDeployPaths(ctx context.Context) map[string]func(c *CLab) error {
	return map[string]func(c *CLab) error{
		"deploy": func(c *CLab) error {
			_, err := c.deployWithLabRuntime(ctx, &DeployOptions{reconfigure: true})
			return err
		},
		"dry run": func(c *CLab) error {
			options, err := NewDeployOptions(0)
			if err != nil {
				return err
			}
			_, err = c.Deploy(ctx, options.SetDryRun(true))
			return err
		},
		"manifests": func(c *CLab) error {
			_, err := c.LabRuntimeManifests(ctx, nil)
			return err
		},
		"validate": func(c *CLab) error {
			return c.ValidateLabRuntimeTopology(ctx)
		},
		"start": func(c *CLab) error {
			return c.StartNodes(ctx, nil)
		},
		"restart": func(c *CLab) error {
			return c.RestartNodes(ctx, nil)
		},
		"check before a redeploy": func(c *CLab) error {
			return c.CheckLabRuntimeDeploy()
		},
	}
}

// impairedLabRuntimeLab is a lab with a link impairment on the given lab runtime.
func impairedLabRuntimeLab(t *testing.T, runtime clablabruntime.LabRuntime) *CLab {
	t.Helper()

	var link clablinks.LinkDefinition
	if err := yaml.Unmarshal([]byte(`
endpoints: ["r1:eth1", "r2:eth1"]
netem: [{delay: 10ms}]
`), &link); err != nil {
		t.Fatal(err)
	}

	return &CLab{
		Config: &Config{
			Name:     "lab1",
			Topology: &clabtypes.Topology{Links: []*clablinks.LinkDefinition{&link}},
		},
		LabRuntime:        runtime,
		TopoPaths:         &clabtypes.TopoPaths{},
		renderedTopology:  []byte("name: lab1\n"),
		globalRuntimeName: clablabruntime.ClabernetesRuntimeName,
	}
}

func TestLabRuntimeRejectsLinkImpairments(t *testing.T) {
	t.Parallel()

	for name, run := range labRuntimeDeployPaths(context.Background()) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runtime := &requestRecordingLabRuntime{}

			err := run(impairedLabRuntimeLab(t, runtime))
			if err == nil ||
				!strings.Contains(err.Error(), "link impairments (netem) are not supported") {
				t.Fatalf("%s = %v; want the link impairments rejected", name, err)
			}
			if runtime.requested || runtime.deployed || runtime.request.Name != "" {
				t.Fatal("the request reached the lab runtime")
			}
		})
	}
}

type impairingLabRuntime struct {
	requestRecordingLabRuntime
	clablabruntime.LinkImpairer
}

func TestLinkImpairingLabRuntimeKeepsLinkImpairments(t *testing.T) {
	t.Parallel()

	for name, run := range labRuntimeDeployPaths(context.Background()) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runtime := &impairingLabRuntime{}

			if err := run(impairedLabRuntimeLab(t, runtime)); err != nil {
				t.Fatal(err)
			}
			if name != "check before a redeploy" && !runtime.requested && !runtime.deployed {
				t.Fatal("the request didn't reach the lab runtime")
			}
		})
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
