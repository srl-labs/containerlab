package events

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

func Stream(ctx context.Context, opts Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	clab, err := clabcore.NewContainerLab(opts.ClabOptions...)
	if err != nil {
		return err
	}

	runtime, ok := clab.Runtimes[opts.Runtime]
	if !ok {
		return fmt.Errorf("runtime %q is not initialized", opts.Runtime)
	}

	printer, err := newFormatter(opts.Format, opts.writer())
	if err != nil {
		return err
	}

	eventCh := make(chan aggregatedEvent, 128)
	registry := newNetlinkRegistry(ctx, eventCh)

	containers, err := clab.ListContainers(ctx, clabcore.WithListclabLabelExists())
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for idx := range containers {
		container := containers[idx]
		if !isRunningContainer(&container) {
			continue
		}

		registry.Start(&container)
	}

	streamOpts := clabruntime.EventStreamOptions{
		Labels: map[string]string{
			clabconstants.Containerlab: "",
		},
	}

	runtimeEvents, runtimeErrs, err := runtime.StreamEvents(ctx, streamOpts)
	if err != nil {
		return fmt.Errorf("failed to stream events for runtime %q: %w", opts.Runtime, err)
	}

	errCh := make(chan error, 1)
	go forwardRuntimeEvents(ctx, runtime, registry, runtimeEvents, runtimeErrs, eventCh, errCh)

	runtimeErrors := errCh

	for {
		select {
		case ev := <-eventCh:
			if err := printer(ev); err != nil {
				log.Debugf("failed to write event: %v", err)
			}
		case err, ok := <-runtimeErrors:
			if !ok {
				runtimeErrors = nil

				continue
			}

			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func forwardRuntimeEvents(
	ctx context.Context,
	runtime clabruntime.ContainerRuntime,
	registry *netlinkRegistry,
	runtimeEvents <-chan clabruntime.ContainerEvent,
	runtimeErrs <-chan error,
	eventSink chan<- aggregatedEvent,
	errSink chan<- error,
) {
	defer close(errSink)

	sendErr := func(err error) {
		select {
		case errSink <- err:
		case <-ctx.Done():
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-runtimeErrs:
			if !ok {
				return
			}

			if err != nil && !errors.Is(err, context.Canceled) {
				sendErr(err)

				return
			}
		case ev, ok := <-runtimeEvents:
			if !ok {
				return
			}

			registry.HandleContainerEvent(runtime, ev)

			aggregated := aggregatedEventFromContainerEvent(ev)
			select {
			case eventSink <- aggregated:
			case <-ctx.Done():
				return
			}
		}
	}
}

func aggregatedEventFromContainerEvent(ev clabruntime.ContainerEvent) aggregatedEvent {
	ts := ev.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	attributes := cloneStringMap(ev.Attributes)

	actorFullID := ev.ActorFullID
	if actorFullID == "" {
		actorFullID = ev.ActorID
	}

	actorName := ev.ActorName
	if actorName == "" && attributes != nil {
		actorName = attributes["name"]
	}

	short := ev.ActorID
	if short == "" {
		short = actorFullID
	}

	action := strings.ToLower(ev.Action)
	if action == "" {
		action = ev.Action
	}

	eventType := strings.ToLower(ev.Type)
	if eventType == "" {
		eventType = ev.Type
	}

	return aggregatedEvent{
		Timestamp:   ts,
		Type:        eventType,
		Action:      action,
		ActorID:     shortID(short),
		ActorName:   actorName,
		ActorFullID: actorFullID,
		Attributes:  attributes,
	}
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}

	result := make(map[string]string, len(input))
	for k, v := range input {
		result[k] = v
	}

	return result
}

func isRunningContainer(container *clabruntime.GenericContainer) bool {
	if container == nil {
		return false
	}

	return strings.EqualFold(container.State, "running")
}
