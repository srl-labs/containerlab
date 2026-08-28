package clabernetes

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// The direct runtime has no image copy machinery: the kubelet pulls device images natively.
// Pull progress therefore comes from the kubelet's Pod events for the lab's device (and
// planning) pods.
type imagePullProgress struct {
	reportedPullStates map[string]struct{}
	listErrorReported  bool
}

var quotedImagePattern = regexp.MustCompile(`"([^"]+)"`)

func (p *imagePullProgress) observe(
	ctx context.Context,
	r *Runtime,
	namespace string,
	state *clablabruntime.LabState,
) {
	if state == nil {
		return
	}

	pods, err := r.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			labelApp:           clabernetesAppValue,
			labelTopologyOwner: state.Name,
		}.String(),
	})
	if err != nil {
		p.reportListError(err)

		return
	}

	nodeByPod := make(map[string]podPullContext, len(pods.Items))
	for idx := range pods.Items {
		pod := &pods.Items[idx]
		// Only the device application containers carry lab images; the init and connectivity
		// containers run the c9s runtime image and are infrastructure noise for a lab user.
		deviceImages := make(map[string]struct{}, len(pod.Spec.Containers))
		for _, container := range pod.Spec.Containers {
			deviceImages[container.Image] = struct{}{}
		}
		nodeByPod[pod.Name] = podPullContext{
			node:           pod.Labels[labelTopologyNode],
			kubernetesNode: pod.Spec.NodeName,
			deviceImages:   deviceImages,
		}
	}

	events, err := r.kubeClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.kind=Pod",
	})
	if err != nil {
		p.reportListError(err)

		return
	}
	p.listErrorReported = false

	sort.Slice(events.Items, func(i, j int) bool {
		left := eventOrderTime(&events.Items[i])
		right := eventOrderTime(&events.Items[j])

		return left.Before(&right)
	})

	if p.reportedPullStates == nil {
		p.reportedPullStates = map[string]struct{}{}
	}

	for idx := range events.Items {
		event := &events.Items[idx]
		pullContext, ok := nodeByPod[event.InvolvedObject.Name]
		if !ok {
			continue
		}

		image := quotedImageFromMessage(event.Message)
		if _, isDeviceImage := pullContext.deviceImages[image]; !isDeviceImage {
			continue
		}

		// A grouped or chassis pod repeats the same image across several containers; report
		// each pull state once per pod and image.
		stateKey := event.InvolvedObject.Name + "\x00" + image + "\x00" + event.Reason
		if _, reported := p.reportedPullStates[stateKey]; reported {
			continue
		}
		p.reportedPullStates[stateKey] = struct{}{}

		p.reportEvent(event, pullContext, image)
	}
}

type podPullContext struct {
	node           string
	kubernetesNode string
	deviceImages   map[string]struct{}
}

func eventOrderTime(event *corev1.Event) metav1.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp
	}
	if !event.EventTime.IsZero() {
		return metav1.Time{Time: event.EventTime.Time}
	}

	return event.FirstTimestamp
}

func (p *imagePullProgress) reportEvent(
	event *corev1.Event,
	pullContext podPullContext,
	image string,
) {
	switch event.Reason {
	case "Pulling":
		log.Info(
			"Pulling clabernetes node image",
			"node", pullContext.node,
			"image", image,
			"kubernetes-node", pullContext.kubernetesNode,
		)
	case "Pulled":
		if strings.Contains(event.Message, "already present") {
			log.Info(
				"Clabernetes node image already present on kubernetes node",
				"node", pullContext.node,
				"image", image,
				"kubernetes-node", pullContext.kubernetesNode,
			)

			return
		}
		log.Info(
			"Clabernetes node image pull completed",
			"node", pullContext.node,
			"image", image,
			"kubernetes-node", pullContext.kubernetesNode,
		)
	case "Failed", "BackOff", "ErrImagePull", "ImagePullBackOff":
		if !strings.Contains(event.Message, "image") {
			return
		}
		log.Warn(
			"Clabernetes node image pull is failing",
			"node", pullContext.node,
			"image", image,
			"kubernetes-node", pullContext.kubernetesNode,
			"reason", event.Reason,
			"message", strings.TrimSpace(event.Message),
		)
	}
}

func quotedImageFromMessage(message string) string {
	match := quotedImagePattern.FindStringSubmatch(message)
	if len(match) != 2 {
		return ""
	}

	return match[1]
}

func (p *imagePullProgress) reportListError(err error) {
	if p.listErrorReported {
		return
	}
	log.Debug("Unable to inspect clabernetes image pulls", "error", err)
	p.listErrorReported = true
}

// nodePhaseProgress reports each Node's controller-observed lifecycle phase transitions during
// a deploy wait: profile resolution, device planning, file preparation, connectivity, container
// startup, and finally the readiness probes. It also captures deterministic planning failures
// so a deploy can fail fast instead of waiting out the timeout.
type nodePhaseProgress struct {
	phases            map[string]string
	terminalFailures  map[string]string
	failureStreaks    map[string]int
	listErrorReported bool
}

// terminalFailureStreak is how many consecutive polls a deterministic plan failure must persist
// before the deploy aborts. Controllers replan asynchronously, so even a deterministic-looking
// condition needs a debounce against in-flight reconciliation.
const terminalFailureStreak = 3

// registryFailureStreak is the debounce for registry metadata failures (missing credentials,
// unknown image). They are usually configuration mistakes that would otherwise hang the deploy
// until timeout, but a registry blip must not abort a deploy, hence the longer window.
const registryFailureStreak = 15

// terminalFailure reports one deterministic per-node failure observed during the wait, or "".
func (p *nodePhaseProgress) terminalFailure() string {
	names := make([]string, 0, len(p.terminalFailures))
	for name := range p.terminalFailures {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		return fmt.Sprintf("node %s: %s", name, p.terminalFailures[name])
	}

	return ""
}

// nodeConditionPhases orders the Node conditions along the direct-runtime lifecycle; the first
// condition not yet True names the phase the node is in.
var nodeConditionPhases = []struct {
	conditionType string
	phase         string
}{
	{"NodeProfileResolved", "resolving profile"},
	{"PlanApplied", "planning device"},
	{"Prepared", "preparing device"},
	{"ConnectivityReady", "wiring connectivity"},
	{"ContainersReady", "starting containers"},
}

func (p *nodePhaseProgress) observe(
	ctx context.Context,
	r *Runtime,
	name,
	namespace string,
) {
	nodes, err := r.nodesForTopology(ctx, name, namespace)
	if err != nil {
		if !p.listErrorReported {
			log.Debug("Unable to inspect clabernetes node conditions", "error", err)
			p.listErrorReported = true
		}

		return
	}
	p.listErrorReported = false

	if p.phases == nil {
		p.phases = map[string]string{}
	}

	sort.Slice(nodes.Items, func(i, j int) bool {
		return nodes.Items[i].GetName() < nodes.Items[j].GetName()
	})

	for idx := range nodes.Items {
		node := &nodes.Items[idx]
		phase, detail := nodeLifecyclePhase(node)

		if p.terminalFailures == nil {
			p.terminalFailures = map[string]string{}
			p.failureStreaks = map[string]int{}
		}
		if failure, requiredStreak := nodePlanFailure(node); failure != "" {
			p.failureStreaks[node.GetName()]++
			if p.failureStreaks[node.GetName()] >= requiredStreak {
				p.terminalFailures[node.GetName()] = failure
			}
		} else {
			delete(p.failureStreaks, node.GetName())
			delete(p.terminalFailures, node.GetName())
		}

		if p.phases[node.GetName()] == phase {
			continue
		}
		p.phases[node.GetName()] = phase

		// Readiness transitions are reported by the readiness tracker; phases end here.
		if phase == "ready" {
			continue
		}

		fields := []any{"node", node.GetName(), "phase", phase}
		if detail != "" {
			fields = append(fields, "detail", detail)
		}
		log.Info("Clabernetes node progress", fields...)
	}
}

// terminalPlanReasons are PlanApplied failure reasons that are deterministic functions of the
// Node spec: the controller cannot converge without a spec change. PlanMissingInput is excluded
// because it also fires transiently while cluster inventory (Link acceptance, ConfigMaps) is
// still landing.
var terminalPlanReasons = map[string]struct{}{
	"PlanUnsupported":   {},
	"PlanInvalidInput":  {},
	"PlanInvariant":     {},
	"PlanSideEffect":    {},
	"PlanSerialization": {},
	// The API server rejected the rendered device Deployment; the controller cannot converge
	// without a spec change. DeploymentApplyFailed (transient apply errors) is deliberately
	// absent.
	"DeploymentInvalid": {},
}

// nodePlanFailure reports a deterministic device-planning failure for the node together with
// the debounce it must outlast, or "".
func nodePlanFailure(node *unstructured.Unstructured) (string, int) {
	conditions, _, _ := unstructured.NestedSlice(node.Object, "status", "conditions")
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if condition["type"] != "PlanApplied" || condition["status"] != "False" {
			continue
		}

		reason, _ := condition["reason"].(string)
		requiredStreak := 0
		switch {
		case strings.HasPrefix(reason, "OCIMetadata"):
			requiredStreak = registryFailureStreak
		default:
			if _, terminal := terminalPlanReasons[reason]; terminal {
				requiredStreak = terminalFailureStreak
			}
		}
		if requiredStreak == 0 {
			return "", 0
		}

		if message, ok := condition["message"].(string); ok && message != "" {
			return message, requiredStreak
		}

		return reason, requiredStreak
	}

	return "", 0
}

func nodeLifecyclePhase(node *unstructured.Unstructured) (string, string) {
	readiness, _, _ := unstructured.NestedString(node.Object, "status", "readiness")
	if readiness == "ready" {
		return "ready", ""
	}

	for _, candidate := range nodeConditionPhases {
		reason := conditionPendingReason(node, candidate.conditionType)
		if reason == "" {
			continue
		}

		detail := reason
		if reason == "waiting for "+candidate.conditionType {
			detail = ""
		}

		return candidate.phase, detail
	}

	return "waiting for device readiness", ""
}
