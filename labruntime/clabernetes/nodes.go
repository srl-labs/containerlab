package clabernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Node run state contract of clabernetes (nodes.c9s.run spec.runState, condition Stopped and
// the c9s.run/restart-requested annotation). Stop, start and restart are merge patches on the
// primary Node of a pod; the controllers own the Deployment and the Pod.
const (
	nodeRunStateRunning = "Running"
	nodeRunStateStopped = "Stopped"
	// nodeConditionStopped is True once a stopped Node has no Pod left, and False with reason
	// Stopping while its Pod terminates or Running otherwise.
	nodeConditionStopped = "Stopped"
	// restartRequestedAnnotation asks the Node controller to replace the Pod once per new
	// token. status.observedRestartToken reports the token of the running Pod.
	restartRequestedAnnotation = "c9s.run/restart-requested"
	nodeReadinessReady         = "ready"
	// nodeConditionContainersReady is True once the node's device containers are ready.
	nodeConditionContainersReady = "ContainersReady"
)

func (r *Runtime) Start(ctx context.Context, req clablabruntime.NodeRequest) error {
	return r.setNodesRunState(ctx, req, nodeRunStateRunning)
}

func (r *Runtime) Stop(ctx context.Context, req clablabruntime.NodeRequest) error {
	return r.setNodesRunState(ctx, req, nodeRunStateStopped)
}

// Restart replaces the Pod of every requested node once, through a new restart token on its
// primary Node. A stopped node is started with the same patch.
func (r *Runtime) Restart(ctx context.Context, req clablabruntime.NodeRequest) error {
	r, connErr := r.connect(ctx, req.Identity)
	if connErr != nil {
		return connErr
	}

	namespace, primaries, err := r.targetPrimaryNodes(ctx, req)
	if err != nil {
		return err
	}

	token := newRestartToken()
	patch := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{restartRequestedAnnotation: token},
		},
	}
	stoppedPatch := map[string]any{
		"metadata": patch["metadata"],
		"spec":     map[string]any{"runState": nodeRunStateRunning},
	}

	for _, node := range primaries {
		nodePatch := patch
		if nodeRunState(node) == nodeRunStateStopped {
			nodePatch = stoppedPatch
		}
		if _, err := r.patchNode(ctx, namespace, node.GetName(), nodePatch); err != nil {
			return fmt.Errorf("failed to restart c9s node %s/%s/%s: %w",
				namespace, req.Name, node.GetName(), err)
		}
	}

	return r.waitNodes(ctx, req, namespace, primaries, "restart",
		func(node *unstructured.Unstructured, stoppedPeers func() []string) (bool, string) {
			observed, _, _ := unstructured.NestedString(
				node.Object, "status", "observedRestartToken")
			readiness := nodeReadiness(node)
			describe := fmt.Sprintf("observed restart token %q (want %q), readiness %q",
				observed, token, readiness)
			if observed != token {
				return false, describe
			}

			return readiness == nodeReadinessReady ||
				waitsOnlyForStoppedPeers(node, stoppedPeers()), describe
		})
}

// setNodesRunState patches spec.runState of the requested nodes' primary Nodes and waits until
// the controller reports the new state: condition Stopped True for a stop, and Stopped False
// with readiness "ready" for a start.
func (r *Runtime) setNodesRunState(
	ctx context.Context,
	req clablabruntime.NodeRequest,
	runState string,
) error {
	r, connErr := r.connect(ctx, req.Identity)
	if connErr != nil {
		return connErr
	}

	namespace, primaries, err := r.targetPrimaryNodes(ctx, req)
	if err != nil {
		return err
	}

	generations := make(map[string]int64, len(primaries))
	for idx, node := range primaries {
		patched, err := r.patchNode(ctx, namespace, node.GetName(), runStatePatch(runState))
		if err != nil {
			return fmt.Errorf("failed to set c9s node %s/%s/%s run state to %s: %w",
				namespace, req.Name, node.GetName(), runState, err)
		}
		primaries[idx] = patched
		generations[patched.GetName()] = patched.GetGeneration()
	}

	operation := "start"
	if runState == nodeRunStateStopped {
		operation = "stop"
	}

	return r.waitNodes(ctx, req, namespace, primaries, operation,
		func(node *unstructured.Unstructured, stoppedPeers func() []string) (bool, string) {
			reached, describe := nodeReachedRunState(node, runState, generations[node.GetName()])
			if reached || runState == nodeRunStateStopped ||
				!nodeConditionCurrent(node, generations[node.GetName()]) {
				return reached, describe
			}

			// a started node whose link peers are still stopped never becomes ready
			return waitsOnlyForStoppedPeers(node, stoppedPeers()), describe
		})
}

// runStatePatch is the merge patch that sets a Node's run state.
func runStatePatch(runState string) map[string]any {
	return map[string]any{"spec": map[string]any{"runState": runState}}
}

// nodeReachedRunState reports whether the controller observed the Node generation that carries
// the run state and reached it, and describes the observed state otherwise.
func nodeReachedRunState(
	node *unstructured.Unstructured,
	runState string,
	generation int64,
) (bool, string) {
	condition := nodeCondition(node, nodeConditionStopped)
	readiness := nodeReadiness(node)

	describe := fmt.Sprintf("readiness %q, no %s condition", readiness, nodeConditionStopped)
	if condition != nil {
		describe = fmt.Sprintf("readiness %q, condition %s=%s (%s): %s", readiness,
			nodeConditionStopped, condition.status, condition.reason, condition.message)
	}

	if !nodeConditionCurrent(node, generation) {
		return false, describe
	}

	switch runState {
	case nodeRunStateStopped:
		return condition != nil && condition.status == string(metav1.ConditionTrue), describe
	default:
		stopped := condition != nil && condition.status == string(metav1.ConditionTrue)
		return !stopped && readiness == nodeReadinessReady, describe
	}
}

// nodeConditionCurrent reports whether the controller's Stopped condition describes at least the
// given Node generation, or the Node has no such condition yet.
func nodeConditionCurrent(node *unstructured.Unstructured, generation int64) bool {
	condition := nodeCondition(node, nodeConditionStopped)

	return condition == nil || condition.observedGeneration >= generation
}

// targetPrimaryNodes resolves the requested nodes (every node when none are given) to the
// primary Nodes that own their pods, once each. The primary Node governs the pod it shares with
// the nodes grouped onto it (network-mode container:<primary>).
func (r *Runtime) targetPrimaryNodes(
	ctx context.Context,
	req clablabruntime.NodeRequest,
) (string, []*unstructured.Unstructured, error) {
	targets, namespace, nodes, err := r.targetNodeResources(ctx, req)
	if err != nil {
		return "", nil, err
	}

	networkModes := make(map[string]string, len(nodes))
	byName := make(map[string]*unstructured.Unstructured, len(nodes))
	for name, node := range nodes {
		networkModes[name] = nodeNetworkMode(node)
		byName[name] = node
	}

	primaries := make([]*unstructured.Unstructured, 0, len(targets))
	seen := map[string]struct{}{}
	for _, target := range targets {
		primary := resolvePrimaryNode(target, networkModes)
		if _, ok := seen[primary]; ok {
			continue
		}
		seen[primary] = struct{}{}

		node, ok := byName[primary]
		if !ok {
			return "", nil, fmt.Errorf("primary node %q of node %q was not found in lab %s/%s",
				primary, target, namespace, req.Name)
		}
		primaries = append(primaries, node)
	}

	return namespace, primaries, nil
}

func (r *Runtime) targetNodes(
	ctx context.Context,
	req clablabruntime.NodeRequest,
) ([]string, string, error) {
	targets, namespace, _, err := r.targetNodeResources(ctx, req)

	return targets, namespace, err
}

// targetNodeResources resolves the requested node names (as the topology or the cluster spells
// them) against the lab's Node resources. No names select every node.
func (r *Runtime) targetNodeResources(
	ctx context.Context,
	req clablabruntime.NodeRequest,
) ([]string, string, map[string]*unstructured.Unstructured, error) {
	if req.Name == "" {
		return nil, "", nil, fmt.Errorf("topology name is required")
	}

	namespace, err := r.namespaceForLab(req.Name, req.Namespace)
	if err != nil {
		return nil, "", nil, err
	}
	list, err := r.nodesForTopology(ctx, req.Name, namespace)
	if err != nil {
		return nil, "", nil, err
	}

	known := make(map[string]*unstructured.Unstructured, len(list.Items))
	for idx := range list.Items {
		known[list.Items[idx].GetName()] = &list.Items[idx]
	}
	if len(known) == 0 {
		return nil, "", nil, fmt.Errorf("c9s lab %s/%s has no Node resources", namespace, req.Name)
	}

	var targets []string
	if len(req.Nodes) == 0 {
		targets = make([]string, 0, len(known))
		for nodeName := range known {
			targets = append(targets, nodeName)
		}
		sort.Strings(targets)

		return targets, namespace, known, nil
	}

	for _, nodeName := range req.Nodes {
		resolved, ok := resolveKnownNodeName(known, nodeName)
		if !ok {
			return nil, "", nil, fmt.Errorf("node %q was not found in topology %s/%s",
				nodeName, namespace, req.Name)
		}
		targets = append(targets, resolved)
	}

	return targets, namespace, known, nil
}

func (r *Runtime) patchNode(
	ctx context.Context,
	namespace,
	name string,
	patch map[string]any,
) (*unstructured.Unstructured, error) {
	data, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	return r.client.Resource(nodeGVR).Namespace(namespace).
		Patch(ctx, name, types.MergePatchType, data, metav1.PatchOptions{})
}

// waitNodes polls the Nodes until done reports every one of them finished, within the request
// timeout. A timeout names each unfinished node with its observed state.
func (r *Runtime) waitNodes(
	ctx context.Context,
	req clablabruntime.NodeRequest,
	namespace string,
	nodes []*unstructured.Unstructured,
	operation string,
	done func(node *unstructured.Unstructured, stoppedPeers func() []string) (bool, string),
) error {
	waitCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(req.Timeout))
	defer cancel()

	pending := make(map[string]string, len(nodes))
	for _, node := range nodes {
		pending[node.GetName()] = "not observed yet"
	}

	err := wait.PollUntilContextCancel(waitCtx, r.nodePollInterval(), true,
		func(ctx context.Context) (bool, error) {
			// the lab's run states are read at most once per poll, and only for a node that is
			// up but not ready
			var states *labRunStates
			for name := range pending {
				node, err := r.client.Resource(nodeGVR).Namespace(namespace).
					Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					return false, fmt.Errorf("failed to get c9s node %s/%s/%s: %w",
						namespace, req.Name, name, err)
				}
				stoppedPeers := func() []string {
					if states == nil {
						loaded, err := r.labRunStates(ctx, req.Name, namespace)
						if err != nil {
							return nil
						}
						states = loaded
					}

					return states.stoppedPeers[name]
				}
				finished, state := done(node, stoppedPeers)
				if finished {
					delete(pending, name)
					continue
				}
				pending[name] = state
			}

			return len(pending) == 0, nil
		})
	if err == nil {
		return nil
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		return err
	}

	names := make([]string, 0, len(pending))
	for name := range pending {
		names = append(names, name)
	}
	sort.Strings(names)

	details := make([]string, 0, len(names))
	for _, name := range names {
		details = append(details, fmt.Sprintf("%s: %s", name, pending[name]))
	}

	return fmt.Errorf("timed out waiting for the %s of c9s nodes in lab %s/%s (%s): %w",
		operation, namespace, req.Name, strings.Join(details, "; "), err)
}

func (r *Runtime) nodePollInterval() time.Duration {
	if r.pollInterval > 0 {
		return r.pollInterval
	}

	return pollInterval
}

// newRestartToken returns an opaque token that differs from every earlier one.
func newRestartToken() string {
	return time.Now().UTC().Format("20060102T150405.000000000Z")
}

func nodeRunState(node *unstructured.Unstructured) string {
	runState, _, _ := unstructured.NestedString(node.Object, "spec", "runState")
	if runState == "" {
		return nodeRunStateRunning
	}

	return runState
}

func nodeReadiness(node *unstructured.Unstructured) string {
	readiness, _, _ := unstructured.NestedString(node.Object, "status", "readiness")

	return readiness
}

type nodeStatusCondition struct {
	status             string
	reason             string
	message            string
	observedGeneration int64
}

func nodeCondition(node *unstructured.Unstructured, conditionType string) *nodeStatusCondition {
	conditions, _, _ := unstructured.NestedSlice(node.Object, "status", "conditions")
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if !ok || condition["type"] != conditionType {
			continue
		}

		result := &nodeStatusCondition{
			status:             fmt.Sprint(condition["status"]),
			observedGeneration: int64(intField(condition["observedGeneration"])),
		}
		result.reason, _ = condition["reason"].(string)
		result.message, _ = condition["message"].(string)

		return result
	}

	return nil
}

// labRunStates is the run state view of a lab: its Nodes and, per node, the link peers whose
// pod is stopped. clabernetes keeps a node's connectivity not ready while a link peer is
// stopped (its wire has no far end), so waits accept a node whose containers are ready and
// whose only missing peers are stopped ones.
type labRunStates struct {
	nodes        map[string]*unstructured.Unstructured
	networkModes map[string]string
	stoppedPeers map[string][]string
}

func (r *Runtime) labRunStates(
	ctx context.Context,
	lab,
	namespace string,
) (*labRunStates, error) {
	nodeList, err := r.nodesForTopology(ctx, lab, namespace)
	if err != nil {
		return nil, err
	}
	links, err := r.primitiveResourcesForTopology(ctx, linkGVR, lab, namespace)
	if err != nil {
		return nil, err
	}

	states := &labRunStates{
		nodes:        make(map[string]*unstructured.Unstructured, len(nodeList.Items)),
		networkModes: make(map[string]string, len(nodeList.Items)),
		stoppedPeers: map[string][]string{},
	}
	for idx := range nodeList.Items {
		node := &nodeList.Items[idx]
		states.nodes[node.GetName()] = node
		states.networkModes[node.GetName()] = nodeNetworkMode(node)
	}

	for idx := range links.Items {
		a, _, _ := unstructured.NestedString(
			links.Items[idx].Object,
			"spec",
			"endpointA",
			"nodeName",
		)
		b, _, _ := unstructured.NestedString(
			links.Items[idx].Object,
			"spec",
			"endpointB",
			"nodeName",
		)
		if states.stopped(b) {
			states.stoppedPeers[a] = append(states.stoppedPeers[a], b)
		}
		if states.stopped(a) {
			states.stoppedPeers[b] = append(states.stoppedPeers[b], a)
		}
	}

	return states, nil
}

// stopped reports whether a node's pod is stopped or stopping: the run state of the node's
// primary governs the pod it runs in.
func (s *labRunStates) stopped(nodeName string) bool {
	primary := s.nodes[resolvePrimaryNode(nodeName, s.networkModes)]

	return primary != nil && nodeRunState(primary) == nodeRunStateStopped
}

// waitsOnlyForStoppedPeers reports whether a running node that is not ready has ready containers
// and at least one stopped link peer, the state clabernetes reports for such a node.
func waitsOnlyForStoppedPeers(node *unstructured.Unstructured, stoppedPeers []string) bool {
	if len(stoppedPeers) == 0 || nodeRunState(node) == nodeRunStateStopped {
		return false
	}
	containers := nodeCondition(node, nodeConditionContainersReady)

	return containers != nil && containers.status == string(metav1.ConditionTrue)
}

// settledNode reports whether a deploy or start no longer waits for a node: it is ready, it is
// stopped on purpose, or it is up and only waits for stopped link peers. The reason describes a
// settled node that is not ready.
func (s *labRunStates) settledNode(name string) (bool, string) {
	node := s.nodes[name]
	if node == nil {
		return false, ""
	}
	if s.stopped(name) {
		return true, "stopped"
	}
	if nodeReadiness(node) == nodeReadinessReady {
		return true, ""
	}
	if peers := s.stoppedPeers[name]; waitsOnlyForStoppedPeers(node, peers) {
		sorted := append([]string(nil), peers...)
		sort.Strings(sorted)

		return true, "link peers stopped: " + strings.Join(sorted, ", ")
	}

	return false, ""
}
