package events

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type formatter func(aggregatedEvent) error

func newFormatter(format string, w io.Writer) (formatter, error) {
	normalized := strings.TrimSpace(strings.ToLower(format))
	if normalized == "" {
		normalized = "plain"
	}

	switch normalized {
	case "plain":
		return plainFormatter(w), nil
	case "json":
		return jsonFormatter(w), nil
	default:
		return nil, fmt.Errorf("output format %q is not supported, use 'plain' or 'json'", format)
	}
}

func plainFormatter(w io.Writer) formatter {
	return func(ev aggregatedEvent) error {
		ts := ev.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		ts = ts.UTC()

		actor := ev.ActorID
		if actor == "" {
			actor = ev.ActorName
		}
		if actor == "" {
			actor = "-"
		}

		attrs := mergedEventAttributes(ev)
		keys := make([]string, 0, len(attrs))
		for k := range attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", k, attrs[k]))
		}

		suffix := ""
		if len(parts) > 0 {
			suffix = " (" + strings.Join(parts, ", ") + ")"
		}

		_, err := fmt.Fprintf(
			w,
			"%s %s %s %s%s\n",
			ts.Format(time.RFC3339Nano),
			ev.Type,
			ev.Action,
			actor,
			suffix,
		)

		return err
	}
}

func jsonFormatter(w io.Writer) formatter {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)

	return func(ev aggregatedEvent) error {
		copy := ev
		copy.Attributes = mergedEventAttributes(ev)

		return encoder.Encode(copy)
	}
}

func mergedEventAttributes(ev aggregatedEvent) map[string]string {
	if len(ev.Attributes) == 0 && ev.ActorName == "" && ev.ActorFullID == "" {
		return nil
	}

	attrs := make(map[string]string, len(ev.Attributes)+2)
	for k, v := range ev.Attributes {
		if v == "" {
			continue
		}

		attrs[k] = v
	}

	if ev.ActorName != "" {
		attrs["name"] = ev.ActorName
	}

	if ev.ActorFullID != "" {
		attrs["id"] = ev.ActorFullID
	}

	if len(attrs) == 0 {
		return nil
	}

	return attrs
}
