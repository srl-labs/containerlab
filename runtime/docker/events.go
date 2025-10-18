package docker

import (
	"context"
	"errors"
	"fmt"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func (d *DockerRuntime) StreamEvents(
	ctx context.Context,
	opts clabruntime.EventStreamOptions,
) (<-chan clabruntime.ContainerEvent, <-chan error, error) {
	events := make(chan clabruntime.ContainerEvent, 128)
	errs := make(chan error, 1)

	go d.streamDockerEvents(ctx, opts, events, errs)

	return events, errs, nil
}

func (d *DockerRuntime) streamDockerEvents(
	ctx context.Context,
	opts clabruntime.EventStreamOptions,
	eventSink chan<- clabruntime.ContainerEvent,
	errSink chan<- error,
) {
	defer close(eventSink)
	defer close(errSink)

	filtersArgs := filters.NewArgs()
	for key, value := range opts.Labels {
		if value == "" {
			filtersArgs.Add("label", key)
		} else {
			filtersArgs.Add("label", fmt.Sprintf("%s=%s", key, value))
		}
	}

	messages, errs := d.Client.Events(ctx, dockerTypes.EventsOptions{Filters: filtersArgs})

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errs:
			if !ok {
				return
			}

			if err != nil && !errors.Is(err, context.Canceled) {
				errSink <- err

				return
			}
		case msg, ok := <-messages:
			if !ok {
				return
			}

			eventData := clabutils.DockerMessageToEventData(msg)

			eventSink <- clabruntime.ContainerEvent{
				Timestamp:   eventData.Timestamp,
				Type:        eventData.Type,
				Action:      eventData.Action,
				ActorID:     eventData.ActorID,
				ActorName:   eventData.ActorName,
				ActorFullID: eventData.ActorFullID,
				Attributes:  eventData.Attributes,
			}
		}
	}
}
