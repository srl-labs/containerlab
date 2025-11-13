package utils

import (
	"maps"
	"time"

	dockerEvents "github.com/docker/docker/api/types/events"
)

// DockerEventData captures container-related information from a Docker event message.
type DockerEventData struct {
	Timestamp   time.Time
	Type        string
	Action      string
	ActorID     string
	ActorName   string
	ActorFullID string
	Attributes  map[string]string
}

// DockerMessageToEventData normalizes a Docker event message into DockerEventData.
func DockerMessageToEventData(msg dockerEvents.Message) DockerEventData {
	ts := time.Unix(0, msg.TimeNano)
	if ts.IsZero() {
		ts = time.Unix(msg.Time, 0)
	}
	if ts.IsZero() {
		ts = time.Now()
	}

	attributes := make(map[string]string, len(msg.Actor.Attributes)+1)
	maps.Copy(attributes, msg.Actor.Attributes)

	if msg.Scope != "" {
		attributes["scope"] = msg.Scope
	}

	return DockerEventData{
		Timestamp:   ts,
		Type:        string(msg.Type),
		Action:      string(msg.Action),
		ActorID:     msg.Actor.ID,
		ActorName:   attributes["name"],
		ActorFullID: msg.Actor.ID,
		Attributes:  attributes,
	}
}
