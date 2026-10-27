package events

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	dockerEvents "github.com/docker/docker/api/types/events"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// These tests build the events the Docker runtime and the c9s lab runtime report for the same
// node and compare them: every attribute a Docker event carries is in the c9s event, with the
// same meaning, apart from the ones c9s has no equivalent for.

// dockerOnlyContainerAttributes are the Docker container event attributes c9s has no
// equivalent for: the Docker event scope, the Docker management bridge, and the exit code and
// run time of a container process, which Kubernetes does not keep once the pod is gone.
var dockerOnlyContainerAttributes = map[string]bool{ //nolint:gochecknoglobals
	"scope":                     true,
	clabconstants.NodeMgmtNetBr: true,
	clabconstants.NodeLabDir:    true,
	"exitCode":                  true,
	"execDuration":              true,
}

const parityTopology = `name: parity
mgmt:
  network: clab
topology:
  nodes:
    client:
      kind: linux
      image: ghcr.io/srl-labs/network-multitool
    srl1:
      kind: nokia_srlinux
      image: ghcr.io/nokia/srlinux:25.10
  links:
    - endpoints: ["client:eth1", "srl1:ethernet-1/1"]
`

// parityLab loads the topology as a Docker deploy does and returns its labels per node.
func parityLab(t *testing.T) (string, *clabcore.CLab) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "parity.clab.yml")
	if err := os.WriteFile(path, []byte(parityTopology), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USER", "alice")
	t.Setenv("SUDO_USER", "")

	c, err := clabcore.NewContainerLab(clabcore.WithTopoPath(path, nil))
	if err != nil {
		t.Fatal(err)
	}

	return path, c
}

// fakeDockerRuntime answers the container lookup the events command makes for a Docker event.
type fakeDockerRuntime struct {
	clabruntime.ContainerRuntime
	container clabruntime.GenericContainer
}

func (f *fakeDockerRuntime) ListContainers(
	context.Context,
	[]*clabtypes.GenericFilter,
) ([]clabruntime.GenericContainer, error) {
	return []clabruntime.GenericContainer{f.container}, nil
}

// dockerContainer is the container Docker runs for a node.
func dockerContainer(t *testing.T, c *clabcore.CLab, node string) clabruntime.GenericContainer {
	t.Helper()

	cfg := c.Nodes[node].Config()
	labels := cloneStringMap(cfg.Labels)
	labels[clabconstants.NodeMgmtNetBr] = "br-92bb9fb46cd2"

	return clabruntime.GenericContainer{
		Names:   []string{cfg.LongName},
		ID:      "57b7763f3b4d97ff0c96b1dab8294b836199628a2d9400549948a963732e185a",
		ShortID: "57b7763f3b4d",
		Image:   cfg.Image,
		State:   "running",
		Status:  "Up 19 seconds",
		Labels:  labels,
		NetworkSettings: clabruntime.GenericMgmtIPs{
			IPv4addr: "172.20.20.2", IPv4pLen: 24, IPv6addr: "3fff:172:20:20::2", IPv6pLen: 64,
		},
		NetworkName: "clab",
		Ports: []*clabtypes.GenericPortBinding{
			{ContainerPort: 22, Protocol: "tcp"}, {ContainerPort: 80, Protocol: "tcp"},
		},
	}
}

// dockerContainerEvent is a Docker container event as `containerlab events` reports it: the
// Docker message (container labels, image and name as attributes, plus the action's own) through
// the events command's conversion and enrichment.
func dockerContainerEvent(
	t *testing.T,
	container clabruntime.GenericContainer,
	action string,
	extra map[string]string,
) aggregatedEvent {
	t.Helper()

	attributes := cloneStringMap(container.Labels)
	attributes["image"] = container.Image
	attributes["name"] = container.Names[0]
	for k, v := range extra {
		attributes[k] = v
	}
	data := clabutils.DockerMessageToEventData(dockerEvents.Message{
		Type:     dockerEvents.ContainerEventType,
		Action:   dockerEvents.Action(action),
		Actor:    dockerEvents.Actor{ID: container.ID, Attributes: attributes},
		Scope:    "local",
		TimeNano: time.Now().UnixNano(),
	})

	return aggregatedEventFromContainerEvent(context.Background(),
		&fakeDockerRuntime{container: container},
		clabruntime.ContainerEvent{
			Timestamp: data.Timestamp, Type: data.Type, Action: data.Action,
			ActorID: data.ActorID, ActorName: data.ActorName, ActorFullID: data.ActorFullID,
			Attributes: data.Attributes,
		})
}

// c9sNode is the same node as the c9s runtime reports it, deployed from the same file.
func c9sNode(c *clabcore.CLab, path, node string) (*clablabruntime.LabState, clablabruntime.NodeState) {
	cfg := c.Nodes[node].Config()
	state := &clablabruntime.LabState{
		Name: "parity", Namespace: "c9s-parity", Owner: "alice", TopologyPath: path,
		State: "running", Ready: true,
	}
	nodeState := clablabruntime.NodeState{
		Name: node, Kind: cfg.Kind, Image: cfg.Image, State: "ready", Ready: true,
		ContainerState:  clablabruntime.ContainerStateRunning,
		Health:          clablabruntime.ContainerHealthHealthy,
		MgmtIPv4Address: "172.20.20.84/24", MgmtIPv6Address: "3fff:172:20:20::84/64",
		Pod: node + "-88598986c-rbdfq", Container: "node-" + node, NodeUID: "uid-" + node,
		ExposedPorts: []clablabruntime.ExposedPort{
			{Port: 22, ExposePort: 22, Protocol: "tcp"}, {Port: 80, ExposePort: 80, Protocol: "tcp"},
		},
	}
	state.Nodes = []clablabruntime.NodeState{nodeState}

	return state, nodeState
}

// c9sContainerEvent is the event the c9s runtime emits, as `containerlab events` reports it.
func c9sContainerEvent(
	clab *clabcore.CLab,
	state *clablabruntime.LabState,
	node clablabruntime.NodeState,
	action, containerState string,
	extra map[string]string,
) aggregatedEvent {
	key := state.Namespace + "/" + state.Name + "/" + node.Name
	attributes := map[string]string{
		"namespace": state.Namespace, "lab": state.Name, "node": node.Name, "pod": node.Pod,
		"state": containerState,
	}
	for k, v := range extra {
		attributes[k] = v
	}

	return aggregatedEventFromLabRuntimeEvent(clab, clablabruntime.Event{
		Type: clablabruntime.EventTypeContainer, Action: action,
		ActorID: key, ActorName: state.Name + "-" + node.Name, ActorFullID: key,
		Attributes: attributes, Lab: state, Node: &node,
	})
}

func attributeKeys(attributes map[string]string) []string {
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}

// assertAttributeParity checks that the c9s event carries every attribute of the Docker event,
// apart from the ones listed.
func assertAttributeParity(t *testing.T, docker, c9s aggregatedEvent, dockerOnly map[string]bool) {
	t.Helper()

	dockerAttributes := mergedEventAttributes(docker)
	c9sAttributes := mergedEventAttributes(c9s)
	var missing []string
	for key := range dockerAttributes {
		if _, ok := c9sAttributes[key]; !ok && !dockerOnly[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		t.Fatalf("c9s %s %s lacks %v\ndocker: %v\nc9s:    %v", c9s.Type, c9s.Action, missing,
			attributeKeys(dockerAttributes), attributeKeys(c9sAttributes))
	}
}

// TestContainerEventParity compares the container events of a node's lifecycle, with the c9s
// lab read with its topology file (events -t) and without it (events --name), where the
// attributes come from the cluster alone.
func TestContainerEventParity(t *testing.T) {
	path, dockerLab := parityLab(t)
	byName, err := clabcore.NewContainerLab(clabcore.WithLabNameOnly("parity"))
	if err != nil {
		t.Fatal(err)
	}

	for _, mode := range []struct {
		name string
		lab  *clabcore.CLab
	}{{"with the topology", dockerLab}, {"by name", byName}} {
		t.Run(mode.name, func(t *testing.T) {
			assertContainerEventParity(t, path, dockerLab, mode.lab, mode.lab == byName)
		})
	}
}

func assertContainerEventParity(
	t *testing.T,
	path string,
	dockerLab, c9sLab *clabcore.CLab,
	byName bool,
) {
	t.Helper()

	for _, node := range []string{"client", "srl1"} {
		container := dockerContainer(t, dockerLab, node)
		state, nodeState := c9sNode(dockerLab, path, node)
		dockerOnly := dockerOnlyContainerAttributes
		if byName && node == "srl1" {
			// The type is the SR Linux default; the cluster has only the types a topology
			// sets, so a lab read by name does not know it.
			dockerOnly = map[string]bool{clabconstants.NodeType: true}
			for key := range dockerOnlyContainerAttributes {
				dockerOnly[key] = true
			}
		}

		for _, tt := range []struct {
			action, state string
			extra         map[string]string
		}{
			{"create", "created", nil},
			{"start", "running", nil},
			{"kill", "running", map[string]string{"signal": "15"}},
			{"stop", "exited", nil},
			{"die", "exited", map[string]string{"exitCode": "137", "execDuration": "3"}},
			{"health_status: healthy", "running", nil},
			{"destroy", "exited", nil},
		} {
			docker := dockerContainerEvent(t, container, tt.action, tt.extra)
			var c9sExtra map[string]string
			if tt.action == "kill" {
				c9sExtra = tt.extra
			}
			c9s := c9sContainerEvent(c9sLab, state, nodeState, tt.action, tt.state, c9sExtra)
			assertAttributeParity(t, docker, c9s, dockerOnly)

			got := mergedEventAttributes(c9s)
			want := map[string]string{
				clabconstants.Containerlab: "parity",
				clabconstants.NodeName:     node,
				clabconstants.LongName:     "parity-" + node,
				clabconstants.NodeKind:     dockerLab.Nodes[node].Config().Kind,
				clabconstants.TopoFile:     path,
				clabconstants.Owner:        "alice",
				"image":                    container.Image,
				"name":                     "parity-" + node,
				"id":                       "c9s-parity/parity/" + node,
				"mgmt_ipv4":                "172.20.20.84/24",
				"mgmt_ipv6":                "3fff:172:20:20::84/64",
				"ports":                    "22/tcp;80/tcp",
				"state":                    tt.state,
			}
			for key, value := range want {
				if got[key] != value {
					t.Fatalf("%s %s: %s = %q, want %q", node, tt.action, key, got[key], value)
				}
			}
			// the label values have Docker's format
			dockerAttributes := mergedEventAttributes(docker)
			for _, key := range []string{
				clabconstants.Containerlab, clabconstants.NodeName, clabconstants.NodeKind,
				clabconstants.NodeType, clabconstants.TopoFile, clabconstants.Owner, "image",
				"ports",
			} {
				if dockerOnly[key] {
					continue
				}
				if got[key] != dockerAttributes[key] {
					t.Fatalf("%s %s: %s = %q, docker %q", node, tt.action, key, got[key],
						dockerAttributes[key])
				}
			}
			if c9s.ActorID != c9s.ActorFullID || c9s.ActorName != "parity-"+node {
				t.Fatalf("%s %s: actor = %+v", node, tt.action, c9s)
			}
		}
	}
}

// TestContainerSnapshotParity compares the --initial-state events of a running node, with the
// c9s lab read with its topology file.
func TestContainerSnapshotParity(t *testing.T) {
	path, dockerLab := parityLab(t)
	c9sLab := dockerLab

	for _, node := range []string{"client", "srl1"} {
		container := dockerContainer(t, dockerLab, node)
		docker := aggregatedEventFromContainerSnapshot(&container)

		state, nodeState := c9sNode(dockerLab, path, node)
		c9s := c9sContainerEvent(c9sLab, state, nodeState, "running", "running",
			map[string]string{"origin": clablabruntime.EventOriginSnapshot})

		assertAttributeParity(t, docker, c9s, dockerOnlyContainerAttributes)
		got := mergedEventAttributes(c9s)
		if docker.Action != c9s.Action || got["origin"] != "snapshot" || got["state"] != "running" ||
			got["status"] != "healthy" || got["network"] != "c9s-parity" {
			t.Fatalf("%s snapshot = %s %v", node, c9s.Action, got)
		}
	}
}

// TestContainerStateUsesDockerVocabulary checks the state and status of every node state c9s
// reports, in events and inspect.
func TestContainerStateUsesDockerVocabulary(t *testing.T) {
	t.Parallel()

	c9sLab, err := clabcore.NewContainerLab(clabcore.WithLabNameOnly("parity"))
	if err != nil {
		t.Fatal(err)
	}

	state := &clablabruntime.LabState{Name: "parity", Namespace: "c9s-parity"}
	tests := []struct {
		node          clablabruntime.NodeState
		state, status string
	}{
		{clablabruntime.NodeState{Name: "a", ContainerState: "created", State: "notready"},
			"created", "Created"},
		{clablabruntime.NodeState{Name: "b", ContainerState: "running", Health: "starting",
			State: "notready"}, "running", "health: starting"},
		{clablabruntime.NodeState{Name: "c", ContainerState: "running", Health: "healthy",
			State: "ready", Ready: true}, "running", "healthy"},
		{clablabruntime.NodeState{Name: "d", ContainerState: "exited", State: "stopped"},
			"exited", "Exited"},
	}
	for _, tt := range tests {
		container := c9sLab.LabRuntimeContainer(state, tt.node)
		if container.State != tt.state || container.Status != tt.status {
			t.Fatalf("%s: %s / %s, want %s / %s", tt.node.Name, container.State, container.Status,
				tt.state, tt.status)
		}
	}
}

// dockerInterfaceEvent is a netlink interface event of a Docker container.
func dockerInterfaceEvent(
	container clabruntime.GenericContainer,
	action string,
	snapshot ifaceSnapshot,
	stats *ifaceStatsMetrics,
) aggregatedEvent {
	attributes := interfaceAttributes(&container, snapshot)
	if stats != nil {
		interfaceStatsAttributes(attributes, *stats)
	}

	return aggregatedEvent{
		Type: "interface", Action: action, ActorID: container.ShortID,
		ActorName: container.Names[0], ActorFullID: container.ID, Attributes: attributes,
	}
}

func c9sInterfaceEvent(
	state *clablabruntime.LabState,
	node clablabruntime.NodeState,
	action string,
	iface clablabruntime.InterfaceEventData,
) aggregatedEvent {
	key := state.Namespace + "/" + state.Name + "/" + node.Name

	return aggregatedEventFromLabRuntimeEvent(nil, clablabruntime.Event{
		Type: clablabruntime.EventTypeInterface, Action: action,
		ActorID: key, ActorName: state.Name + "-" + node.Name, ActorFullID: key,
		Attributes: map[string]string{
			"namespace": state.Namespace, "lab": state.Name, "node": node.Name, "pod": node.Pod,
			"origin": "clabernetes",
		},
		Lab: state, Node: &node, Interface: &iface,
	})
}

// TestInterfaceEventParity compares the interface events of the lab link (linux eth1, SR Linux
// e1-1 with its alias ethernet-1/1), the management interface and loopback.
func TestInterfaceEventParity(t *testing.T) {
	path, dockerLab := parityLab(t)

	tests := []struct {
		node string
		data clablabruntime.InterfaceEventData
	}{
		{"client", clablabruntime.InterfaceEventData{
			Name: "eth1", Index: 7, MTU: 9500, MAC: "aa:c1:ab:56:aa:46", OperState: "up", Type: "veth",
		}},
		{"srl1", clablabruntime.InterfaceEventData{
			Name: "e1-1", Alias: "ethernet-1/1", Index: 7, MTU: 9500, MAC: "aa:c1:ab:56:aa:47",
			OperState: "lower-layer-down", Type: "veth",
		}},
		{"client", clablabruntime.InterfaceEventData{
			Name: "eth0", Index: 4, MTU: 1500, MAC: "02:42:ac:14:14:02", OperState: "up", Type: "veth",
		}},
		{"client", clablabruntime.InterfaceEventData{
			Name: "lo", Index: 1, MTU: 65536, OperState: "unknown", Type: "device",
		}},
	}
	for _, tt := range tests {
		container := dockerContainer(t, dockerLab, tt.node)
		state, nodeState := c9sNode(dockerLab, path, tt.node)
		snapshot := ifaceSnapshot{
			Index: tt.data.Index, Name: tt.data.Name, Alias: tt.data.Alias, MTU: tt.data.MTU,
			MAC: tt.data.MAC, OperState: tt.data.OperState, Type: tt.data.Type,
		}
		metrics := ifaceStatsMetrics{
			RxBytes: 2000, TxBytes: 3000, RxPackets: 20, TxPackets: 30,
			RxBps: 8000, TxBps: 12000, RxPps: 10, TxPps: 15, Interval: 2 * time.Second,
		}

		for _, action := range []string{"snapshot", "create", "update", "delete", "stats"} {
			var stats *ifaceStatsMetrics
			data := tt.data
			if action == "stats" {
				stats = &metrics
				data.Stats = &clablabruntime.InterfaceStats{
					RxBytes: 2000, TxBytes: 3000, RxPackets: 20, TxPackets: 30,
					RxBps: 8000, TxBps: 12000, RxPps: 10, TxPps: 15, Interval: 2 * time.Second,
				}
			}
			docker := dockerInterfaceEvent(container, action, snapshot, stats)
			c9s := c9sInterfaceEvent(state, nodeState, action, data)
			assertAttributeParity(t, docker, c9s, nil)

			dockerAttributes := mergedEventAttributes(docker)
			got := mergedEventAttributes(c9s)
			for key, value := range dockerAttributes {
				switch key {
				case "id", "name", "lab", "origin":
					// the node's own ID, name and lab; the source of the event
					continue
				}
				if got[key] != value {
					t.Fatalf("%s %s %s: %s = %q, docker %q", tt.node, tt.data.Name, action, key,
						got[key], value)
				}
			}
			if got["name"] != "parity-"+tt.node || got["lab"] != "parity" ||
				got["id"] != "c9s-parity/parity/"+tt.node {
				t.Fatalf("%s %s %s: node attributes %v", tt.node, tt.data.Name, action, got)
			}
		}
	}
}

// TestInterfaceEventOmitsUnknownValues checks that values the runtime could not read are left
// out: an index of 0 would make clients take two interfaces for one.
func TestInterfaceEventOmitsUnknownValues(t *testing.T) {
	t.Parallel()

	state := &clablabruntime.LabState{Name: "parity", Namespace: "c9s-parity"}
	ev := c9sInterfaceEvent(state, clablabruntime.NodeState{Name: "client"}, "stats",
		clablabruntime.InterfaceEventData{
			Name: "eth1", Type: "veth",
			Stats: &clablabruntime.InterfaceStats{RxBytes: 1, Interval: time.Second},
		})
	got := mergedEventAttributes(ev)
	for _, key := range []string{"index", "mtu", "state", "mac", "alias"} {
		if _, ok := got[key]; ok {
			t.Fatalf("%s reported without a value: %v", key, got)
		}
	}
	if got["ifname"] != "eth1" || got["rx_bytes"] != "1" || got["interval_seconds"] != "1" {
		t.Fatalf("stats = %v", got)
	}
}

func TestLabRuntimeTopologyEventsKeepTheirAttributes(t *testing.T) {
	t.Parallel()

	ev := aggregatedEventFromLabRuntimeEvent(nil, clablabruntime.Event{
		Type: "topology", Action: "modified", ActorID: "c9s-parity/parity", ActorName: "parity",
		Attributes: map[string]string{"lab": "parity", "state": "running"},
	})
	if !strings.EqualFold(ev.Attributes["lab"], "parity") || len(ev.Attributes) != 2 {
		t.Fatalf("topology event = %+v", ev)
	}
}
