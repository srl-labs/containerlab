package clabernetes

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

func captureTestNode(name, original, container string) *unstructured.Unstructured {
	labels := map[string]any{labelTopologyOwner: "lab1"}
	if original != "" {
		labels[labelOriginalNodeName] = original
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Node",
		"metadata": map[string]any{
			"name": name, "namespace": "lab-ns", "uid": "uid-" + name, "labels": labels,
		},
		"spec": map[string]any{"kind": "srl", "runState": nodeRunStateRunning},
		"status": map[string]any{
			"readiness":        "ready",
			"directContainers": []any{map[string]any{"name": container, "id": name}},
		},
	}}
}

func captureTestPod(name, node string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "lab-ns",
			Labels: map[string]string{
				labelApp: clabernetesAppValue, labelTopologyOwner: "lab1", labelTopologyNode: node,
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

// captureListing is the --list-interfaces output of each pod.
var captureListing = map[string][]clabwireInterface{ //nolint:gochecknoglobals
	"lab1-r1-abc": {
		{
			Node:          "r1",
			Interface:     "e1-1",
			Alias:         "ethernet-1/1",
			PeerNode:      "r2",
			PeerInterface: "ethernet-1/1",
		},
		{Node: "r1", Interface: "e1-2", PeerNode: "host", PeerInterface: "r1-e1-2"},
	},
	"lab1-r2-def": {
		{
			Node:          "r2",
			Interface:     "e1-1",
			Alias:         "ethernet-1/1",
			PeerNode:      "r1",
			PeerInterface: "ethernet-1/1",
		},
	},
}

// fakeClabwire plays clabwire packet-capture and --list-interfaces in the fake pod exec. Every
// capture process sees the same traffic: one packet per tick on each requested interface,
// carrying the tick number, stamped with the tick time plus a per-process jitter (clabwire stamps
// packets when it reads them).
type fakeClabwire struct {
	t     *testing.T
	epoch time.Time
	tick  time.Duration

	mu       sync.Mutex
	commands [][]string
	// processes counts the capture processes started per node
	processes map[string]int

	// startDelay delays the Started record of the given process index (all nodes).
	startDelay map[int]time.Duration
	// firstStartDelay delays the Started record of a node's first process.
	firstStartDelay map[string]time.Duration
	// fail makes a process write this audit status and exit 1.
	fail string
	// stderrOnly makes a process write this text to stderr and exit 1, like a clabwire that
	// rejects a flag.
	stderrOnly string
	// stampSkew stamps the packets of a node this much earlier, as if its stream reached
	// containerlab this much later than the others.
	stampSkew map[string]time.Duration
	// stdinClosed counts the processes that ended because their stdin closed, per node.
	stdinClosed map[string]int
	// sysfs is the device container's interface sample.
	sysfs string
}

func newFakeClabwire(t *testing.T, tick time.Duration) *fakeClabwire {
	t.Helper()

	return &fakeClabwire{
		t: t, epoch: time.Now(), tick: tick, processes: map[string]int{},
		startDelay: map[int]time.Duration{}, stampSkew: map[string]time.Duration{},
		firstStartDelay: map[string]time.Duration{},
		stdinClosed:     map[string]int{},
	}
}

func (f *fakeClabwire) exec(
	ctx context.Context,
	namespace, pod, container string,
	command []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) error {
	f.mu.Lock()
	f.commands = append(
		f.commands,
		append([]string{namespace + "/" + pod + "/" + container}, command...),
	)
	f.mu.Unlock()

	if container != clabwireContainer {
		_, _ = io.WriteString(stdout, f.sysfs)
		return nil
	}
	if slices.Contains(command, "--list-interfaces") {
		return json.NewEncoder(stdout).Encode(captureListing[pod])
	}

	return f.capture(ctx, command, stdin, stdout, stderr)
}

func flagValues(command []string, name string) []string {
	var values []string
	for idx := 0; idx+1 < len(command); idx++ {
		if command[idx] == name {
			values = append(values, command[idx+1])
		}
	}

	return values
}

func (f *fakeClabwire) audit(
	stderr io.Writer,
	status, node string,
	interfaces []string,
	reason string,
) {
	record := map[string]any{
		"schemaVersion": captureAuditSchema, "time": time.Now().UTC(), "operation": "PacketCapture",
		"status": status, "nodeName": node, "format": "pcapng", "reason": reason,
	}
	entries := []map[string]any{}
	for _, name := range interfaces {
		entries = append(entries, map[string]any{"name": name})
	}
	record["interfaces"] = entries
	data, _ := json.Marshal(record)
	_, _ = stderr.Write(append(data, '\n'))
}

func (f *fakeClabwire) capture(
	ctx context.Context,
	command []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) error {
	f.mu.Lock()
	rejected := f.stderrOnly
	f.mu.Unlock()
	if rejected != "" {
		_, _ = io.WriteString(stdout, "Incorrect Usage: "+rejected+"\n\nNAME:\n   manager\n")
		_, _ = io.WriteString(stderr, rejected+"\n")
		return &clablabruntime.CommandExitError{Code: 1}
	}

	// clabwire ends the capture when stdin closes (--stop-on-stdin-close)
	if !slices.Contains(command, clabwireStopOnStdinClose) || stdin == nil {
		f.t.Errorf("capture without stdin and %s: %v", clabwireStopOnStdinClose, command)
		return fmt.Errorf("no stdin")
	}
	stdinClosed := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, stdin)
		close(stdinClosed)
	}()

	node := flagValues(command, "--nodeName")[0]
	interfaces := flagValues(command, "--interface")
	duration, err := time.ParseDuration(flagValues(command, "--duration")[0])
	if err != nil {
		f.t.Errorf("duration: %v", err)
	}
	if !slices.Equal(flagValues(command, "--format"), []string{"pcapng"}) {
		f.t.Errorf("capture without --format pcapng: %v", command)
	}

	f.mu.Lock()
	index := f.processes[node]
	f.processes[node]++
	delay := f.startDelay[index]
	if index == 0 {
		delay += f.firstStartDelay[node]
	}
	failure := f.fail
	skew := f.stampSkew[node]
	f.mu.Unlock()

	if failure != "" {
		f.audit(stderr, failure, node, interfaces, "interface \"eth9\" is not uniquely planned")
		return &clablabruntime.CommandExitError{Code: 1}
	}

	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return ctx.Err()
	case <-stdinClosed:
		f.audit(stderr, captureAuditStarted, node, interfaces, "")
		f.audit(stderr, captureAuditSucceeded, node, interfaces, "client closed: stdin reached EOF")
		return nil
	}

	writer := newPCAPNGWriter(stdout)
	f.audit(stderr, captureAuditStarted, node, interfaces, "")
	if err := writer.WriteHeader(interfaces, DefaultCaptureSnapLength); err != nil {
		return err
	}

	// clabwire's own section header names it
	end := time.Now().Add(duration)
	jitter := time.Duration(index%3) * 7 * time.Microsecond
	next := time.Now().Sub(f.epoch)/f.tick + 1
	for {
		at := f.epoch.Add(next * f.tick)
		if at.After(end) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-stdinClosed:
			f.mu.Lock()
			f.stdinClosed[node]++
			f.mu.Unlock()
			f.audit(
				stderr,
				captureAuditSucceeded,
				node,
				interfaces,
				"client closed: stdin reached EOF",
			)
			return nil
		case <-time.After(time.Until(at)):
		}
		for id := range interfaces {
			data := make([]byte, 64)
			binary.BigEndian.PutUint64(data[0:], uint64(next))
			copy(data[8:], node+":"+interfaces[id])
			if err := writer.WritePacket(uint32(id), &pcapngPacket{ //nolint:gosec // small
				Timestamp: at.Add(jitter - skew).UnixNano(), OriginalLength: 64, Data: data,
			}); err != nil {
				return err
			}
		}
		next++
	}
	f.audit(stderr, captureAuditSucceeded, node, interfaces, "")

	return nil
}

func (f *fakeClabwire) captureCommands() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()

	var result [][]string
	for _, command := range f.commands {
		if slices.Contains(command, "packet-capture") &&
			!slices.Contains(command, "--list-interfaces") {
			result = append(result, command)
		}
	}

	return result
}

// captureTestRuntime is lab1 in lab-ns with nodes r1 (topology name R1) and r2, each in its
// own pod, and a fake clabwire.
func captureTestRuntime(t *testing.T, tick time.Duration) (*Runtime, *fakeClabwire) {
	t.Helper()

	r := newTestRuntimeWithKubeObjects(
		[]*unstructured.Unstructured{
			captureTestNode("r1", "R1", "node-r1-x"),
			captureTestNode("r2", "", "node-r2-x"),
		},
		[]k8sruntime.Object{
			captureTestPod("lab1-r1-abc", "r1"),
			captureTestPod("lab1-r2-def", "r2"),
		},
	)
	r.labNamespaceOverride = "lab-ns"
	clabwire := newFakeClabwire(t, tick)
	r.podExec = clabwire.exec

	return r, clabwire
}

// parsedCapture is a pcapng stream read back.
type parsedCapture struct {
	sections   int
	interfaces []string
	packets    []pcapngPacket
}

func parseCapture(t *testing.T, data []byte) parsedCapture {
	t.Helper()

	reader := newPCAPNGReader(bytes.NewReader(data))
	var parsed parsedCapture
	for {
		event, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("invalid capture: %v", err)
		}
		if event.Interface != nil {
			if len(parsed.packets) != 0 {
				t.Fatal("interface description after the first packet")
			}
			if event.Interface.TSResol != pcapngNanoseconds {
				t.Fatalf("interface %s tsresol = %d", event.Interface.Name, event.Interface.TSResol)
			}
			parsed.interfaces = append(parsed.interfaces, event.Interface.Name)
		}
		if event.Packet != nil {
			parsed.packets = append(parsed.packets, *event.Packet)
		}
	}
	parsed.sections = reader.sections

	return parsed
}

func packetSequence(packet pcapngPacket) uint64 {
	return binary.BigEndian.Uint64(packet.Data[0:8])
}

func packetOrigin(packet pcapngPacket) string {
	return strings.TrimRight(string(packet.Data[8:]), "\x00")
}

func TestCaptureInterfacesUsesTopologyNames(t *testing.T) {
	t.Parallel()

	r, _ := captureTestRuntime(t, time.Millisecond)
	interfaces, err := r.CaptureInterfaces(
		context.Background(),
		clablabruntime.NodeRequest{Name: "lab1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []clablabruntime.CaptureInterface{
		{
			Node:          "R1",
			Interface:     "e1-1",
			Alias:         "ethernet-1/1",
			PeerNode:      "r2",
			PeerInterface: "ethernet-1/1",
		},
		{Node: "R1", Interface: "e1-2", PeerNode: "host", PeerInterface: "r1-e1-2"},
		{
			Node:          "r2",
			Interface:     "e1-1",
			Alias:         "ethernet-1/1",
			PeerNode:      "R1",
			PeerInterface: "ethernet-1/1",
		},
	}
	if !slices.Equal(interfaces, want) {
		t.Fatalf("interfaces = %+v\nwant %+v", interfaces, want)
	}

	one, err := r.CaptureInterfaces(context.Background(), clablabruntime.NodeRequest{
		Name: "lab1", Nodes: []string{"r2"},
	})
	if err != nil || len(one) != 1 || one[0].Node != "r2" {
		t.Fatalf("r2 interfaces = %+v, %v", one, err)
	}
}

func TestCaptureMergesNodesIntoOneSection(t *testing.T) {
	t.Parallel()

	r, clabwire := captureTestRuntime(t, 10*time.Millisecond)
	var out bytes.Buffer
	err := r.Capture(context.Background(), clablabruntime.CaptureRequest{
		Name: "lab1",
		// the topology name of r1, an alias, and a second node between two r1 interfaces
		Targets: []clablabruntime.CaptureTarget{
			{Node: "R1", Interface: "ethernet-1/1"},
			{Node: "r2", Interface: "e1-1"},
			{Node: "r1", Interface: "e1-2"},
		},
		Duration: 300 * time.Millisecond,
	}, &out)
	if err != nil {
		t.Fatal(err)
	}

	parsed := parseCapture(t, out.Bytes())
	if parsed.sections != 1 {
		t.Fatalf("sections = %d, want one merged section", parsed.sections)
	}
	wantNames := []string{"R1:e1-1", "r2:e1-1", "R1:e1-2"}
	if !slices.Equal(parsed.interfaces, wantNames) {
		t.Fatalf("interfaces = %v, want %v", parsed.interfaces, wantNames)
	}

	// one clabwire process per node, with all of the node's interfaces in request order
	commands := clabwire.captureCommands()
	if len(commands) != 2 {
		t.Fatalf("capture processes = %d, want one per node: %v", len(commands), commands)
	}
	for _, command := range commands {
		node := flagValues(command, "--nodeName")[0]
		want := map[string][]string{"r1": {"e1-1", "e1-2"}, "r2": {"e1-1"}}[node]
		if got := flagValues(command, "--interface"); !slices.Equal(got, want) {
			t.Fatalf("%s interfaces = %v, want %v", node, got, want)
		}
		if !strings.HasSuffix(command[0], "/"+clabwireContainer) {
			t.Fatalf("capture ran in %s, want the clabwire container", command[0])
		}
	}

	// every packet carries the merged ID of its interface
	wantOrigin := map[uint32]string{0: "r1:e1-1", 1: "r2:e1-1", 2: "r1:e1-2"}
	perInterface := map[uint32]int{}
	for _, packet := range parsed.packets {
		if packetOrigin(packet) != wantOrigin[packet.InterfaceID] {
			t.Fatalf("packet from %s has interface ID %d", packetOrigin(packet), packet.InterfaceID)
		}
		perInterface[packet.InterfaceID]++
	}
	for id := range uint32(3) {
		if perInterface[id] == 0 {
			t.Fatalf("no packets on %s: %v", wantNames[id], perInterface)
		}
	}
}

// TestCaptureChainsSegmentsWithoutDuplicatesOrGaps runs captures longer than one segment: every
// tick must appear exactly once per interface, across every switch.
func TestCaptureChainsSegmentsWithoutDuplicatesOrGaps(t *testing.T) {
	t.Parallel()

	r, clabwire := captureTestRuntime(t, 4*time.Millisecond)
	r.captureSegment = 300 * time.Millisecond

	var gaps []clablabruntime.CaptureGap
	var out bytes.Buffer
	err := r.Capture(context.Background(), clablabruntime.CaptureRequest{
		Name: "lab1",
		Targets: []clablabruntime.CaptureTarget{
			{Node: "r1", Interface: "e1-1"},
			{Node: "r1", Interface: "e1-2"},
		},
		Duration: 1300 * time.Millisecond,
		OnGap:    func(gap clablabruntime.CaptureGap) { gaps = append(gaps, gap) },
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Fatalf("gaps = %+v", gaps)
	}
	if processes := len(clabwire.captureCommands()); processes < 4 {
		t.Fatalf("capture processes = %d, want a chain of at least 4 segments", processes)
	}

	parsed := parseCapture(t, out.Bytes())
	if parsed.sections != 1 || len(parsed.interfaces) != 2 {
		t.Fatalf("sections = %d, interfaces = %v; want the follow-up headers dropped",
			parsed.sections, parsed.interfaces)
	}
	for id := range uint32(2) {
		var sequence []uint64
		for _, packet := range parsed.packets {
			if packet.InterfaceID == id {
				sequence = append(sequence, packetSequence(packet))
			}
		}
		if len(sequence) < 200 {
			t.Fatalf("interface %d has %d packets", id, len(sequence))
		}
		slices.Sort(sequence)
		for idx := 1; idx < len(sequence); idx++ {
			switch {
			case sequence[idx] == sequence[idx-1]:
				t.Fatalf("interface %d: tick %d written twice", id, sequence[idx])
			case sequence[idx] != sequence[idx-1]+1:
				t.Fatalf(
					"interface %d: ticks %d to %d missing",
					id,
					sequence[idx-1]+1,
					sequence[idx]-1,
				)
			}
		}
	}
}

func TestCaptureReportsAGapWhenTheFollowUpStartsLate(t *testing.T) {
	t.Parallel()

	r, clabwire := captureTestRuntime(t, 5*time.Millisecond)
	r.captureSegment = 300 * time.Millisecond
	// the second process starts long after the first one ended
	clabwire.startDelay[1] = 400 * time.Millisecond

	var gaps []clablabruntime.CaptureGap
	var out bytes.Buffer
	err := r.Capture(context.Background(), clablabruntime.CaptureRequest{
		Name:     "lab1",
		Targets:  []clablabruntime.CaptureTarget{{Node: "r2", Interface: "e1-1"}},
		Duration: 800 * time.Millisecond,
		OnGap:    func(gap clablabruntime.CaptureGap) { gaps = append(gaps, gap) },
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) == 0 || gaps[0].Node != "r2" ||
		!slices.Equal(gaps[0].Interfaces, []string{"e1-1"}) ||
		gaps[0].From.IsZero() ||
		gaps[0].Reason == "" {
		t.Fatalf("gaps = %+v, want the late follow-up reported", gaps)
	}
	if !gaps[0].To.IsZero() && !gaps[0].To.After(gaps[0].From) {
		t.Fatalf("gap ends before it starts: %+v", gaps[0])
	}
	parseCapture(t, out.Bytes())
}

func TestCaptureMapsDeniedAndFailedAuditsToErrors(t *testing.T) {
	t.Parallel()

	for _, status := range []string{captureAuditDenied, captureAuditFailed} {
		r, clabwire := captureTestRuntime(t, time.Millisecond)
		clabwire.fail = status

		var out bytes.Buffer
		err := r.Capture(context.Background(), clablabruntime.CaptureRequest{
			Name:     "lab1",
			Targets:  []clablabruntime.CaptureTarget{{Node: "r2", Interface: "e1-1"}},
			Duration: time.Second,
		}, &out)
		if err == nil || !strings.Contains(err.Error(), "is not uniquely planned") ||
			!strings.Contains(err.Error(), "node r2") {
			t.Fatalf("%s: error = %v, want the audit reason", status, err)
		}
		if status == captureAuditDenied && !strings.Contains(err.Error(), "was denied") {
			t.Fatalf("Denied error = %v", err)
		}
		// the header is written before the capture starts, so the output stays a valid file
		parseCapture(t, out.Bytes())
	}
}

func TestCaptureAuditLineWriter(t *testing.T) {
	t.Parallel()

	var audits []*captureAudit
	writer := &auditLineWriter{send: func(audit *captureAudit) { audits = append(audits, audit) }}
	started := fmt.Sprintf(`{"schemaVersion":%q,"time":"2026-10-27T10:00:00.123456789Z",`+
		`"status":"Started","nodeName":"r1","interfaces":[{"name":"e1-1","id":"if1"}]}`,
		captureAuditSchema)
	// lines arrive split across writes
	_, _ = writer.Write([]byte(started[:20]))
	_, _ = writer.Write([]byte(started[20:] + "\nnot json\n{\"status\":\"Started\"}\n"))
	_, _ = writer.Write(
		[]byte(`{"schemaVersion":"` + captureAuditSchema + `","status":"Succeeded","packets":3}`),
	)
	writer.flush()

	if len(audits) != 2 || audits[0].Status != captureAuditStarted || audits[1].Packets != 3 ||
		audits[0].Time.Nanosecond() != 123456789 || audits[0].Interfaces[0].Name != "e1-1" {
		t.Fatalf("audits = %+v", audits)
	}
	if writer.text() != `not json; {"status":"Started"}` {
		t.Fatalf("other stderr = %q", writer.text())
	}

	err := captureAuditError("r1", &captureAudit{
		Status: captureAuditDenied, Reason: "nope",
		Interfaces: []struct {
			Name    string `json:"name"`
			ID      string `json:"id"`
			Packets int    `json:"packets"`
		}{{Name: "e1-1"}, {Name: "eth0"}},
	})
	if err.Error() != "capture of e1-1,eth0 on node r1 was denied by clabwire: nope" {
		t.Fatalf("audit error = %q", err)
	}
}

func TestCaptureStopsAtThePacketLimit(t *testing.T) {
	t.Parallel()

	r, clabwire := captureTestRuntime(t, 2*time.Millisecond)
	var out bytes.Buffer
	err := r.Capture(context.Background(), clablabruntime.CaptureRequest{
		Name: "lab1",
		Targets: []clablabruntime.CaptureTarget{
			{Node: "r1", Interface: "e1-1"},
			{Node: "r2", Interface: "e1-1"},
		},
		PacketLimit: 7,
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(parseCapture(t, out.Bytes()).packets); got != 7 {
		t.Fatalf("packets = %d, want 7", got)
	}
	// each clabwire process gets the limit that remained when it started
	for _, command := range clabwire.captureCommands() {
		limit := flagValues(command, "--packetLimit")
		if len(limit) != 1 || limit[0] < "1" || limit[0] > "7" || len(limit[0]) != 1 {
			t.Fatalf("clabwire packet limit = %v, want at most 7", limit)
		}
	}
}

func TestCaptureEndsCleanlyWhenTheContextEnds(t *testing.T) {
	t.Parallel()

	r, _ := captureTestRuntime(t, 2*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	var out bytes.Buffer
	if err := r.Capture(ctx, clablabruntime.CaptureRequest{
		Name:    "lab1",
		Targets: []clablabruntime.CaptureTarget{{Node: "r1", Interface: "e1-1"}},
	}, &out); err != nil {
		t.Fatalf("Capture() after the context ended = %v, want nil", err)
	}
	if len(parseCapture(t, out.Bytes()).packets) == 0 {
		t.Fatal("no packets before the context ended")
	}
}

func TestCaptureValidatesTargets(t *testing.T) {
	t.Parallel()

	r, clabwire := captureTestRuntime(t, time.Millisecond)
	ctx := context.Background()
	cases := map[string]clablabruntime.CaptureRequest{
		"is not a capturable link interface; capturable: e1-1 (ethernet-1/1), e1-2": {
			Targets: []clablabruntime.CaptureTarget{{Node: "r1", Interface: "eth0"}},
		},
		"requested more than once": {
			Targets: []clablabruntime.CaptureTarget{
				{Node: "r1", Interface: "e1-1"}, {Node: "R1", Interface: "ethernet-1/1"},
			},
		},
		`node "missing" was not found`: {
			Targets: []clablabruntime.CaptureTarget{{Node: "missing", Interface: "e1-1"}},
		},
		"at least one capture target": {},
		"snap length 10 is outside": {
			Targets:    []clablabruntime.CaptureTarget{{Node: "r1", Interface: "e1-1"}},
			SnapLength: 10,
		},
		"duration must not be negative": {
			Targets:  []clablabruntime.CaptureTarget{{Node: "r1", Interface: "e1-1"}},
			Duration: -time.Second,
		},
	}
	for want, req := range cases {
		req.Name = "lab1"
		err := r.Capture(ctx, req, io.Discard)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want %q", err, want)
		}
	}
	if commands := clabwire.captureCommands(); len(commands) != 0 {
		t.Fatalf("invalid requests started captures: %v", commands)
	}
}

func TestInspectInterfacesReportsStateFromSysfs(t *testing.T) {
	t.Parallel()

	r, clabwire := captureTestRuntime(t, time.Millisecond)
	clabwire.sysfs = "Inter-|   Receive\n face |bytes\n" + interfaceSysfsMarker + "\n" +
		"e1-1\t12\t9500\taa:c1:ab:00:00:01\tup\t0x1003\t\n" +
		"e1-2\t13\t1500\taa:c1:ab:00:00:02\tlowerlayerdown\t0x1003\t\n"

	interfaces, err := r.InspectInterfaces(context.Background(), clablabruntime.NodeRequest{
		Name: "lab1", Nodes: []string{"R1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(interfaces) != 1 || interfaces[0].ContainerName != "lab1-r1" ||
		len(interfaces[0].Interfaces) != 2 {
		t.Fatalf("interfaces = %+v", interfaces)
	}
	first := interfaces[0].Interfaces[0]
	if first.InterfaceName != "e1-1" || first.InterfaceAlias != "ethernet-1/1" ||
		first.InterfaceMAC != "aa:c1:ab:00:00:01" || first.InterfaceIndex != 12 ||
		first.InterfaceMTU != 9500 || first.InterfaceState != "up" {
		t.Fatalf("e1-1 = %+v", first)
	}
	if second := interfaces[0].Interfaces[1]; second.InterfaceState != "lowerlayerdown" {
		t.Fatalf("e1-2 = %+v", second)
	}
}

// TestCaptureClosesStdinWhenItEnds ends every clabwire process through its stdin, whether the
// caller cancels the capture or the output fails, so no capture is left running in the pod.
func TestCaptureClosesStdinWhenItEnds(t *testing.T) {
	t.Parallel()

	targets := []clablabruntime.CaptureTarget{
		{Node: "r1", Interface: "e1-1"},
		{Node: "r2", Interface: "e1-1"},
	}

	t.Run("cancel", func(t *testing.T) {
		t.Parallel()

		r, clabwire := captureTestRuntime(t, 2*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		if err := r.Capture(ctx, clablabruntime.CaptureRequest{Name: "lab1", Targets: targets},
			io.Discard); err != nil {
			t.Fatal(err)
		}
		requireStdinClosedEverywhere(t, clabwire)
	})

	t.Run("output fails", func(t *testing.T) {
		t.Parallel()

		r, clabwire := captureTestRuntime(t, 2*time.Millisecond)
		output := &failingWriter{accept: 3}
		err := r.Capture(context.Background(), clablabruntime.CaptureRequest{
			Name: "lab1", Targets: targets, Duration: time.Hour,
		}, output)
		if err == nil || !strings.Contains(err.Error(), "broken pipe") {
			t.Fatalf("Capture() = %v, want the write error", err)
		}
		requireStdinClosedEverywhere(t, clabwire)
	})
}

// failingWriter accepts some writes, then fails like a closed pipe.
type failingWriter struct {
	mu     sync.Mutex
	accept int
}

func (w *failingWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.accept == 0 {
		return 0, errors.New("write |1: broken pipe")
	}
	w.accept--

	return len(data), nil
}

func requireStdinClosedEverywhere(t *testing.T, clabwire *fakeClabwire) {
	t.Helper()

	clabwire.mu.Lock()
	defer clabwire.mu.Unlock()
	for _, node := range []string{"r1", "r2"} {
		if clabwire.processes[node] == 0 || clabwire.stdinClosed[node] != clabwire.processes[node] {
			t.Fatalf("%s: %d processes, %d ended through stdin", node,
				clabwire.processes[node], clabwire.stdinClosed[node])
		}
	}
}

// TestCaptureRejectsAClabwireWithoutStopOnStdinClose turns the usage error of an older clabwire
// into the release error.
func TestCaptureRejectsAClabwireWithoutStopOnStdinClose(t *testing.T) {
	t.Parallel()

	r, clabwire := captureTestRuntime(t, time.Millisecond)
	clabwire.stderrOnly = "flag provided but not defined: -stop-on-stdin-close"

	err := r.Capture(context.Background(), clablabruntime.CaptureRequest{
		Name: "lab1", Targets: []clablabruntime.CaptureTarget{{Node: "r2", Interface: "e1-1"}},
		Duration: time.Second,
	}, io.Discard)
	if !errors.Is(err, clablabruntime.ErrUnsupportedRuntimeVersion) ||
		!strings.Contains(err.Error(), "clabwire in pod lab-ns/lab1-r2-def does not accept "+
			"packet-capture --stop-on-stdin-close") ||
		!strings.Contains(err.Error(), "(0.11 or newer)") {
		t.Fatalf("Capture() = %v, want the release error", err)
	}
}

// requireTimeOrdered checks that the merged stream's timestamps never decrease.
func requireTimeOrdered(t *testing.T, packets []pcapngPacket) {
	t.Helper()

	for idx := 1; idx < len(packets); idx++ {
		if packets[idx].Timestamp < packets[idx-1].Timestamp {
			t.Fatalf("packet %d (%s) at %d precedes packet %d (%s) at %d", idx,
				packetOrigin(packets[idx]), packets[idx].Timestamp, idx-1,
				packetOrigin(packets[idx-1]), packets[idx-1].Timestamp)
		}
	}
}

// TestCaptureMergesNodesInTimestampOrder merges two nodes whose streams arrive with different
// delays: r2's packets are stamped 60 ms before they reach containerlab. The reorder window puts
// them in order; without it they are not.
func TestCaptureMergesNodesInTimestampOrder(t *testing.T) {
	t.Parallel()

	for _, reorder := range []bool{true, false} {
		t.Run(fmt.Sprintf("reorder=%v", reorder), func(t *testing.T) {
			t.Parallel()

			r, clabwire := captureTestRuntime(t, 5*time.Millisecond)
			clabwire.stampSkew["r2"] = 60 * time.Millisecond
			if !reorder {
				r.captureReorderWindow = -1
			}

			var out bytes.Buffer
			err := r.Capture(context.Background(), clablabruntime.CaptureRequest{
				Name: "lab1",
				Targets: []clablabruntime.CaptureTarget{
					{Node: "r1", Interface: "e1-1"},
					{Node: "r2", Interface: "e1-1"},
				},
				Duration: 500 * time.Millisecond,
			}, &out)
			if err != nil {
				t.Fatal(err)
			}

			packets := parseCapture(t, out.Bytes()).packets
			ordered := slices.IsSortedFunc(packets, func(a, b pcapngPacket) int {
				return int(a.Timestamp - b.Timestamp)
			})
			if reorder {
				requireTimeOrdered(t, packets)
			} else if ordered {
				t.Fatal("the skewed streams came out ordered without the reorder window")
			}
		})
	}
}

// TestCaptureChainsSeveralNodesInOrderWithoutDuplicates chains two nodes with skewed streams and a
// reorder window shorter than a segment switch:
// every tick appears exactly once per interface and the merged stream stays in timestamp order
// across every segment switch, while old segments are ended through stdin at the switch.
func TestCaptureChainsSeveralNodesInOrderWithoutDuplicates(t *testing.T) {
	t.Parallel()

	r, clabwire := captureTestRuntime(t, 4*time.Millisecond)
	r.captureSegment = 300 * time.Millisecond
	// a window shorter than a switch: the chains' holds keep the order across the switches
	r.captureReorderWindow = 15 * time.Millisecond
	clabwire.stampSkew["r2"] = 5 * time.Millisecond
	// r2 switches segments 120 ms after r1, so one chain streams live while the other holds
	clabwire.firstStartDelay["r2"] = 120 * time.Millisecond

	var gaps []clablabruntime.CaptureGap
	var out bytes.Buffer
	err := r.Capture(context.Background(), clablabruntime.CaptureRequest{
		Name: "lab1",
		Targets: []clablabruntime.CaptureTarget{
			{Node: "r1", Interface: "e1-1"},
			{Node: "r2", Interface: "e1-1"},
		},
		Duration: 1300 * time.Millisecond,
		OnGap:    func(gap clablabruntime.CaptureGap) { gaps = append(gaps, gap) },
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Fatalf("gaps = %+v", gaps)
	}

	parsed := parseCapture(t, out.Bytes())
	requireTimeOrdered(t, parsed.packets)
	for id := range uint32(2) {
		var sequence []uint64
		for _, packet := range parsed.packets {
			if packet.InterfaceID == id {
				sequence = append(sequence, packetSequence(packet))
			}
		}
		slices.Sort(sequence)
		if len(sequence) < 200 {
			t.Fatalf("interface %d has %d packets", id, len(sequence))
		}
		for idx := 1; idx < len(sequence); idx++ {
			if sequence[idx] != sequence[idx-1]+1 {
				t.Fatalf("interface %d: tick %d followed by %d", id, sequence[idx-1], sequence[idx])
			}
		}
	}

	clabwire.mu.Lock()
	defer clabwire.mu.Unlock()
	for _, node := range []string{"r1", "r2"} {
		if clabwire.processes[node] < 4 || clabwire.stdinClosed[node] == 0 {
			t.Fatalf("%s: %d processes, %d stopped through stdin at a switch", node,
				clabwire.processes[node], clabwire.stdinClosed[node])
		}
	}
}
