package cmd

import "testing"

func TestRootRequirementHelpers(t *testing.T) {
	tests := []struct {
		name               string
		runtime            string
		wantGlobalRoot     bool
		wantCommandSkipped bool
	}{
		{
			name:               "default runtime",
			runtime:            "",
			wantGlobalRoot:     false,
			wantCommandSkipped: false,
		},
		{
			name:               "docker runtime",
			runtime:            "docker",
			wantGlobalRoot:     false,
			wantCommandSkipped: false,
		},
		{
			name:               "podman runtime",
			runtime:            "podman",
			wantGlobalRoot:     true,
			wantCommandSkipped: false,
		},
		{
			name:               "clabernetes runtime",
			runtime:            "clabernetes",
			wantGlobalRoot:     false,
			wantCommandSkipped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := globalRuntimeRequiresRoot(tt.runtime); got != tt.wantGlobalRoot {
				t.Fatalf("globalRuntimeRequiresRoot(%q) = %v, want %v",
					tt.runtime, got, tt.wantGlobalRoot)
			}

			if got := commandSkipsRoot(tt.runtime); got != tt.wantCommandSkipped {
				t.Fatalf("commandSkipsRoot(%q) = %v, want %v",
					tt.runtime, got, tt.wantCommandSkipped)
			}
		})
	}
}
