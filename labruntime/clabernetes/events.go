package clabernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

func (r *Runtime) StreamEvents(
	ctx context.Context,
	req clablabruntime.EventStreamRequest,
) (<-chan clablabruntime.Event, <-chan error, error) {
	events := make(chan clablabruntime.Event, 128)
	errs := make(chan error, 2)

	namespace := r.namespaceFor(req.Namespace)
	if req.AllNamespaces {
		namespace = metav1.NamespaceAll
	}

	if req.IncludeInitialState {
		go r.emitInitialEvents(ctx, namespace, events, errs)
	}

	if req.IncludeInterfaceStats {
		go r.pollInterfaceStats(ctx, namespace, req.StatsInterval, events)
	}

	go r.watchTopologies(ctx, namespace, events, errs)
	go r.watchPods(ctx, namespace, events, errs)

	return events, errs, nil
}

func (r *Runtime) emitInitialEvents(
	ctx context.Context,
	namespace string,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	states, err := r.List(ctx, clablabruntime.ListRequest{
		Namespace:     namespace,
		AllNamespaces: namespace == metav1.NamespaceAll,
	})
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
	namespace string,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	resource := r.client.Resource(topologyGVR).Namespace(namespace)

	for {
		watcher, err := resource.Watch(ctx, metav1.ListOptions{})
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			sendEventError(ctx, errSink, fmt.Errorf("failed to watch clabernetes topologies: %w", err))
			return
		}

		if !r.forwardTopologyWatch(ctx, namespace, watcher, eventSink, errSink) {
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
				log.Debug("clabernetes topology watch closed, reconnecting")
				return true
			}
			if ev.Type == watch.Error {
				sendEventError(ctx, errSink, fmt.Errorf("clabernetes topology watch returned an error"))
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
	namespace string,
	eventSink chan<- clablabruntime.Event,
	errSink chan<- error,
) {
	for {
		watcher, err := r.kubeClient.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
			LabelSelector: labelTopologyOwner,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			sendEventError(ctx, errSink, fmt.Errorf("failed to watch clabernetes pods: %w", err))
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
				log.Debug("clabernetes pod watch closed, reconnecting")
				return true
			}
			if ev.Type == watch.Error {
				sendEventError(ctx, errSink, fmt.Errorf("clabernetes pod watch returned an error"))
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
