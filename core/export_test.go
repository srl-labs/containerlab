package core

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmocklinks "github.com/srl-labs/containerlab/mocks/mocklinks"
	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
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

// TestCLab_exportTopologyDataWithLinkIPs tests that link IPv4/IPv6 addresses
// are correctly exported in the topology data.
func TestCLab_exportTopologyDataWithLinkIPs(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// Create mock nodes
	mockNode1 := clabmocksmocknodes.NewMockNode(mockCtrl)
	mockNode1.EXPECT().Config().Return(
		&clabtypes.NodeConfig{
			ShortName: "node1",
			LongName:  "clab-test-node1",
			Kind:      "linux",
			Image:     "alpine:latest",
		},
	).AnyTimes()
	mockNode1.EXPECT().GetShortName().Return("node1").AnyTimes()

	mockNode2 := clabmocksmocknodes.NewMockNode(mockCtrl)
	mockNode2.EXPECT().Config().Return(
		&clabtypes.NodeConfig{
			ShortName: "node2",
			LongName:  "clab-test-node2",
			Kind:      "linux",
			Image:     "alpine:latest",
		},
	).AnyTimes()
	mockNode2.EXPECT().GetShortName().Return("node2").AnyTimes()

	// Create mock endpoints with IP addresses
	mockEndpoint1 := clabmocksmocklinks.NewMockEndpoint(mockCtrl)
	mockEndpoint1.EXPECT().GetNode().Return(mockNode1).AnyTimes()
	mockEndpoint1.EXPECT().GetIfaceName().Return("eth1").AnyTimes()
	mockEndpoint1.EXPECT().GetMac().Return(nil).AnyTimes()
	mockEndpoint1.EXPECT().GetIPv4Addr().Return("192.168.1.1/24").AnyTimes()
	mockEndpoint1.EXPECT().GetIPv6Addr().Return("2001:db8::1/64").AnyTimes()

	mockEndpoint2 := clabmocksmocklinks.NewMockEndpoint(mockCtrl)
	mockEndpoint2.EXPECT().GetNode().Return(mockNode2).AnyTimes()
	mockEndpoint2.EXPECT().GetIfaceName().Return("eth1").AnyTimes()
	mockEndpoint2.EXPECT().GetMac().Return(nil).AnyTimes()
	mockEndpoint2.EXPECT().GetIPv4Addr().Return("192.168.1.2/24").AnyTimes()
	mockEndpoint2.EXPECT().GetIPv6Addr().Return("2001:db8::2/64").AnyTimes()

	// Create mock link
	mockLink := clabmocksmocklinks.NewMockLink(mockCtrl)
	mockLink.EXPECT().GetEndpoints().Return([]clablinks.Endpoint{mockEndpoint1, mockEndpoint2}).AnyTimes()

	// Create CLab instance with the mock link
	prefix := "clab"
	c := &CLab{
		Config: &Config{
			Name:   "test-lab",
			Prefix: &prefix,
			Mgmt:   new(clabtypes.MgmtNet),
		},
		Nodes: map[string]clabnodes.Node{
			"node1": mockNode1,
			"node2": mockNode2,
		},
		Links: map[int]clablinks.Link{
			0: mockLink,
		},
	}

	// Export the topology
	var buf bytes.Buffer
	err := c.exportTopologyDataWithTemplate(context.Background(), &buf, "")
	if err != nil {
		t.Fatalf("exportTopologyDataWithTemplate() error = %v", err)
	}

	// Parse the JSON output
	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Verify the links section exists
	links, ok := result["links"].([]interface{})
	if !ok {
		t.Fatalf("links field is missing or not an array")
	}

	if len(links) == 0 {
		t.Fatalf("expected at least one link, got 0")
	}

	// Check the first link
	link := links[0].(map[string]interface{})

	// Verify endpoint "a" has IP addresses
	endpointA, ok := link["a"].(map[string]interface{})
	if !ok {
		t.Fatalf("endpoint 'a' is missing or not an object")
	}

	if ipv4, ok := endpointA["ipv4"].(string); !ok || ipv4 != "192.168.1.1/24" {
		t.Errorf("endpoint 'a' ipv4 = %v, want '192.168.1.1/24'", endpointA["ipv4"])
	}

	if ipv6, ok := endpointA["ipv6"].(string); !ok || ipv6 != "2001:db8::1/64" {
		t.Errorf("endpoint 'a' ipv6 = %v, want '2001:db8::1/64'", endpointA["ipv6"])
	}

	// Verify endpoint "z" has IP addresses
	endpointZ, ok := link["z"].(map[string]interface{})
	if !ok {
		t.Fatalf("endpoint 'z' is missing or not an object")
	}

	if ipv4, ok := endpointZ["ipv4"].(string); !ok || ipv4 != "192.168.1.2/24" {
		t.Errorf("endpoint 'z' ipv4 = %v, want '192.168.1.2/24'", endpointZ["ipv4"])
	}

	if ipv6, ok := endpointZ["ipv6"].(string); !ok || ipv6 != "2001:db8::2/64" {
		t.Errorf("endpoint 'z' ipv6 = %v, want '2001:db8::2/64'", endpointZ["ipv6"])
	}
}
