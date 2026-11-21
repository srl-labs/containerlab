package schemas

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestPortMappingPattern(t *testing.T) {
	b, err := os.ReadFile("clab.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Definitions map[string]struct {
			Properties map[string]struct {
				Items struct {
					Pattern string
				}
			}
		}
	}
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(
		schema.Definitions["node-config"].Properties["ports"].Items.Pattern,
	)
	for _, tc := range []struct {
		spec  string
		valid bool
	}{
		{spec: "80:8080", valid: true},
		{spec: "127.0.0.1:8022:22/tcp", valid: true},
		{spec: "55555:43555/udp", valid: true},
		{spec: "55555:43555/sctp", valid: true},
		{spec: "8022:22/ts", valid: true},
		{spec: " 8574:57400/TS ", valid: true},
		{spec: "9000-9001:80-81/ts", valid: true},
		{spec: "1:65535/ts", valid: true},
		{spec: "65535:1/ts", valid: true},
		{spec: "22/ts"},
		{spec: "0:22/ts"},
		{spec: "8022:0/ts"},
		{spec: "65536:22/ts"},
		{spec: "8022:65536/ts"},
		{spec: "127.0.0.1:8022:22/ts"},
	} {
		t.Run(tc.spec, func(t *testing.T) {
			if got := pattern.MatchString(tc.spec); got != tc.valid {
				t.Errorf("schema accepts = %v, want %v", got, tc.valid)
			}
			if strings.HasSuffix(strings.ToLower(strings.TrimSpace(tc.spec)), "/ts") {
				_, _, err := clabtypes.SplitDockerAndTailscalePorts([]string{tc.spec})
				if (err == nil) != tc.valid {
					t.Errorf("runtime error = %v, schema expects valid = %v", err, tc.valid)
				}
			}
		})
	}
}

func TestNetemDurationPattern(t *testing.T) {
	b, err := os.ReadFile("clab.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Definitions map[string]struct {
			Properties map[string]struct {
				Pattern string
			}
		}
	}
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(schema.Definitions["netem"].Properties["delay"].Pattern)
	for _, tc := range []struct {
		duration string
		valid    bool
	}{
		{duration: "100ms", valid: true},
		{duration: "1.5s", valid: true},
		{duration: "1m30s", valid: true},
		{duration: "250µs", valid: true},
		{duration: "250us", valid: true},
		{duration: "100"},
		{duration: "-5ms"},
		{duration: "5 ms"},
	} {
		t.Run(tc.duration, func(t *testing.T) {
			if got := pattern.MatchString(tc.duration); got != tc.valid {
				t.Errorf("schema accepts = %v, want %v", got, tc.valid)
			}
			d, err := time.ParseDuration(tc.duration)
			if tc.valid && (err != nil || d < 0) {
				t.Errorf("the schema accepts %q, but the topology doesn't: %v", tc.duration, err)
			}
		})
	}
}
