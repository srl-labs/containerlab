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
	clabtypes "github.com/srl-labs/containerlab/types"
)

// Stream subscribes to the selected runtime and netlink sources and forwards
// aggregated events to the configured writer.
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
	registry := newNetlinkRegistry(ctx, eventCh, opts.IncludeInitialState)

	containers, err := clab.ListContainers(ctx, clabcore.WithListclabLabelExists())
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if opts.IncludeInitialState {
		go emitContainerSnapshots(ctx, containers, eventCh)
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

			aggregated := aggregatedEventFromContainerEvent(ctx, runtime, ev)
			select {
			case eventSink <- aggregated:
			case <-ctx.Done():
				return
			}
		}
	}
}

func aggregatedEventFromContainerEvent(
	ctx context.Context,
	runtime clabruntime.ContainerRuntime,
	ev clabruntime.ContainerEvent,
) aggregatedEvent {
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

	attributes = ensureMgmtIPAttributes(ctx, runtime, attributes, actorFullID, actorName)

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

func ensureMgmtIPAttributes(
	ctx context.Context,
	runtime clabruntime.ContainerRuntime,
	attributes map[string]string,
	actorFullID, actorName string,
) map[string]string {
	if runtime == nil {
		return attributes
	}

	hasIPv4 := attributes != nil && attributes["mgmt_ipv4"] != ""
	hasIPv6 := attributes != nil && attributes["mgmt_ipv6"] != ""

	if hasIPv4 && hasIPv6 {
		return attributes
	}

	filters := make([]*clabtypes.GenericFilter, 0, 2)

	if actorName == "" && attributes != nil {
		actorName = attributes["name"]
	}

	if actorName != "" {
		filters = append(filters, &clabtypes.GenericFilter{FilterType: "name", Match: actorName})
	}

	if actorFullID != "" {
		filters = append(filters, &clabtypes.GenericFilter{FilterType: "id", Match: actorFullID})
	}

	if len(filters) == 0 {
		return attributes
	}

	containers, err := runtime.ListContainers(ctx, filters)
	if err != nil {
		log.Debugf("failed to resolve container for event: %v", err)

		return attributes
	}

	container := selectContainerForEvent(containers, actorFullID, actorName)
	if container == nil {
		return attributes
	}

	if attributes == nil {
		attributes = make(map[string]string)
	}

	if !hasIPv4 {
		if ipv4 := container.GetContainerIPv4(); ipv4 != "" && ipv4 != clabconstants.NotApplicable {
			attributes["mgmt_ipv4"] = ipv4
		}
	}

	if !hasIPv6 {
		if ipv6 := container.GetContainerIPv6(); ipv6 != "" && ipv6 != clabconstants.NotApplicable {
			attributes["mgmt_ipv6"] = ipv6
		}
	}

	if len(attributes) == 0 {
		return nil
	}

	return attributes
}

func selectContainerForEvent(
	containers []clabruntime.GenericContainer,
	actorFullID, actorName string,
) *clabruntime.GenericContainer {
	if len(containers) == 0 {
		return nil
	}

	if actorFullID != "" {
		for idx := range containers {
			container := &containers[idx]

			switch {
			case container.ID == actorFullID:
				return container
			case strings.HasPrefix(container.ID, actorFullID):
				return container
			case container.ShortID == actorFullID:
				return container
			}
		}
	}

	if actorName != "" {
		for idx := range containers {
			container := &containers[idx]

			for _, name := range container.Names {
				if name == actorName {
					return container
				}
			}
		}
	}

	return &containers[0]
}

func emitContainerSnapshots(
	ctx context.Context,
	containers []clabruntime.GenericContainer,
	sink chan<- aggregatedEvent,
) {
	for idx := range containers {
		container := containers[idx]
		if !isRunningContainer(&container) {
			continue
		}

		event := aggregatedEventFromContainerSnapshot(&container)
		if event.ActorID == "" && event.ActorName == "" {
			continue
		}

		select {
		case sink <- event:
		case <-ctx.Done():
			return
		}
	}
}

func aggregatedEventFromContainerSnapshot(
	container *clabruntime.GenericContainer,
) aggregatedEvent {
	if container == nil {
		return aggregatedEvent{}
	}

	state := strings.ToLower(container.State)

	short := container.ShortID
	if short == "" {
		short = shortID(container.ID)
	}

	attributes := cloneStringMap(container.Labels)
	if attributes == nil {
		attributes = make(map[string]string)
	}

	if _, ok := attributes["origin"]; !ok {
		attributes["origin"] = "snapshot"
	}

	if container.Image != "" {
		attributes["image"] = container.Image
	}

	if container.Status != "" {
		attributes["status"] = container.Status
	}

	if state != "" {
		attributes["state"] = state
	}

	if container.NetworkName != "" {
		attributes["network"] = container.NetworkName
	}

	if container.NetworkSettings.IPv4addr != "" {
		attributes["mgmt_ipv4"] = container.GetContainerIPv4()
	}

	if container.NetworkSettings.IPv6addr != "" {
		attributes["mgmt_ipv6"] = container.GetContainerIPv6()
	}

	if len(attributes) == 0 {
		attributes = nil
	}

	actorName := firstContainerName(container)

	action := state
	if action == "" {
		action = "snapshot"
	}

	return aggregatedEvent{
		Timestamp:   time.Now(),
		Type:        "container",
		Action:      action,
		ActorID:     shortID(short),
		ActorName:   actorName,
		ActorFullID: container.ID,
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
