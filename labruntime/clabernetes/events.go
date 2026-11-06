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
	errs := make(chan error, 4)
	// changed carries the labs whose Topology, Nodes or pods changed; the container tracker
	// reads them again and reports what changed for their nodes.
	changed := make(chan labRef, 256)

	go r.trackContainers(ctx, scope, req.IncludeInitialState, changed, events, errs)

	// Interface state events are always reported, like the netlink-backed events of the local
	// runtimes; statistics only on request. Both come from the same exec per node.
	go r.pollInterfaces(ctx, scope, interfacePollOptions{
		interval:            req.StatsInterval,
		includeStats:        req.IncludeInterfaceStats,
		includeInitialState: req.IncludeInitialState,
	}, events)

	go r.watchTopologies(ctx, scope, changed, events, errs)
	go r.watchNodes(ctx, scope, changed, errs)
	go r.watchPods(ctx, scope, changed, errs)

	return events, errs, nil
}

// labRef names a lab in a namespace.
type labRef struct {
	namespace string
	lab       string
}

func sendChangedLab(ctx context.Context, changed chan<- labRef, ref labRef) {
	if ref.lab == "" {
		return
	}

	select {
	case changed <- ref:
	case <-ctx.Done():
	}
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
	changed chan<- labRef,
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

		if !r.forwardNodeWatch(ctx, watcher, changed, errSink) {
			return
		}

		if !sleepContext(ctx, pollInterval) {
			return
		}
	}
}

// forwardNodeWatch reports the lab of every changed Node to the container tracker.
func (r *Runtime) forwardNodeWatch(
	ctx context.Context,
	watcher watch.Interface,
	changed chan<- labRef,
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

			sendChangedLab(ctx, changed, labRef{
				namespace: node.GetNamespace(),
				lab:       node.GetLabels()[labelTopologyOwner],
			})
		}
	}
}

func (r *Runtime) watchTopologies(
	ctx context.Context,
	scope eventScope,
	changed chan<- labRef,
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

		if !r.forwardTopologyWatch(ctx, scope.namespace, watcher, changed, eventSink, errSink) {
			return
		}

		if !sleepContext(ctx, pollInterval) {
			return
		}
	}
}

// forwardTopologyWatch reports Topology changes as topology events, which only c9s has, and
// reports the lab to the container tracker.
func (r *Runtime) forwardTopologyWatch(
	ctx context.Context,
	namespace string,
	watcher watch.Interface,
	changed chan<- labRef,
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
			sendChangedLab(ctx, changed, labRef{namespace: state.Namespace, lab: state.Name})
		}
	}
}

func (r *Runtime) watchPods(
	ctx context.Context,
	scope eventScope,
	changed chan<- labRef,
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

		if !r.forwardPodWatch(ctx, watcher, changed, errSink) {
			return
		}

		if !sleepContext(ctx, pollInterval) {
			return
		}
	}
}

// forwardPodWatch reports the lab of every changed pod to the container tracker.
func (r *Runtime) forwardPodWatch(
	ctx context.Context,
	watcher watch.Interface,
	changed chan<- labRef,
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

			sendChangedLab(ctx, changed, labRef{
				namespace: pod.Namespace,
				lab:       pod.Labels[labelTopologyOwner],
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
