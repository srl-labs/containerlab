package cmd

import (
	"testing"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

func TestCancelledDestroyTimeout(t *testing.T) {
	tests := []struct {
		name    string
		runtime string
		timeout time.Duration
		want    time.Duration
	}{
		{
			name:    "default runtime uses the local bound",
			runtime: "",
			timeout: defaultTimeout,
			want:    maxCancelledDestroyTimeout,
		},
		{
			name:    "docker runtime uses the local bound",
			runtime: "docker",
			timeout: defaultTimeout,
			want:    maxCancelledDestroyTimeout,
		},
		{
			name:    "lab runtime gets the full command timeout",
			runtime: clablabruntime.ClabernetesRuntimeName,
			timeout: defaultLabRuntimeTimeout,
			want:    defaultLabRuntimeTimeout,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := &Options{Global: &GlobalOptions{Runtime: test.runtime, Timeout: test.timeout}}

			if got := cancelledDestroyTimeout(o); got != test.want {
				t.Fatalf("cancelledDestroyTimeout() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestCancellationRequestedDefaultsToFalse(t *testing.T) {
	// the signal handler is never started in tests, so no cancellation can have been
	// requested; the check must not block on the unclosed channel
	if cancellationRequested() {
		t.Fatal("cancellationRequested() = true without a signal handler")
	}
}
