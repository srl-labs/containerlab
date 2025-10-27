package events

import "time"

type aggregatedEvent struct {
	Timestamp   time.Time         `json:"timestamp"`
	Type        string            `json:"type"`
	Action      string            `json:"action"`
	ActorID     string            `json:"actor_id"`
	ActorName   string            `json:"actor_name"`
	ActorFullID string            `json:"actor_full_id"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}
