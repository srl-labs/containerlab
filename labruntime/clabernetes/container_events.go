package clabernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

// Container event actions, as the Docker runtime reports them.
const (
	containerActionCreate        = "create"
	containerActionStart         = "start"
	containerActionKill          = "kill"
	containerActionStop          = "stop"
	containerActionDie           = "die"
	containerActionDestroy       = "destroy"
	containerActionHealthPrefix  = "health_status: "
	containerEventSignalSIGTERM  = "15"
	containerRefreshDelay        = 200 * time.Millisecond
	containerRefreshRetryDelay   = pollInterval
	containerEventAttributeState = "state"
)

// trackedContainer is the last reported container state of a node.
type trackedContainer struct {
	ref    labRef
	state  string
	health string
	pod    string
	// lab and node are the last observation, which a removed node is described with.
	lab  *clablabruntime.LabState
	node clablabruntime.NodeState
}

// containerTracker turns the node states of the labs in an event scope into the container
// events the Docker runtime emits for the same lifecycle: create and start when a node's pod
// starts, kill, stop and die when it stops (on request, or between the pods of a restart),
// destroy when the node is removed, and health_status as its device becomes ready.
type containerTracker struct {
	r     *Runtime
	sink  chan<- clablabruntime.Event
	nodes map[string]*trackedContainer
}

// trackContainers reports the container events of a scope. It reads the labs once, reporting
// them as a snapshot when the stream includes the initial state, and then reads a lab again
// whenever its Topology, Nodes or pods change. Changes that arrive together are read once.
func (r *Runtime) trackContainers(
	ctx context.Context,
	scope eventScope,
	includeInitialState bool,
	changed <-chan labRef,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	tracker := &containerTracker{r: r, sink: eventSink, nodes: map[string]*trackedContainer{}}

	states, err := r.labStates(ctx, scope)
	if err != nil {
		sendEventError(ctx, errSink, err)
		return
	}
	for _, state := range states {
		tracker.observeLab(ctx, labRef{namespace: state.Namespace, lab: state.Name}, state,
			&observeSeed{snapshot: includeInitialState})
	}

	pending := map[labRef]bool{}
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	armed := false
	arm := func(delay time.Duration) {
		if !armed {
			timer.Reset(delay)
			armed = true
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ref := <-changed:
			pending[ref] = true
			arm(containerRefreshDelay)
		case <-timer.C:
			armed = false
			refs := pending
			pending = map[labRef]bool{}
			for ref := range refs {
				if !tracker.refresh(ctx, ref) {
					pending[ref] = true
				}
			}
			if len(pending) != 0 {
				arm(containerRefreshRetryDelay)
			}
		}
	}
}

// observeSeed marks the first observation of a stream: it records the node states without
// reporting changes, and reports them as a snapshot when asked to.
type observeSeed struct {
	snapshot bool
}

// refresh reads a lab again and reports what changed for its nodes. A lab that is gone reports
// its nodes as destroyed. It returns false when the lab could not be read.
func (t *containerTracker) refresh(ctx context.Context, ref labRef) bool {
	state, err := t.r.Inspect(ctx, clablabruntime.InspectRequest{
		Name:      ref.lab,
		Namespace: ref.namespace,
	})
	if err != nil {
		var notFound *labNotFoundError
		if !errors.As(err, &notFound) {
			if ctx.Err() == nil {
				log.Debug("Failed to read a C9s lab for container events",
					"namespace", ref.namespace, "lab", ref.lab, "error", err)
			}

			return ctx.Err() != nil
		}
		state = nil
	}

	t.observeLab(ctx, ref, state, nil)

	return true
}

// observeLab compares the nodes of a lab with what was reported before and reports the
// difference. A nil state removes every node of the lab.
func (t *containerTracker) observeLab(
	ctx context.Context,
	ref labRef,
	state *clablabruntime.LabState,
	seed *observeSeed,
) {
	seen := map[string]bool{}
	if state != nil {
		for idx := range state.Nodes {
			key := c9sNodeKey(state.Namespace, state.Name, state.Nodes[idx].Name)
			seen[key] = true
			t.observeNode(ctx, ref, state, state.Nodes[idx], seed)
		}
	}

	for key, tracked := range t.nodes {
		if tracked.ref != ref || seen[key] {
			continue
		}

		for _, action := range containerRemovalActions(tracked) {
			t.emit(ctx, tracked.lab, tracked.node, action, nil)
		}
		delete(t.nodes, key)
	}
}

func (t *containerTracker) observeNode(
	ctx context.Context,
	ref labRef,
	state *clablabruntime.LabState,
	node clablabruntime.NodeState,
	seed *observeSeed,
) {
	key := c9sNodeKey(state.Namespace, state.Name, node.Name)
	previous := t.nodes[key]
	if node.ContainerState == "" {
		node.ContainerState = clablabruntime.ContainerStateCreated
	}
	next := &trackedContainer{
		ref:   ref,
		state: node.ContainerState,
		pod:   node.Pod,
		lab:   state,
		node:  node,
	}
	next.health = containerHealth(previous, next, node.Health)
	t.nodes[key] = next

	if seed != nil {
		if seed.snapshot {
			t.emit(ctx, state, node, next.state, map[string]string{
				"origin": clablabruntime.EventOriginSnapshot,
			})
		}

		return
	}

	for _, action := range containerTransitionActions(previous, next) {
		t.emit(ctx, state, node, action, nil)
	}
}

// containerHealth is the health a running node reports: a node that was healthy and is not
// ready anymore in the same pod is unhealthy, as a container whose healthcheck starts failing.
func containerHealth(previous, next *trackedContainer, health string) string {
	if next.state != clablabruntime.ContainerStateRunning {
		return ""
	}
	if health == "" {
		health = clablabruntime.ContainerHealthStarting
	}
	if health == clablabruntime.ContainerHealthStarting && previous != nil &&
		previous.state == clablabruntime.ContainerStateRunning && previous.pod == next.pod &&
		(previous.health == clablabruntime.ContainerHealthHealthy ||
			previous.health == clablabruntime.ContainerHealthUnhealthy) {
		return clablabruntime.ContainerHealthUnhealthy
	}

	return health
}

// containerTransitionActions are the container event actions Docker reports when a container
// goes from the previous to the next state:
//
//	new node            create (+ start when it runs, + stop when it is already stopped)
//	created -> running  start
//	exited  -> running  start
//	running -> exited   kill, stop, die (stop, or the old pod of a restart ending)
//	running -> created  kill, stop, die (the replacement pod of a restart not running yet)
//	running, new pod    kill, stop, die, start (restart)
//	created -> exited   stop
//
// A started node also reports its health (health_status: starting or healthy), and a running
// node reports health changes, like a container with a healthcheck.
func containerTransitionActions(previous, next *trackedContainer) []string {
	const (
		created = clablabruntime.ContainerStateCreated
		running = clablabruntime.ContainerStateRunning
		exited  = clablabruntime.ContainerStateExited
	)

	var actions []string
	previousState := ""
	if previous != nil {
		previousState = previous.state
	}

	switch {
	case previous == nil:
		actions = append(actions, containerActionCreate)
		switch next.state {
		case running:
			actions = append(actions, containerActionStart)
		case exited:
			actions = append(actions, containerActionStop)
		}
	case previousState == running && next.state == running && previous.pod != next.pod:
		actions = append(actions, containerActionKill, containerActionStop, containerActionDie,
			containerActionStart)
	case previousState == running && next.state != running:
		actions = append(actions, containerActionKill, containerActionStop, containerActionDie)
	case previousState != running && next.state == running:
		actions = append(actions, containerActionStart)
	case previousState == created && next.state == exited:
		actions = append(actions, containerActionStop)
	}

	if next.state != running {
		return actions
	}

	started := len(actions) != 0 && actions[len(actions)-1] == containerActionStart
	if !started && previous != nil && previous.health == next.health {
		return actions
	}
	if next.health == clablabruntime.ContainerHealthStarting && !started {
		return actions
	}

	return append(actions, containerActionHealthPrefix+next.health)
}

// containerRemovalActions are the actions Docker reports when a container is removed: a running
// one is killed and dies first.
func containerRemovalActions(previous *trackedContainer) []string {
	if previous.state == clablabruntime.ContainerStateRunning {
		return []string{containerActionKill, containerActionDie, containerActionDestroy}
	}

	return []string{containerActionDestroy}
}

// containerActionState is the container state after an action, which every container event
// reports in its state attribute.
func containerActionState(action, snapshotState string) string {
	switch action {
	case containerActionCreate:
		return clablabruntime.ContainerStateCreated
	case containerActionStop, containerActionDie, containerActionDestroy:
		return clablabruntime.ContainerStateExited
	case containerActionStart, containerActionKill:
		return clablabruntime.ContainerStateRunning
	}
	if strings.HasPrefix(action, containerActionHealthPrefix) {
		return clablabruntime.ContainerStateRunning
	}

	return snapshotState
}

func (t *containerTracker) emit(
	ctx context.Context,
	state *clablabruntime.LabState,
	node clablabruntime.NodeState,
	action string,
	extra map[string]string,
) {
	t.r.sendEvent(ctx, t.sink, c9sContainerEvent(state, node, action, extra))
}

// c9sContainerEvent is a container event of a node. The actor is the node, as for its interface
// events; its ID is the container_id inspect reports. Lab and Node let the events command
// describe the node with the attributes inspect and the Docker runtime report.
func c9sContainerEvent(
	state *clablabruntime.LabState,
	node clablabruntime.NodeState,
	action string,
	extra map[string]string,
) clablabruntime.Event {
	key := c9sNodeKey(state.Namespace, state.Name, node.Name)
	attributes := map[string]string{
		"namespace":                  state.Namespace,
		"lab":                        state.Name,
		"node":                       node.Name,
		containerEventAttributeState: containerActionState(action, node.ContainerState),
	}
	if node.Pod != "" {
		attributes["pod"] = node.Pod
	}
	if action == containerActionKill {
		// Kubernetes stops a pod's containers with SIGTERM.
		attributes["signal"] = containerEventSignalSIGTERM
	}
	for k, v := range extra {
		attributes[k] = v
	}

	nodeCopy := node

	return clablabruntime.Event{
		Timestamp:   time.Now(),
		Type:        clablabruntime.EventTypeContainer,
		Action:      action,
		ActorID:     key,
		ActorName:   fmt.Sprintf("%s-%s", state.Name, node.Name),
		ActorFullID: key,
		Attributes:  attributes,
		Lab:         state,
		Node:        &nodeCopy,
	}
}
