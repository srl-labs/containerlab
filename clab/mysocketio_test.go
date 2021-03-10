package clab

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCheckSockType(t *testing.T) {
	tests := map[string]struct {
		got  string
		want error
	}{
		"correct-type": {
			got:  "tcp",
			want: nil,
		},
		"incorrect-type": {
			got:  "dns",
			want: fmt.Errorf("mysocketio type dns is not supported. Supported types are tcp/tls/http/https"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := checkSockType(tc.got)

			if err != nil && tc.want != nil && (err.Error() != tc.want.Error()) {
				t.Fatalf("wanted '%v' got '%v'", tc.want, err)
			}
			if err != nil && tc.want == nil {
				t.Fatalf("wanted '%v' got '%v'", tc.want, err)
			}

		})
	}
}

func TestCheckSockPort(t *testing.T) {
	tests := map[string]struct {
		got  int
		want error
	}{
		"correct-port": {
			got:  22,
			want: nil,
		},
		"negative-port": {
			got:  -22,
			want: fmt.Errorf("incorrect port number -22"),
		},
		"large-port": {
			got:  123123123,
			want: fmt.Errorf("incorrect port number 123123123"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := checkSockPort(tc.got)

			if err != nil && tc.want != nil && (err.Error() != tc.want.Error()) {
				t.Fatalf("wanted '%v' got '%v'", tc.want, err)
			}
			if err != nil && tc.want == nil {
				t.Fatalf("wanted '%v' got '%v'", tc.want, err)
			}

		})
	}
}

func TestParseSocketCfg(t *testing.T) {
	tests := map[string]struct {
		got  string
		want mysocket
		err  error
	}{
		"simple-tcp": {
			got: "tcp/22",
			want: mysocket{
				Stype: "tcp",
				Port:  22,
			},
			err: nil,
		},
		"simple-http": {
			got: "http/8080",
			want: mysocket{
				Stype: "http",
				Port:  8080,
			},
			err: nil,
		},
		"wrong-type": {
			got:  "stcp/22",
			want: mysocket{},
			err:  fmt.Errorf(""),
		},
		"wrong-num-of-sections": {
			got:  "tcp/22/2",
			want: mysocket{},
			err:  fmt.Errorf(""),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseSocketCfg(tc.got)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("parseSocketCfg() mismatch (-want +got):\n%s", diff)
			}

			switch tc.err {
			case nil:
				if err != nil {
					t.Errorf("unexpected error %v", err)
				}
			case fmt.Errorf(""):
				if err != nil {
					t.Errorf("expected to have an errorm but got nil instead")
				}

			}

		})
	}
}
