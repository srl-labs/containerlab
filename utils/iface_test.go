package utils

import (
	"testing"
)

func TestSanitiseInterfaceName(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"sanitise-test-original": {
			input: "eth0",
			want:  "eth0",
		},
		"sanitise-test-xrd": {
			input: "Gi0-0-0-0",
			want:  "Gi0-0-0-0",
		},
		"sanitise-test-c8000": {
			input: "Hu0_0_0_1",
			want:  "Hu0_0_0_1",
		},
		"sanitise-test-asa": {
			input: "GigabitEthernet 0/0",
			want:  "GigabitEthernet-0-0",
		},
		"sanitise-test-junos": {
			input: "ge-0/0/0",
			want:  "ge-0-0-0",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := SanitiseInterfaceName(tt.input)
			if got != tt.want {
				t.Errorf("got wrong sanitised interface name %q, want %q", got, tt.want)
			}
		})
	}
}
