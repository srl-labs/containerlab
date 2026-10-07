package core

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestCLab_exportTopologyDataWithMinimalTemplate(t *testing.T) {
	tests := []struct {
		name     string
		labName  string
		wantJSON string
	}{
		{
			name:    "basic_export",
			labName: "test-lab",
			wantJSON: `{
  "name": "test-lab",
  "type": "clab"
}`,
		},
		{
			name:    "empty_name",
			labName: "",
			wantJSON: `{
  "name": "",
  "type": "clab"
}`,
		},
		{
			name:    "special_characters",
			labName: "test-lab-123_special.chars",
			wantJSON: `{
  "name": "test-lab-123_special.chars",
  "type": "clab"
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal CLab instance
			c := &CLab{
				Config: &Config{
					Name: tt.labName,
				},
			}

			// Create a buffer to capture the output
			var buf bytes.Buffer

			// Call the function
			err := c.exportTopologyDataWithMinimalTemplate(&buf)
			if err != nil {
				t.Fatalf("exportTopologyDataWithMinimalTemplate() error = %v", err)
			}

			// Get the result and normalize whitespace
			got := strings.TrimSpace(buf.String())
			want := strings.TrimSpace(tt.wantJSON)

			// Compare the JSON output
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}

			// Verify it's valid JSON by checking structure
			if !strings.Contains(got, `"name"`) {
				t.Fatalf("output missing 'name' field: %s", got)
			}

			if !strings.Contains(got, `"type"`) {
				t.Fatalf("output missing 'type' field: %s", got)
			}

			if !strings.Contains(got, `"clab"`) {
				t.Fatalf("output missing 'clab' type value: %s", got)
			}
		})
	}
}

func TestCLab_exportTopologyDataWithMinimalTemplate_NilConfig(t *testing.T) {
	// Test behavior when Config is nil - should cause panic
	c := &CLab{
		Config: nil,
	}

	var buf bytes.Buffer

	// Expect this to panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when Config is nil, but didn't panic")
		}
	}()

	// This should panic
	_ = c.exportTopologyDataWithMinimalTemplate(&buf)
}

func TestExportTopologyDataMgmtNetworksList(t *testing.T) {
	main := &clabtypes.MgmtNet{Network: "main", IPv4Subnet: "192.0.2.0/24"}
	oob := &clabtypes.MgmtNet{Network: "oob", IPv4Subnet: "198.51.100.0/24"}
	c := &CLab{
		Config: &Config{
			Name:         "lab",
			Mgmt:         main,
			MgmtNetworks: clabtypes.MgmtNetworks{main, oob},
		},
		Nodes: map[string]clabnodes.Node{},
	}

	var buf bytes.Buffer
	if err := c.exportTopologyDataWithTemplate(context.Background(), &buf, ""); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Clab struct {
			Config struct {
				Mgmt []clabtypes.MgmtNet `json:"mgmt"`
			} `json:"config"`
		} `json:"clab"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid export JSON: %v\n%s", err, buf.String())
	}
	mgmt := got.Clab.Config.Mgmt
	if len(mgmt) != 2 || mgmt[0].Network != "main" ||
		mgmt[1].Network != "oob" || mgmt[1].IPv4Subnet != "198.51.100.0/24" {
		t.Fatalf("exported mgmt = %+v", mgmt)
	}
}
