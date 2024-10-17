package srl

import (
	"testing"
)

func Test_getPrompt(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    string
		wantErr bool
	}{
		{
			name:    "Test with valid input",
			s:       `value = "test-prompt"`,
			want:    "test-prompt",
			wantErr: false,
		},
		{
			name:    "Test with invalid input",
			s:       `invalid input`,
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPrompt(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPrompt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getPrompt() = %v, want %v", got, tt.want)
			}
		})
	}
}
