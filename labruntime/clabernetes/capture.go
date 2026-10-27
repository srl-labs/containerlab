package clabernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	"golang.org/x/sync/errgroup"
)

// The clabwire packet capture contract (clabernetes 0.11 or newer).
const (
	clabwireContainer = "clabwire"
	clabwireManager   = "/clabernetes/manager"
	// captureAuditSchema is the schema of the JSON audit lines clabwire writes to stderr.
	captureAuditSchema = "c9s.direct-packet-capture-audit/v1alpha1"

	captureAuditStarted   = "Started"
	captureAuditSucceeded = "Succeeded"
	captureAuditFailed    = "Failed"
	captureAuditDenied    = "Denied"

	// DefaultCaptureSnapLength is clabwire's default snap length.
	DefaultCaptureSnapLength = 256 << 10
	minCaptureSnapLength     = 64
	maxCaptureSnapLength     = 1 << 20
	// maxCaptureSegment is the longest single clabwire capture; longer captures are chained.
	maxCaptureSegment = time.Hour
	// maxClabwirePackets is the packet limit of a single clabwire capture.
	maxClabwirePackets = 1_000_000
	// maxCaptureOverlap is how long two chained segments run side by side.
	maxCaptureOverlap = 10 * time.Second
	// maxCaptureCutWindow is the half-width of the window around the cut in which packets of
	// two segments are matched by content; it covers the difference between the timestamps
	// two clabwire processes give the same packet.
	maxCaptureCutWindow = 500 * time.Millisecond
	// minCaptureSegment is the shortest segment the chain requests.
	minCaptureSegment = 100 * time.Millisecond
	// captureEndGrace is how long a capture with a duration waits for its last clabwire
	// process to end after the duration passed.
	captureEndGrace = 30 * time.Second
	// captureStartRetries bounds the attempts to start a follow-up segment.
	captureStartRetries = 3

	// envCaptureSegment overrides the segment length. It exists to exercise chaining in tests
	// without hour-long captures.
	envCaptureSegment = "CLAB_C9S_CAPTURE_SEGMENT"
)

// podExecFunc runs a command in a pod container and streams its output. A nonzero exit returns
// a *labruntime.CommandExitError.
type podExecFunc func(
	ctx context.Context,
	namespace, pod, container string,
	command []string,
	stdout, stderr io.Writer,
) error

// clabwireInterface is one entry of `packet-capture --list-interfaces`.
type clabwireInterface struct {
	Node          string `json:"node"`
	Interface     string `json:"interface"`
	Alias         string `json:"alias,omitempty"`
	PeerNode      string `json:"peerNode"`
	PeerInterface string `json:"peerInterface"`
}

// captureAudit is one clabwire audit line.
type captureAudit struct {
	SchemaVersion string    `json:"schemaVersion"`
	Time          time.Time `json:"time"`
	Status        string    `json:"status"`
	NodeName      string    `json:"nodeName"`
	Format        string    `json:"format"`
	Reason        string    `json:"reason"`
	Packets       int       `json:"packets"`
	Interfaces    []struct {
		Name    string `json:"name"`
		ID      string `json:"id"`
		Packets int    `json:"packets"`
	} `json:"interfaces"`
}

func (r *Runtime) execInPodContainer() podExecFunc {
	if r.podExec != nil {
		return r.podExec
	}

	return r.streamPodExec
}

// captureSegmentLength is the length of one clabwire capture before the next takes over.
func (r *Runtime) captureSegmentLength() time.Duration {
	if r.captureSegment > 0 && r.captureSegment <= maxCaptureSegment {
		return r.captureSegment
	}

	return maxCaptureSegment
}

func captureSegmentFromEnv() time.Duration {
	value, err := time.ParseDuration(os.Getenv(envCaptureSegment))
	if err != nil || value <= 0 {
		return 0
	}

	return value
}

// CaptureInterfaces lists the capturable lab link interfaces of the requested nodes, or of every
// node, as clabwire reports them, with their peers. Node names are reported as the topology
// spells them.
func (r *Runtime) CaptureInterfaces(
	ctx context.Context,
	req clablabruntime.NodeRequest,
) ([]clablabruntime.CaptureInterface, error) {
	r, err := r.connect(ctx, req.Identity)
	if err != nil {
		return nil, err
	}

	locations, err := r.LocateNode(ctx, clablabruntime.NodeRequest{
		Name: req.Name, Namespace: req.Namespace, Nodes: req.Nodes, Identity: req.Identity,
	})
	if err != nil {
		return nil, err
	}
	if len(req.Nodes) == 0 {
		// a lab-wide listing skips nodes that do not run
		running := locations[:0]
		for _, location := range locations {
			if location.Pod != "" {
				running = append(running, location)
			}
		}
		locations = running
	}

	listed, err := r.listCaptureInterfaces(ctx, req.Name, locations)
	if err != nil {
		return nil, err
	}
	names, err := r.nodeDisplayNames(ctx, req.Name, locations)
	if err != nil {
		return nil, err
	}

	result := make([]clablabruntime.CaptureInterface, 0)
	for _, location := range locations {
		for _, entry := range listed[location.NodeName] {
			result = append(result, clablabruntime.CaptureInterface{
				Node:          names.display(entry.Node),
				Interface:     entry.Interface,
				Alias:         entry.Alias,
				PeerNode:      names.display(entry.PeerNode),
				PeerInterface: entry.PeerInterface,
			})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Node != result[j].Node {
			return result[i].Node < result[j].Node
		}

		return result[i].Interface < result[j].Interface
	})

	return result, nil
}

// listCaptureInterfaces runs one --list-interfaces per pod and returns the entries per Node.
func (r *Runtime) listCaptureInterfaces(
	ctx context.Context,
	lab string,
	locations []clablabruntime.NodeLocation,
) (map[string][]clabwireInterface, error) {
	pods := map[string]clablabruntime.NodeLocation{}
	wanted := map[string]bool{}
	for _, location := range locations {
		if location.Pod == "" {
			return nil, fmt.Errorf("c9s node %s/%s/%s has no pod; is it running?",
				location.Namespace, lab, location.NodeName)
		}
		pods[location.Namespace+"/"+location.Pod] = location
		wanted[location.NodeName] = true
	}

	var mu sync.Mutex
	result := map[string][]clabwireInterface{}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(interfaceSampleWorkers)
	exec := r.execInPodContainer()
	for _, location := range pods {
		group.Go(func() error {
			var stdout, stderr bytes.Buffer
			err := exec(groupCtx, location.Namespace, location.Pod, clabwireContainer,
				[]string{clabwireManager, "node-runtime", "packet-capture", "--list-interfaces"},
				&stdout, &stderr)
			if err != nil {
				return fmt.Errorf("failed to list the capturable interfaces of pod %s/%s: %w%s",
					location.Namespace, location.Pod, err, stderrSuffix(stderr.Bytes()))
			}

			var entries []clabwireInterface
			if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
				return fmt.Errorf("failed to parse the capturable interfaces of pod %s/%s: %w",
					location.Namespace, location.Pod, err)
			}

			mu.Lock()
			defer mu.Unlock()
			for _, entry := range entries {
				if wanted[entry.Node] {
					result[entry.Node] = append(result[entry.Node], entry)
				}
			}

			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	return result, nil
}

func stderrSuffix(stderr []byte) string {
	text := strings.TrimSpace(string(stderr))
	if text == "" {
		return ""
	}

	return ": " + text
}

// nodeNames maps the node names the cluster carries to the names the topology file uses.
type nodeNames map[string]string

func (n nodeNames) display(name string) string {
	if original := n[name]; original != "" {
		return original
	}

	return name
}

// nodeDisplayNames reads the original node names from the c9s.run/originalNodeName label of the
// lab's Nodes.
func (r *Runtime) nodeDisplayNames(
	ctx context.Context,
	lab string,
	locations []clablabruntime.NodeLocation,
) (nodeNames, error) {
	names := nodeNames{}
	if len(locations) == 0 {
		return names, nil
	}

	nodes, err := r.nodesForTopology(ctx, lab, locations[0].Namespace)
	if err != nil {
		return nil, err
	}
	for idx := range nodes.Items {
		if original := nodes.Items[idx].GetLabels()[labelOriginalNodeName]; original != "" {
			names[nodes.Items[idx].GetName()] = original
		}
	}

	return names, nil
}

// captureGroup is the part of a capture one clabwire process serves: the requested interfaces of
// one node, in one pod.
type captureGroup struct {
	location clablabruntime.NodeLocation
	display  string
	// interfaces are the planned interface names, in the order the merged stream lists them.
	interfaces []string
	// ids are the merged interface IDs of interfaces.
	ids []uint32
}

// resolvedCapture is a validated capture request.
type resolvedCapture struct {
	groups []*captureGroup
	// names are the merged interface names (<node>:<interface>), indexed by merged ID.
	names []string
}

// resolveCaptureTargets locates every target node, checks every interface against the
// interfaces clabwire allows, and groups the targets per node.
func (r *Runtime) resolveCaptureTargets(
	ctx context.Context,
	req clablabruntime.CaptureRequest,
) (*resolvedCapture, error) {
	nodeOrder := make([]string, 0, len(req.Targets))
	seenNodes := map[string]bool{}
	for _, target := range req.Targets {
		if !seenNodes[target.Node] {
			seenNodes[target.Node] = true
			nodeOrder = append(nodeOrder, target.Node)
		}
	}

	locations, err := r.LocateNode(ctx, clablabruntime.NodeRequest{
		Name: req.Name, Namespace: req.Namespace, Nodes: nodeOrder, Identity: req.Identity,
	})
	if err != nil {
		return nil, err
	}
	byRequest := make(map[string]clablabruntime.NodeLocation, len(nodeOrder))
	for idx, name := range nodeOrder {
		byRequest[name] = locations[idx]
	}

	listed, err := r.listCaptureInterfaces(ctx, req.Name, locations)
	if err != nil {
		return nil, err
	}
	names, err := r.nodeDisplayNames(ctx, req.Name, locations)
	if err != nil {
		return nil, err
	}

	resolved := &resolvedCapture{}
	groups := map[string]*captureGroup{}
	seen := map[string]bool{}
	for _, target := range req.Targets {
		location := byRequest[target.Node]
		entry, ok := findCaptureInterface(listed[location.NodeName], target.Interface)
		if !ok {
			return nil, fmt.Errorf("interface %q of node %q is not a capturable link "+
				"interface; capturable: %s", target.Interface, target.Node,
				describeCaptureInterfaces(listed[location.NodeName]))
		}

		key := location.NodeName + "\x00" + entry.Interface
		if seen[key] {
			return nil, fmt.Errorf("interface %q of node %q is requested more than once",
				entry.Interface, target.Node)
		}
		seen[key] = true

		group := groups[location.NodeName]
		if group == nil {
			group = &captureGroup{location: location, display: names.display(location.NodeName)}
			groups[location.NodeName] = group
			resolved.groups = append(resolved.groups, group)
		}

		id := uint32(len(resolved.names)) //nolint:gosec // bounded by the targets
		group.interfaces = append(group.interfaces, entry.Interface)
		group.ids = append(group.ids, id)
		resolved.names = append(resolved.names, group.display+":"+entry.Interface)
	}

	return resolved, nil
}

// findCaptureInterface matches a requested interface by its planned name or its alias.
func findCaptureInterface(entries []clabwireInterface, name string) (clabwireInterface, bool) {
	for _, entry := range entries {
		if entry.Interface == name {
			return entry, true
		}
	}
	for _, entry := range entries {
		if entry.Alias != "" && entry.Alias == name {
			return entry, true
		}
	}

	return clabwireInterface{}, false
}

func describeCaptureInterfaces(entries []clabwireInterface) string {
	if len(entries) == 0 {
		return "none"
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Interface
		if entry.Alias != "" {
			name += " (" + entry.Alias + ")"
		}
		names = append(names, name)
	}

	return strings.Join(names, ", ")
}

// validateCaptureRequest checks the request bounds and fills the default snap length.
func validateCaptureRequest(req *clablabruntime.CaptureRequest) error {
	if req.Name == "" {
		return fmt.Errorf("topology name is required")
	}
	if len(req.Targets) == 0 {
		return fmt.Errorf("at least one capture target (node and interface) is required")
	}
	for _, target := range req.Targets {
		if target.Node == "" || target.Interface == "" {
			return fmt.Errorf("capture target %+v needs a node and an interface", target)
		}
	}
	if req.Duration < 0 {
		return fmt.Errorf("capture duration must not be negative")
	}
	if req.PacketLimit < 0 {
		return fmt.Errorf("capture packet limit must not be negative")
	}
	if req.SnapLength == 0 {
		req.SnapLength = DefaultCaptureSnapLength
	}
	if req.SnapLength < minCaptureSnapLength || req.SnapLength > maxCaptureSnapLength {
		return fmt.Errorf("capture snap length %d is outside %d to %d bytes", req.SnapLength,
			minCaptureSnapLength, maxCaptureSnapLength)
	}

	return nil
}

// Capture streams pcapng from lab link interfaces to w: one section, one Interface Description
// Block per target in request order named <node>:<interface>, then the packets of every target.
//
// Each node's interfaces are captured by one clabwire process in the node's pod. clabwire limits
// a capture to one hour, so a longer capture (or one without Duration) chains captures: the next
// one starts before the current one ends, and the stream switches at a cut in the overlap. See
// captureChain for how packets of the overlap are kept once.
//
// Capture returns when Duration has passed, PacketLimit packets were written, or ctx ends; an
// ended ctx is not an error. Every block reaches w in one write, so the stream is a valid capture
// whenever it stops. A clabwire Denied or Failed audit record is returned as an error.
func (r *Runtime) Capture(
	ctx context.Context,
	req clablabruntime.CaptureRequest,
	w io.Writer,
) error {
	if err := validateCaptureRequest(&req); err != nil {
		return err
	}

	r, err := r.connect(ctx, req.Identity)
	if err != nil {
		return err
	}

	resolved, err := r.resolveCaptureTargets(ctx, req)
	if err != nil {
		return err
	}

	writer := newPCAPNGWriter(w)
	if err := writer.WriteHeader(resolved.names, uint32(req.SnapLength)); err != nil { //nolint:gosec // validated
		return err
	}

	captureCtx, stop := context.WithCancel(ctx)
	defer stop()

	run := &captureRun{
		r:        r,
		req:      req,
		writer:   writer,
		stop:     stop,
		segment:  r.captureSegmentLength(),
		deadline: time.Time{},
	}
	if req.Duration > 0 {
		run.deadline = time.Now().Add(req.Duration)
	}
	run.overlap = min(maxCaptureOverlap, run.segment/3)
	run.window = min(maxCaptureCutWindow, run.overlap/4)

	if req.Duration > 0 {
		// a clabwire process that does not end on its own does not hold the capture open
		var cancelTimeout context.CancelFunc
		captureCtx, cancelTimeout = context.WithTimeout(captureCtx,
			req.Duration+run.overlap+captureEndGrace)
		defer cancelTimeout()
	}

	group, groupCtx := errgroup.WithContext(captureCtx)
	for _, captureGroup := range resolved.groups {
		chain := &captureChain{capture: run, group: captureGroup}
		group.Go(func() error {
			return chain.run(groupCtx)
		})
	}

	err = group.Wait()
	if writeErr := writer.err; writeErr != nil {
		return writeErr
	}
	if err != nil && !(errors.Is(err, context.Canceled) && captureCtx.Err() != nil) {
		return err
	}

	return nil
}

// captureRun is the state a capture's chains share.
type captureRun struct {
	r      *Runtime
	req    clablabruntime.CaptureRequest
	writer *pcapngWriter
	// stop ends every chain, once the packet limit is reached.
	stop context.CancelFunc
	// written counts the packets written to the stream.
	written atomic.Int64

	segment  time.Duration
	overlap  time.Duration
	window   time.Duration
	deadline time.Time
}

// emit writes a packet under its merged interface ID. It reports false once the packet limit is
// reached and the capture stops.
func (run *captureRun) emit(id uint32, packet *pcapngPacket) (bool, error) {
	if run.req.PacketLimit > 0 {
		count := run.written.Add(1)
		if count > int64(run.req.PacketLimit) {
			run.stop()
			return false, nil
		}
		if err := run.writer.WritePacket(id, packet); err != nil {
			return false, err
		}
		if count == int64(run.req.PacketLimit) {
			run.stop()
			return false, nil
		}

		return true, nil
	}

	run.written.Add(1)

	return true, run.writer.WritePacket(id, packet)
}

func (run *captureRun) reportGap(gap clablabruntime.CaptureGap) {
	if run.req.OnGap != nil {
		run.req.OnGap(gap)
		return
	}

	log.Warn("Capture has a gap", "node", gap.Node, "interfaces", strings.Join(gap.Interfaces, ","),
		"from", gap.From.Format(time.RFC3339Nano), "to", gap.To.Format(time.RFC3339Nano),
		"reason", gap.Reason)
}

// captureSegment is one clabwire capture process of a chain.
type captureSegment struct {
	index      int
	localStart time.Time
	duration   time.Duration
	cancel     context.CancelFunc
	// started and ended are the pod-clock times of the Started and Succeeded audit records.
	started time.Time
	ended   time.Time
	done    bool
}

func (s *captureSegment) plannedEnd() time.Time {
	return s.started.Add(s.duration)
}

// segmentEvent is something a segment reported: a packet, an audit record, or its end.
type segmentEvent struct {
	segment *captureSegment
	packet  *pcapngPacket
	audit   *captureAudit
	done    bool
	err     error
}

// captureChain runs the chained clabwire captures of one group.
//
// Chaining and the cut: segment k+1 starts `overlap` before segment k is due to end, so both
// capture the same packets for a while. clabwire stamps packets when it reads them, so two
// processes give the same packet timestamps a few microseconds apart, and timestamps alone
// cannot pair them. The chain therefore cuts in the middle of the overlap and matches packets by
// content close to the cut:
//
//  1. When segment k+1 reports Started (pod clock, from its audit record), the cut is
//     T = midpoint of [Started(k+1), Started(k) + duration(k)]: both segments run then.
//  2. Segment k keeps streaming live, but only packets stamped before T; it remembers the
//     content of what it wrote since segment k+1 started.
//  3. Segment k+1's packets are held back until segment k has ended. Then the ones stamped
//     before T-W are dropped (segment k had them), the ones stamped within W of T are dropped
//     when segment k wrote the same bytes on the same interface, and every other one is written.
//     From then on segment k+1 streams live.
//
// W (500 ms, less for short test segments) covers the stamping difference. Identical frames on
// the same interface within W of the cut can be paired with each other, which can drop a true
// repeat; nothing else is lost or doubled. A follow-up segment that does not start in time leaves
// a gap, which is reported through CaptureRequest.OnGap (or logged), never silently ignored.
type captureChain struct {
	capture *captureRun
	group   *captureGroup

	events chan segmentEvent
	wg     sync.WaitGroup

	active *captureSegment
	next   *captureSegment
	// cut is the switch time between active and next, in nanoseconds; zero until known.
	cut int64
	// emitted remembers the content of the packets active wrote while next runs.
	emitted map[uint64]int
	// held are the packets of next until active ends.
	held     []*pcapngPacket
	segments int
	retries  int
	// final is set once no further segment is due.
	final bool
}

func (c *captureChain) run(ctx context.Context) error {
	c.events = make(chan segmentEvent, 256)
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		c.drain()
	}()

	c.active = c.start(ctx, c.segmentDuration(time.Now()))
	timer := time.NewTimer(c.nextStartIn())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			if c.next == nil && !c.final {
				c.startNext(ctx)
			}
		case event := <-c.events:
			stop, err := c.handle(ctx, event, timer)
			if err != nil || stop {
				return err
			}
		}
	}
}

// drain waits for the segment goroutines after the chain ended.
func (c *captureChain) drain() {
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	for {
		select {
		case <-done:
			return
		case <-c.events:
		}
	}
}

// segmentDuration is the requested duration of a segment starting now.
func (c *captureChain) segmentDuration(now time.Time) time.Duration {
	duration := c.capture.segment
	if !c.capture.deadline.IsZero() {
		duration = min(duration, c.capture.deadline.Sub(now))
	}

	return max(duration, minCaptureSegment)
}

// nextStartIn is the time until the follow-up of the active segment is due. It marks the chain
// final when none is due.
func (c *captureChain) nextStartIn() time.Duration {
	activeEnd := c.active.localStart.Add(c.active.duration)
	if !c.capture.deadline.IsZero() && !activeEnd.Before(c.capture.deadline.Add(-c.capture.window)) {
		c.final = true
		return maxCaptureSegment
	}
	c.final = false

	return max(time.Until(activeEnd.Add(-c.capture.overlap)), 0)
}

func (c *captureChain) startNext(ctx context.Context) {
	c.next = c.start(ctx, c.segmentDuration(time.Now()))
	c.emitted = map[uint64]int{}
	c.held = nil
	c.cut = 0
}

func (c *captureChain) handle(
	ctx context.Context,
	event segmentEvent,
	timer *time.Timer,
) (bool, error) {
	segment := event.segment

	switch {
	case event.audit != nil:
		return false, c.handleAudit(segment, event.audit)
	case event.packet != nil:
		return c.handlePacket(segment, event.packet)
	case event.done:
		segment.done = true
		if event.err != nil && ctx.Err() == nil {
			if segment == c.next && segment.started.IsZero() && c.retries < captureStartRetries {
				// the follow-up did not start; try again while the active one still runs
				c.retries++
				log.Warn("Capture follow-up segment failed to start, retrying",
					"node", c.group.display, "error", event.err)
				c.next = nil
				resetTimer(timer, time.Second)

				return false, nil
			}

			return true, fmt.Errorf("capture on c9s node %s/%s/%s failed: %w",
				c.group.location.Namespace, c.capture.req.Name, c.group.display, event.err)
		}
		if segment != c.active {
			return false, nil
		}

		return c.switchSegments(ctx, timer)
	}

	return false, nil
}

func (c *captureChain) handleAudit(segment *captureSegment, audit *captureAudit) error {
	switch audit.Status {
	case captureAuditStarted:
		segment.started = audit.Time
		log.Debug("Capture segment started", "node", c.group.display, "segment", segment.index,
			"interfaces", strings.Join(c.group.interfaces, ","), "duration", segment.duration)
		if segment == c.next && !c.active.started.IsZero() {
			c.setCut()
		}
		if segment == c.next {
			c.retries = 0
		}
	case captureAuditSucceeded:
		segment.ended = audit.Time
		log.Debug("Capture segment succeeded", "node", c.group.display,
			"segment", segment.index, "packets", audit.Packets)
	case captureAuditDenied, captureAuditFailed:
		return captureAuditError(c.group.display, audit)
	}

	return nil
}

// captureAuditError turns a Denied or Failed clabwire audit record into an error.
func captureAuditError(node string, audit *captureAudit) error {
	verb := "failed"
	if audit.Status == captureAuditDenied {
		verb = "was denied"
	}
	interfaces := make([]string, 0, len(audit.Interfaces))
	for _, intf := range audit.Interfaces {
		interfaces = append(interfaces, intf.Name)
	}

	return fmt.Errorf("capture of %s on node %s %s by clabwire: %s",
		strings.Join(interfaces, ","), node, verb, audit.Reason)
}

// setCut places the cut in the middle of the overlap of the active and the next segment, both
// measured on the pod clock.
func (c *captureChain) setCut() {
	overlapStart := c.next.started
	overlapEnd := c.active.plannedEnd()
	if overlapEnd.Sub(overlapStart) < 2*c.capture.window {
		// the overlap is too short to match packets around a cut: the switch is reported as a
		// gap once the active segment ended
		c.cut = 0
		return
	}
	c.cut = overlapStart.Add(overlapEnd.Sub(overlapStart) / 2).UnixNano()
}

func (c *captureChain) handlePacket(segment *captureSegment, packet *pcapngPacket) (bool, error) {
	switch segment {
	case c.active:
		if c.cut != 0 && packet.Timestamp >= c.cut {
			return false, nil
		}
		if c.next != nil {
			c.emitted[packetKey(packet)]++
		}

		return c.emit(packet)
	case c.next:
		c.held = append(c.held, packet)
	}

	return false, nil
}

func (c *captureChain) emit(packet *pcapngPacket) (bool, error) {
	if int(packet.InterfaceID) >= len(c.group.ids) {
		return true, fmt.Errorf("clabwire wrote a packet for interface %d of %d",
			packet.InterfaceID, len(c.group.ids))
	}
	more, err := c.capture.emit(c.group.ids[packet.InterfaceID], packet)

	return !more, err
}

// switchSegments makes the next segment the active one after the active one ended, writing the
// held packets of the next one that the active one did not write.
func (c *captureChain) switchSegments(ctx context.Context, timer *time.Timer) (bool, error) {
	ended := c.active
	if c.final && c.next == nil {
		return true, nil
	}

	if c.next == nil || c.next.started.IsZero() || c.cut == 0 {
		from := ended.ended
		if from.IsZero() {
			from = ended.plannedEnd()
		}
		gap := clablabruntime.CaptureGap{
			Node: c.group.display, Interfaces: c.group.interfaces, From: from,
			Reason: "the follow-up capture did not overlap the previous one",
		}
		if c.next != nil && !c.next.started.IsZero() {
			gap.To = c.next.started
		}
		c.capture.reportGap(gap)
	}

	if c.next != nil && c.next.done {
		// the follow-up already ended (it started too late to take over): keep what it
		// captured and continue with a fresh segment
		for _, packet := range c.held {
			if stop, err := c.emit(packet); stop || err != nil {
				return true, err
			}
		}
		c.next, c.held, c.emitted, c.cut = nil, nil, nil, 0
	}

	if c.next == nil {
		if !c.capture.deadline.IsZero() &&
			!time.Now().Before(c.capture.deadline.Add(-c.capture.window)) {
			return true, nil
		}
		c.active = c.start(ctx, c.segmentDuration(time.Now()))
		resetTimer(timer, c.nextStartIn())

		return false, nil
	}

	window := c.capture.window.Nanoseconds()
	for _, packet := range c.held {
		if c.cut != 0 {
			if packet.Timestamp < c.cut-window {
				continue
			}
			if packet.Timestamp < c.cut+window {
				key := packetKey(packet)
				if c.emitted[key] > 0 {
					c.emitted[key]--
					continue
				}
			}
		}
		if stop, err := c.emit(packet); stop || err != nil {
			return true, err
		}
	}

	c.active, c.next = c.next, nil
	c.held, c.emitted, c.cut = nil, nil, 0
	resetTimer(timer, c.nextStartIn())

	return false, nil
}

func resetTimer(timer *time.Timer, d time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)
}

// packetKey identifies a packet by its interface and content.
func packetKey(packet *pcapngPacket) uint64 {
	hash := fnv.New64a()
	var id [4]byte
	id[0] = byte(packet.InterfaceID)
	id[1] = byte(packet.InterfaceID >> 8)
	_, _ = hash.Write(id[:])
	_, _ = hash.Write(packet.Data)

	return hash.Sum64()
}

// start runs one clabwire capture and reports its packets, audit records and end as events.
func (c *captureChain) start(ctx context.Context, duration time.Duration) *captureSegment {
	segmentCtx, cancel := context.WithCancel(ctx)
	segment := &captureSegment{
		index: c.segments, localStart: time.Now(), duration: duration, cancel: cancel,
	}
	c.segments++

	command := []string{
		clabwireManager, "node-runtime", "packet-capture",
		"--nodeName", c.group.location.NodeName,
		"--format", "pcapng",
		"--snapLength", strconv.Itoa(c.capture.req.SnapLength),
		"--duration", duration.String(),
	}
	for _, name := range c.group.interfaces {
		command = append(command, "--interface", name)
	}
	if c.capture.req.PacketLimit > 0 {
		remaining := int64(c.capture.req.PacketLimit) - c.capture.written.Load()
		command = append(command, "--packetLimit",
			strconv.FormatInt(min(max(remaining, 1), maxClabwirePackets), 10))
	}

	send := func(event segmentEvent) {
		event.segment = segment
		select {
		case c.events <- event:
		case <-segmentCtx.Done():
		}
	}

	stdoutReader, stdoutWriter := io.Pipe()
	stderr := &auditLineWriter{send: func(audit *captureAudit) { send(segmentEvent{audit: audit}) }}

	c.wg.Add(2)
	readDone := make(chan error, 1)
	go func() {
		defer c.wg.Done()
		readDone <- c.readSegment(stdoutReader, func(packet *pcapngPacket) {
			send(segmentEvent{packet: packet})
		})
		_, _ = io.Copy(io.Discard, stdoutReader)
	}()
	go func() {
		defer c.wg.Done()
		defer cancel()

		err := c.capture.r.execInPodContainer()(segmentCtx, c.group.location.Namespace,
			c.group.location.Pod, clabwireContainer, command, stdoutWriter, stderr)
		_ = stdoutWriter.Close()
		readErr := <-readDone
		stderr.flush()
		if err == nil && readErr != nil {
			err = readErr
		}
		if err != nil && stderr.text() != "" {
			err = fmt.Errorf("%w: %s", err, stderr.text())
		}
		send(segmentEvent{done: true, err: err})
	}()

	return segment
}

// readSegment checks that a segment's interfaces are the requested ones and passes its packets
// on with the segment's interface index.
func (c *captureChain) readSegment(stdout io.Reader, packet func(*pcapngPacket)) error {
	reader := newPCAPNGReader(stdout)
	interfaces := 0
	for {
		event, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			// the process was stopped inside a block
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to read the clabwire capture: %w", err)
		}
		if event.Interface != nil {
			if interfaces >= len(c.group.interfaces) ||
				event.Interface.Name != c.group.interfaces[interfaces] {
				return fmt.Errorf("clabwire described interface %q, want %s in order",
					event.Interface.Name, strings.Join(c.group.interfaces, ", "))
			}
			interfaces++
			continue
		}
		if event.Packet != nil {
			packet(event.Packet)
		}
	}
}

// auditLineWriter parses clabwire's stderr: audit records go to send, other lines are kept for
// error messages.
type auditLineWriter struct {
	send func(*captureAudit)

	mu      sync.Mutex
	partial []byte
	other   []string
}

func (w *auditLineWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	w.partial = append(w.partial, data...)
	var lines [][]byte
	for {
		idx := bytes.IndexByte(w.partial, '\n')
		if idx < 0 {
			break
		}
		lines = append(lines, append([]byte(nil), w.partial[:idx]...))
		w.partial = w.partial[idx+1:]
	}
	w.mu.Unlock()

	for _, line := range lines {
		w.line(line)
	}

	return len(data), nil
}

func (w *auditLineWriter) flush() {
	w.mu.Lock()
	line := w.partial
	w.partial = nil
	w.mu.Unlock()
	if len(bytes.TrimSpace(line)) != 0 {
		w.line(line)
	}
}

func (w *auditLineWriter) line(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}

	var audit captureAudit
	if err := json.Unmarshal(line, &audit); err == nil && audit.SchemaVersion == captureAuditSchema {
		w.send(&audit)
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.other) < 20 {
		w.other = append(w.other, string(line))
	}
}

func (w *auditLineWriter) text() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return strings.Join(w.other, "; ")
}
