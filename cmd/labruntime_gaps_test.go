package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestGraphUnderLabRuntimeRendersOnlyFromTheTopologyFile(t *testing.T) {
	tests := []struct {
		name    string
		flags   map[string]string
		wantErr bool
	}{
		{name: "server mode needs running containers", wantErr: true},
		{name: "offline server", flags: map[string]string{"offline": "true"}},
		{name: "dot", flags: map[string]string{"dot": "true"}},
		{name: "mermaid", flags: map[string]string{"mermaid": "true"}},
		{name: "drawio", flags: map[string]string{"drawio": "true"}},
		{name: "explicitly not offline", flags: map[string]string{"offline": "false"}, wantErr: true},
		{
			name:    "node filter",
			flags:   map[string]string{"mermaid": "true", "node-filter": "r1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v = nil
			t.Cleanup(func() { v = nil })

			root := &cobra.Command{Use: "containerlab"}
			graph := &cobra.Command{Use: "graph"}
			root.AddCommand(graph)
			for _, flag := range graphTopologyFileFlags {
				graph.Flags().Bool(flag, false, "")
			}
			graph.Flags().StringSlice("node-filter", nil, "")
			for flag, value := range tt.flags {
				if err := graph.Flags().Set(flag, value); err != nil {
					t.Fatal(err)
				}
			}

			err := checkLabRuntimeCommandSupport(graph, clablabruntime.ClabernetesRuntimeName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkLabRuntimeCommandSupport() error = %v, wantErr %t", err, tt.wantErr)
			}

			if err := checkLabRuntimeCommandSupport(graph, "docker"); err != nil {
				t.Fatalf("docker graph rejected: %v", err)
			}
		})
	}
}

func TestInspectJSONReportsLabRuntimeWorkload(t *testing.T) {
	labels := func(lab, node string) map[string]string {
		return map[string]string{
			clabconstants.Containerlab: lab,
			clabconstants.NodeKind:     "linux",
			clabconstants.TopoFile:     "/labs/" + lab + ".clab.yml",
		}
	}
	containers := []clabruntime.GenericContainer{
		{
			Names:   []string{"clab-docker-r1"},
			ID:      "0123456789abcdef0123",
			ShortID: "0123456789ab",
			Image:   "alpine",
			State:   "running",
			Labels:  labels("docker", "r1"),
		},
		{
			Names:   []string{"c9s-r1"},
			ID:      "c9s-c9s/c9s/r1",
			ShortID: "c9s-c9s/c9s/r1",
			Image:   "alpine",
			State:   "running",
			Labels:  labels("c9s", "r1"),
			Workload: &clabtypes.LabRuntimeWorkload{
				Namespace:           "c9s-c9s",
				Pod:                 "c9s-r1-abc",
				Container:           "node-r1-x",
				NodeUID:             "uid-1",
				ExposedPorts:        []clabtypes.ExposedPort{{Port: 22, ExposePort: 22, Protocol: "tcp"}},
				LoadBalancerAddress: "192.0.2.10",
			},
		},
	}

	o := GetOptions()
	previousFormat := o.Inspect.Format
	o.Inspect.Format = clabconstants.FormatJSON
	t.Cleanup(func() { o.Inspect.Format = previousFormat })

	output := captureApplyOutput(t, func() {
		if err := PrintContainerInspect(containers, o); err != nil {
			t.Fatal(err)
		}
	})

	var labs map[string][]map[string]any
	if err := json.Unmarshal([]byte(output[strings.Index(output, "{"):]), &labs); err != nil {
		t.Fatalf("invalid JSON %q: %v", output, err)
	}

	docker := labs["docker"][0]
	for _, field := range []string{
		"namespace", "pod", "container", "node_uid", "exposed_ports", "load_balancer_address",
	} {
		if _, ok := docker[field]; ok {
			t.Fatalf("docker record carries %q: %v", field, docker)
		}
	}
	if docker["container_id"] != "0123456789ab" {
		t.Fatalf("docker container_id = %v", docker["container_id"])
	}

	c9s := labs["c9s"][0]
	want := map[string]any{
		"container_id":          "c9s-c9s/c9s/r1",
		"namespace":             "c9s-c9s",
		"pod":                   "c9s-r1-abc",
		"container":             "node-r1-x",
		"node_uid":              "uid-1",
		"load_balancer_address": "192.0.2.10",
	}
	for field, value := range want {
		if c9s[field] != value {
			t.Fatalf("c9s %s = %v, want %v (%v)", field, c9s[field], value, c9s)
		}
	}
	ports, ok := c9s["exposed_ports"].([]any)
	if !ok || len(ports) != 1 {
		t.Fatalf("c9s exposed_ports = %v", c9s["exposed_ports"])
	}
	port := ports[0].(map[string]any)
	if port["port"] != float64(22) || port["expose_port"] != float64(22) || port["protocol"] != "tcp" {
		t.Fatalf("exposed port = %v", port)
	}
}

func TestLogsCommandFlags(t *testing.T) {
	o := &Options{Global: &GlobalOptions{}, Logs: &LogsOptions{Tail: -1}}

	c, err := logsCmd(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"node", "tail", "follow", "timestamps"} {
		if c.Flags().Lookup(flag) == nil {
			t.Fatalf("logs has no --%s flag", flag)
		}
	}
	if c.Flags().ShorthandLookup("n").Name != "node" ||
		c.Flags().ShorthandLookup("f").Name != "follow" {
		t.Fatal("logs shorthands -n/-f are missing")
	}

	if err := logsFn(c, o); err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("logs without a lab = %v", err)
	}
	o.Global.TopologyName = "lab1"
	if err := logsFn(c, o); err == nil || !strings.Contains(err.Error(), "--node") {
		t.Fatalf("logs without a node = %v", err)
	}
}
