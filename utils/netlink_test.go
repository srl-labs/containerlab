package utils

import "testing"

func TestSanitizeInterfaceName(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"sanitize-test-original": {
			input: "eth0",
			want:  "eth0",
		},
		"sanitize-test-xrd": {
			input: "Gi0-0-0-0",
			want:  "Gi0-0-0-0",
		},
		"sanitize-test-c8000": {
			input: "Hu0_0_0_1",
			want:  "Hu0_0_0_1",
		},
		"sanitize-test-asa": {
			input: "GigabitEthernet 0/0",
			want:  "GigabitEthernet-0-0",
		},
		"sanitize-test-junos": {
			input: "ge-0/0/0",
			want:  "ge-0-0-0",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := SanitizeInterfaceName(tt.input)
			if got != tt.want {
				t.Errorf("got wrong sanitized interface name %q, want %q", got, tt.want)
			}
		})
	}
}
