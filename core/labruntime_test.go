package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clablinks "github.com/srl-labs/containerlab/links"
	clabtypes "github.com/srl-labs/containerlab/types"
	"gopkg.in/yaml.v2"
)

type failingExecLabRuntime struct {
	clablabruntime.LabRuntime
	returnCode int
}

type recordingDestroyLabRuntime struct {
	clablabruntime.LabRuntime
	request clablabruntime.DestroyRequest
}

func (r *recordingDestroyLabRuntime) Destroy(
	_ context.Context,
	req clablabruntime.DestroyRequest,
) error {
	r.request = req
	return nil
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

func TestLabRuntimeDeployRequestExpandsMagicVars(t *testing.T) {
	t.Parallel()

	topologyDir := t.TempDir()
	topologyFile := filepath.Join(topologyDir, "lab.clab.yml")
	const topology = `name: lab1
topology:
  defaults:
    kind: linux
    startup-config: configs/__clabNodeName__.cfg
    exec:
      - echo __clabLabName__ __clabNodeName__ __clabDir__ __clabNodeDir__
  kinds:
    linux:
      binds:
        - configs/__clabNodeName__/frr.conf:/etc/frr/frr.conf
  groups:
    routers:
      volumes:
        - state/__clabNodeName__:/state
      copy-to-flash:
        - image-__clabNodeName__.bin
      stages:
        configure:
          exec:
            - command: bash configs/__clabNodeName__.sh
  nodes:
    r1:
      group: routers
    r2:
      group: routers
`
	if err := os.WriteFile(topologyFile, []byte(topology), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := NewContainerLab(
		WithTopoPath(topologyFile, nil),
		WithRuntime(clablabruntime.ClabernetesRuntimeName, nil),
	)
	if err != nil {
		t.Fatal(err)
	}

	req, err := c.labRuntimeDeployRequest()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(req.TopologyDefinition), "__clab") {
		t.Fatalf("runtime definition still contains magic variables:\n%s", req.TopologyDefinition)
	}

	runtimeConfig := &Config{}
	if err := yaml.Unmarshal(req.TopologyDefinition, runtimeConfig); err != nil {
		t.Fatal(err)
	}
	for _, nodeName := range []string{"r1", "r2"} {
		node := runtimeConfig.Topology.Nodes[nodeName]
		if node == nil {
			t.Fatalf("node %s is missing", nodeName)
		}
		if got, want := node.StartupConfig, "configs/"+nodeName+".cfg"; got != want {
			t.Errorf("%s startup-config = %q, want %q", nodeName, got, want)
		}
		wantExec := "echo clab-lab1 " + nodeName + " " +
			filepath.Join(topologyDir, "clab-lab1") + " " +
			filepath.Join(topologyDir, "clab-lab1", nodeName)
		if got := node.Exec; len(got) != 1 || got[0] != wantExec {
			t.Errorf("%s exec = %q, want %q", nodeName, got, wantExec)
		}
		wantBind := "configs/" + nodeName + "/frr.conf:/etc/frr/frr.conf"
		if got := node.Binds; len(got) != 1 || got[0] != wantBind {
			t.Errorf("%s binds = %q, want %q", nodeName, got, wantBind)
		}
		wantVolume := "state/" + nodeName + ":/state"
		if got := node.Volumes; len(got) != 1 || got[0] != wantVolume {
			t.Errorf("%s volumes = %q, want %q", nodeName, got, wantVolume)
		}
		wantStageExec := "bash configs/" + nodeName + ".sh"
		if got := node.Stages.Configure.Execs; len(got) != 1 || got[0].Command != wantStageExec {
			t.Errorf("%s staged exec = %+v, want command %q", nodeName, got, wantStageExec)
		}
		images, ok := node.KindSpecificConfig["copy-to-flash"].([]any)
		if !ok || len(images) != 1 || images[0] != "image-"+nodeName+".bin" {
			t.Errorf("%s copy-to-flash = %#v", nodeName,
				node.KindSpecificConfig["copy-to-flash"])
		}
	}

	if got := c.Config.Topology.Defaults.StartupConfig; got != "configs/__clabNodeName__.cfg" {
		t.Fatalf("source topology was modified: startup-config = %q", got)
	}
}

func TestLabRuntimeTopologyPreservesLinksWhenExpandingMagicVars(t *testing.T) {
	t.Parallel()

	c := &CLab{
		TopoPaths: &clabtypes.TopoPaths{},
		renderedTopology: []byte(`name: lab1
topology:
  defaults:
    kind: linux
  nodes:
    r1:
      exec:
        - echo __clabNodeName__
    r2: {}
  links:
    - endpoints: ["r1:eth1", "r2:eth1"]
    - endpoints: ["r1:eth2", "host:r1-eth2"]
    - type: host
      host-interface: r1-eth3
      endpoint:
        node: r1
        interface: eth3
    - type: mgmt-net
      host-interface: r1-eth4
      endpoint:
        node: r1
        interface: eth4
    - type: vxlan-stitch
      remote: 192.0.2.1
      vni: 100
      endpoint:
        node: r1
        interface: eth5
`),
	}

	definition, err := c.labRuntimeTopologyWithExpandedMagicVars()
	if err != nil {
		t.Fatal(err)
	}
	config := &Config{}
	if err := yaml.Unmarshal(definition, config); err != nil {
		t.Fatal(err)
	}
	if got := len(config.Topology.Links); got != 5 {
		t.Fatalf("link count = %d, want 5", got)
	}

	veth, ok := config.Topology.Links[0].Link.(*clablinks.LinkVEthRaw)
	if !ok || len(veth.Endpoints) != 2 {
		t.Fatalf("brief veth link was not preserved: %#v", config.Topology.Links[0].Link)
	}
	for _, index := range []int{1, 2} {
		host, ok := config.Topology.Links[index].Link.(*clablinks.LinkHostRaw)
		if !ok || host.Endpoint == nil {
			t.Fatalf(
				"host link %d was not preserved: %#v",
				index,
				config.Topology.Links[index].Link,
			)
		}
		wantInterface := "r1-eth2"
		wantNodeInterface := "eth2"
		if index == 2 {
			wantInterface = "r1-eth3"
			wantNodeInterface = "eth3"
		}
		if host.HostInterface != wantInterface || host.Endpoint.Node != "r1" ||
			host.Endpoint.Iface != wantNodeInterface {
			t.Errorf("host link %d = %#v", index, host)
		}
	}
	mgmt, ok := config.Topology.Links[3].Link.(*clablinks.LinkMgmtNetRaw)
	if !ok || mgmt.Endpoint == nil || mgmt.HostInterface != "r1-eth4" ||
		mgmt.Endpoint.Iface != "eth4" {
		t.Fatalf("mgmt-net link was not preserved: %#v", config.Topology.Links[3].Link)
	}
	stitch, ok := config.Topology.Links[4].Link.(*clablinks.LinkVxlanRaw)
	if !ok || stitch.LinkType != clablinks.LinkTypeVxlanStitch ||
		stitch.Remote != "192.0.2.1" || stitch.VNI != 100 || stitch.Endpoint.Iface != "eth5" {
		t.Fatalf("vxlan-stitch link was not preserved: %#v", config.Topology.Links[4].Link)
	}
}

func TestLabRuntimeTopologyWithoutMagicVarsReturnsOriginalDefinition(t *testing.T) {
	t.Parallel()

	original := []byte(`name: lab1
topology:
  defaults: &defaults
    kind: linux # retain this comment
  nodes:
    r1:
      <<: *defaults
  links:
    - endpoints: ["r1:eth1", "host:r1-eth1"]
`)
	c := &CLab{renderedTopology: original}

	got, err := c.labRuntimeTopologyWithExpandedMagicVars()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("definition without magic variables was rewritten:\n%s", got)
	}
}

func TestApplyLabRuntimeDeployOptionsCopiesMaxWorkers(t *testing.T) {
	t.Parallel()

	req := &clablabruntime.DeployRequest{}
	applyLabRuntimeDeployOptions(req, &DeployOptions{maxWorkers: 7})

	if req.MaxWorkers != 7 {
		t.Fatalf("runtime max workers = %d, want 7", req.MaxWorkers)
	}
}

func TestDestroyWithLabRuntimeRejectsNodeFilterBeforeDeletion(t *testing.T) {
	t.Parallel()

	c := &CLab{
		Config:            &Config{Name: "lab1"},
		LabRuntime:        &failingExecLabRuntime{},
		globalRuntimeName: clablabruntime.ClabernetesRuntimeName,
	}

	err := c.destroyWithLabRuntime(
		context.Background(),
		&DestroyOptions{nodeFilter: []string{"node1"}},
	)
	if err == nil || !strings.Contains(err.Error(), "no resources were deleted") {
		t.Fatalf("destroyWithLabRuntime() error = %v, want safe node-filter rejection", err)
	}
}

func TestDestroyWithLabRuntimeCopiesMaxWorkers(t *testing.T) {
	t.Parallel()

	runtime := &recordingDestroyLabRuntime{}
	c := &CLab{
		Config:            &Config{Name: "lab1"},
		LabRuntime:        runtime,
		globalRuntimeName: clablabruntime.ClabernetesRuntimeName,
	}

	if err := c.destroyWithLabRuntime(
		context.Background(),
		&DestroyOptions{maxWorkers: 7},
	); err != nil {
		t.Fatal(err)
	}
	if runtime.request.MaxWorkers != 7 {
		t.Fatalf("runtime max workers = %d, want 7", runtime.request.MaxWorkers)
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
