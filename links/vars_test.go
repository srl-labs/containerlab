package links

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_normalizeVars(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: map[string]any{},
		},
		{
			name: "simple string vars",
			input: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
			expected: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "nested map vars",
			input: map[string]any{
				"config": map[string]any{
					"nested": "value",
					"number": 42,
				},
				"simple": "string",
			},
			expected: map[string]any{
				"config": map[string]any{
					"nested": "value",
					"number": 42,
				},
				"simple": "string",
			},
		},
		{
			name: "array vars",
			input: map[string]any{
				"list": []any{"item1", "item2", 123},
				"nested_array": []any{
					map[string]any{"key": "value"},
					"string_item",
				},
			},
			expected: map[string]any{
				"list": []any{"item1", "item2", 123},
				"nested_array": []any{
					map[string]any{"key": "value"},
					"string_item",
				},
			},
		},
		{
			name: "mixed types",
			input: map[string]any{
				"string":  "text",
				"number":  123,
				"boolean": true,
				"null":    nil,
				"array":   []any{1, 2, 3},
				"object": map[string]any{
					"nested_string":  "nested",
					"nested_number":  456,
					"nested_boolean": false,
					"nested_array":   []any{"a", "b"},
					"nested_object": map[string]any{
						"deep": "value",
					},
				},
			},
			expected: map[string]any{
				"string":  "text",
				"number":  123,
				"boolean": true,
				"null":    nil,
				"array":   []any{1, 2, 3},
				"object": map[string]any{
					"nested_string":  "nested",
					"nested_number":  456,
					"nested_boolean": false,
					"nested_array":   []any{"a", "b"},
					"nested_object": map[string]any{
						"deep": "value",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeVars(tt.input)
			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("normalizeVars() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
