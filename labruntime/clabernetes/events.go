package clabernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
)

func (r *Runtime) StreamEvents(
	ctx context.Context,
	req clablabruntime.EventStreamRequest,
) (<-chan clablabruntime.Event, <-chan error, error) {
	r, connErr := r.connect(ctx, req.Identity)
	if connErr != nil {
		return nil, nil, connErr
	}

	scope, err := r.eventScopeFor(req)
	if err != nil {
		return nil, nil, err
	}

	events := make(chan clablabruntime.Event, 128)
	errs := make(chan error, 3)

	if req.IncludeInitialState {
		go r.emitInitialEvents(ctx, scope, events, errs)
	}

	// Interface state events are always reported, like the netlink-backed events of the local
	// runtimes; statistics only on request. Both come from the same exec per node.
	go r.pollInterfaces(ctx, scope, interfacePollOptions{
		interval:            req.StatsInterval,
		includeStats:        req.IncludeInterfaceStats,
		includeInitialState: req.IncludeInitialState,
	}, events)

	go r.watchTopologies(ctx, scope, events, errs)
	go r.watchNodes(ctx, scope, events, errs)
	go r.watchPods(ctx, scope, events, errs)

	return events, errs, nil
}

// eventScope is the namespace and, optionally, the single lab an event stream covers.
type eventScope struct {
	namespace string
	lab       string
}

func (r *Runtime) eventScopeFor(req clablabruntime.EventStreamRequest) (eventScope, error) {
	if req.Name != "" {
		namespace, err := r.namespaceForLab(req.Name, req.Namespace)
		if err != nil {
			return eventScope{}, err
		}

		return eventScope{namespace: namespace, lab: req.Name}, nil
	}

	return eventScope{namespace: r.listNamespace(req.Namespace, req.AllNamespaces)}, nil
}

// labelSelector selects the lab-owned resources of the scope.
func (s eventScope) labelSelector() string {
	if s.lab != "" {
		return labels.Set{labelTopologyOwner: s.lab}.String()
	}

	return labelTopologyOwner
}

// topologyListOptions selects the Topology resources of the scope.
func (s eventScope) topologyListOptions() metav1.ListOptions {
	if s.lab != "" {
		return metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", s.lab).String(),
		}
	}

	return metav1.ListOptions{}
}

// labStates lists the labs of the scope.
func (r *Runtime) labStates(
	ctx context.Context,
	scope eventScope,
) ([]*clablabruntime.LabState, error) {
	if scope.lab != "" {
		state, err := r.Inspect(ctx, clablabruntime.InspectRequest{
			Name:      scope.lab,
			Namespace: scope.namespace,
		})
		if err != nil {
			// A lab that is not deployed yet has no state; its resources show up through
			// the watches once it is.
			var notFound *labNotFoundError
			if errors.As(err, &notFound) {
				return nil, nil
			}

			return nil, err
		}

		return []*clablabruntime.LabState{state}, nil
	}

	return r.List(ctx, clablabruntime.ListRequest{
		Namespace:     scope.namespace,
		AllNamespaces: scope.namespace == metav1.NamespaceAll,
	})
}

func (r *Runtime) watchNodes(
	ctx context.Context,
	scope eventScope,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	resource := r.client.Resource(nodeGVR).Namespace(scope.namespace)

	for {
		watcher, err := resource.Watch(ctx, metav1.ListOptions{
			LabelSelector: scope.labelSelector(),
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			sendEventError(ctx, errSink, fmt.Errorf("failed to watch c9s nodes: %w", err))
			return
		}

		if !r.forwardNodeWatch(ctx, watcher, eventSink, errSink) {
			return
		}

		if !sleepContext(ctx, pollInterval) {
			return
		}
	}
}

func (r *Runtime) forwardNodeWatch(
	ctx context.Context,
	watcher watch.Interface,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) bool {
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case ev, ok := <-watcher.ResultChan():
			if !ok {
				log.Debug("c9s node watch closed, reconnecting")
				return true
			}
			if ev.Type == watch.Error {
				sendEventError(ctx, errSink, fmt.Errorf("c9s node watch returned an error"))
				return false
			}

			node, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}

			labName := node.GetLabels()[labelTopologyOwner]
			if labName == "" {
				continue
			}
			readiness, _, _ := unstructured.NestedString(node.Object, "status", "readiness")
			r.sendEvent(ctx, eventSink, clablabruntime.Event{
				Timestamp: time.Now(),
				Type:      "container",
				Action:    strings.ToLower(string(ev.Type)),
				ActorID:   fmt.Sprintf("%s/%s/%s", node.GetNamespace(), labName, node.GetName()),
				ActorName: fmt.Sprintf("%s-%s", labName, node.GetName()),
				Attributes: map[string]string{
					"namespace": node.GetNamespace(),
					"lab":       labName,
					"node":      node.GetName(),
					"state":     readiness,
				},
			})
		}
	}
}

func (r *Runtime) emitInitialEvents(
	ctx context.Context,
	scope eventScope,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	states, err := r.labStates(ctx, scope)
	if err != nil {
		sendEventError(ctx, errSink, err)
		return
	}

	for _, state := range states {
		for _, node := range state.Nodes {
			action := node.State
			if node.Ready {
				action = "running"
			}
			if action == "" {
				action = state.State
			}
			r.sendEvent(ctx, eventSink, clablabruntime.Event{
				Timestamp: time.Now(),
				Type:      "container",
				Action:    action,
				ActorID:   fmt.Sprintf("%s/%s/%s", state.Namespace, state.Name, node.Name),
				ActorName: fmt.Sprintf("%s-%s", state.Name, node.Name),
				Attributes: map[string]string{
					"namespace": state.Namespace,
					"lab":       state.Name,
					"node":      node.Name,
					"state":     node.State,
				},
			})
		}
	}
}

func (r *Runtime) watchTopologies(
	ctx context.Context,
	scope eventScope,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	resource := r.client.Resource(topologyGVR).Namespace(scope.namespace)

	for {
		watcher, err := resource.Watch(ctx, scope.topologyListOptions())
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			sendEventError(
				ctx,
				errSink,
				fmt.Errorf("failed to watch clabernetes topologies: %w", err),
			)
			return
		}

		if !r.forwardTopologyWatch(ctx, scope.namespace, watcher, eventSink, errSink) {
			return
		}

		if !sleepContext(ctx, pollInterval) {
			return
		}
	}
}

func (r *Runtime) forwardTopologyWatch(
	ctx context.Context,
	namespace string,
	watcher watch.Interface,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) bool {
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case ev, ok := <-watcher.ResultChan():
			if !ok {
				log.Debug("C9s topology watch closed, reconnecting")
				return true
			}
			if ev.Type == watch.Error {
				sendEventError(
					ctx,
					errSink,
					fmt.Errorf("clabernetes topology watch returned an error"),
				)
				return false
			}

			obj, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}

			state := stateFromTopology(obj, namespace)
			r.sendEvent(ctx, eventSink, clablabruntime.Event{
				Timestamp: time.Now(),
				Type:      "topology",
				Action:    strings.ToLower(string(ev.Type)),
				ActorID:   fmt.Sprintf("%s/%s", state.Namespace, state.Name),
				ActorName: state.Name,
				Attributes: map[string]string{
					"namespace": state.Namespace,
					"lab":       state.Name,
					"state":     state.State,
					"ready":     fmt.Sprintf("%t", state.Ready),
				},
			})
		}
	}
}

func (r *Runtime) watchPods(
	ctx context.Context,
	scope eventScope,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	for {
		watcher, err := r.kubeClient.CoreV1().Pods(scope.namespace).Watch(ctx, metav1.ListOptions{
			LabelSelector: scope.labelSelector(),
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			sendEventError(ctx, errSink, fmt.Errorf(
				"failed to watch clabernetes pods: %w", err))
			return
		}

		if !r.forwardPodWatch(ctx, watcher, eventSink, errSink) {
			return
		}

		if !sleepContext(ctx, pollInterval) {
			return
		}
	}
}

func (r *Runtime) forwardPodWatch(
	ctx context.Context,
	watcher watch.Interface,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) bool {
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case ev, ok := <-watcher.ResultChan():
			if !ok {
				log.Debug("C9s pod watch closed, reconnecting")
				return true
			}
			if ev.Type == watch.Error {
				sendEventError(ctx, errSink, fmt.Errorf(
					"clabernetes pod watch returned an error"))
				return false
			}

			pod, ok := ev.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			labName := pod.Labels[labelTopologyOwner]
			nodeName := pod.Labels[labelTopologyNode]
			if labName == "" || nodeName == "" {
				continue
			}

			r.sendEvent(ctx, eventSink, clablabruntime.Event{
				Timestamp:   time.Now(),
				Type:        "container",
				Action:      strings.ToLower(string(ev.Type)),
				ActorID:     fmt.Sprintf("%s/%s/%s", pod.Namespace, labName, nodeName),
				ActorName:   fmt.Sprintf("%s-%s", labName, nodeName),
				ActorFullID: pod.Name,
				Attributes: map[string]string{
					"namespace": pod.Namespace,
					"lab":       labName,
					"node":      nodeName,
					"pod":       pod.Name,
					"phase":     string(pod.Status.Phase),
					"pod_ip":    pod.Status.PodIP,
				},
			})
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (r *Runtime) sendEvent(
	ctx context.Context,
	eventSink chan<- clablabruntime.Event,
	event clablabruntime.Event,
) {
	select {
	case eventSink <- event:
	case <-ctx.Done():
	}
}

func sendEventError(ctx context.Context, errSink chan<- error, err error) {
	select {
	case errSink <- err:
	case <-ctx.Done():
	}
}
