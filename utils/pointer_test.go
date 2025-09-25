package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPointer(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "string_value",
			test: func(t *testing.T) {
				input := "test"
				got := Pointer(input)
				
				if got == nil {
					t.Fatalf("Pointer() returned nil")
				}
				
				if diff := cmp.Diff(*got, input); diff != "" {
					t.Fatalf("mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "int_value",
			test: func(t *testing.T) {
				input := 42
				got := Pointer(input)
				
				if got == nil {
					t.Fatalf("Pointer() returned nil")
				}
				
				if diff := cmp.Diff(*got, input); diff != "" {
					t.Fatalf("mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "bool_value",
			test: func(t *testing.T) {
				input := true
				got := Pointer(input)
				
				if got == nil {
					t.Fatalf("Pointer() returned nil")
				}
				
				if diff := cmp.Diff(*got, input); diff != "" {
					t.Fatalf("mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "empty_string",
			test: func(t *testing.T) {
				input := ""
				got := Pointer(input)
				
				if got == nil {
					t.Fatalf("Pointer() returned nil")
				}
				
				if diff := cmp.Diff(*got, input); diff != "" {
					t.Fatalf("mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "zero_int",
			test: func(t *testing.T) {
				input := 0
				got := Pointer(input)
				
				if got == nil {
					t.Fatalf("Pointer() returned nil")
				}
				
				if diff := cmp.Diff(*got, input); diff != "" {
					t.Fatalf("mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "false_bool",
			test: func(t *testing.T) {
				input := false
				got := Pointer(input)
				
				if got == nil {
					t.Fatalf("Pointer() returned nil")
				}
				
				if diff := cmp.Diff(*got, input); diff != "" {
					t.Fatalf("mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "struct_value",
			test: func(t *testing.T) {
				type testStruct struct {
					Name string
					ID   int
				}
				input := testStruct{Name: "test", ID: 1}
				got := Pointer(input)
				
				if got == nil {
					t.Fatalf("Pointer() returned nil")
				}
				
				if diff := cmp.Diff(*got, input); diff != "" {
					t.Fatalf("mismatch (-want +got):\n%s", diff)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}