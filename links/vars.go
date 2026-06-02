package links

import (
	clabutils "github.com/srl-labs/containerlab/utils"
)

// normalizeVars normalizes a vars map for JSON serialization compatibility.
// It converts map[any]any to map[string]any recursively.
// Returns nil if the input is nil.
func normalizeVars(vars map[string]any) map[string]any {
	if vars == nil {
		return nil
	}

	// NormalizeMapForJSON handles map[string]any recursively
	normalized := clabutils.NormalizeMapForJSON(vars)
	if m, ok := normalized.(map[string]any); ok {
		return m
	}

	// Should not happen with valid input, but return original as fallback
	return vars
}
