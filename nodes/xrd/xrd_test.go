package xrd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func newTestNode(binds []string, endpoints []clablinks.Endpoint, env map[string]string) *xrd {
	return &xrd{
		DefaultNode: clabnodes.DefaultNode{
			Cfg: &clabtypes.NodeConfig{
				ShortName: "xrd-test",
				Binds:     binds,
				Env:       env,
			},
			Endpoints: endpoints,
		},
	}
}

func makeEndpoints(names ...string) []clablinks.Endpoint {
	eps := make([]clablinks.Endpoint, len(names))
	for i, name := range names {
		eps[i] = &clablinks.EndpointVeth{
			EndpointGeneric: clablinks.EndpointGeneric{
				IfaceName: name,
			},
		}
	}
	return eps
}

func writeTempMapping(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, xrdIntfMappingFile)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenInterfacesEnv_NoMapping(t *testing.T) {
	n := newTestNode(nil, makeEndpoints("Gi0-0-0-0", "Gi0-0-0-1"), nil)

	if err := n.genInterfacesEnv(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	xrIntf := n.Cfg.Env["XR_INTERFACES"]
	if !strings.Contains(xrIntf, "xr_name=Gi0/0/0/0") {
		t.Errorf("expected default dash-to-slash mapping for Gi0-0-0-0, got: %s", xrIntf)
	}
	if !strings.Contains(xrIntf, "xr_name=Gi0/0/0/1") {
		t.Errorf("expected default dash-to-slash mapping for Gi0-0-0-1, got: %s", xrIntf)
	}

	// Management interface should use the default from xrdEnv
	mgmt := n.Cfg.Env["XR_MGMT_INTERFACES"]
	if !strings.Contains(mgmt, "Mg0/RP0/CPU0/0") {
		t.Errorf("expected default mgmt interface, got: %s", mgmt)
	}
}

func TestGenInterfacesEnv_FullMapping(t *testing.T) {
	mappingJSON := `{
		"ManagementIntf": { "eth0": "MgmtEth0/RP0/CPU0/0" },
		"DataIntf": {
			"Gi0-0-0-0": "HundredGigE0/0/0/0",
			"Gi0-0-0-1": "HundredGigE0/0/0/1"
		}
	}`
	path := writeTempMapping(t, mappingJSON)
	bind := path + ":/etc/xrd/" + xrdIntfMappingFile + ":ro"

	n := newTestNode(
		[]string{bind},
		makeEndpoints("Gi0-0-0-0", "Gi0-0-0-1"),
		nil,
	)

	if err := n.genInterfacesEnv(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	xrIntf := n.Cfg.Env["XR_INTERFACES"]
	if !strings.Contains(xrIntf, "xr_name=HundredGigE0/0/0/0") {
		t.Errorf("expected mapped HundredGigE0/0/0/0, got: %s", xrIntf)
	}
	if !strings.Contains(xrIntf, "xr_name=HundredGigE0/0/0/1") {
		t.Errorf("expected mapped HundredGigE0/0/0/1, got: %s", xrIntf)
	}

	mgmt := n.Cfg.Env["XR_MGMT_INTERFACES"]
	if !strings.Contains(mgmt, "MgmtEth0/RP0/CPU0/0") {
		t.Errorf("expected custom mgmt interface MgmtEth0/RP0/CPU0/0, got: %s", mgmt)
	}
}

func TestGenInterfacesEnv_PartialMapping(t *testing.T) {
	mappingJSON := `{
		"DataIntf": {
			"Gi0-0-0-0": "TenGigE0/0/0/0"
		}
	}`
	path := writeTempMapping(t, mappingJSON)
	bind := path + ":/etc/xrd/" + xrdIntfMappingFile + ":ro"

	n := newTestNode(
		[]string{bind},
		makeEndpoints("Gi0-0-0-0", "Gi0-0-0-1"),
		nil,
	)

	if err := n.genInterfacesEnv(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	xrIntf := n.Cfg.Env["XR_INTERFACES"]
	// Gi0-0-0-0 should use the mapped name
	if !strings.Contains(xrIntf, "xr_name=TenGigE0/0/0/0") {
		t.Errorf("expected mapped TenGigE0/0/0/0, got: %s", xrIntf)
	}
	// Gi0-0-0-1 should fall back to default dash→slash
	if !strings.Contains(xrIntf, "xr_name=Gi0/0/0/1") {
		t.Errorf("expected default Gi0/0/0/1 for unmapped interface, got: %s", xrIntf)
	}

	// No ManagementIntf in mapping, so default should be used
	mgmt := n.Cfg.Env["XR_MGMT_INTERFACES"]
	if !strings.Contains(mgmt, "Mg0/RP0/CPU0/0") {
		t.Errorf("expected default mgmt interface, got: %s", mgmt)
	}
}

func TestGenInterfacesEnv_UserEnvOverride(t *testing.T) {
	mappingJSON := `{
		"DataIntf": {
			"Gi0-0-0-0": "HundredGigE0/0/0/0"
		}
	}`
	path := writeTempMapping(t, mappingJSON)
	bind := path + ":/etc/xrd/" + xrdIntfMappingFile + ":ro"

	userEnv := map[string]string{
		"XR_INTERFACES": "linux:Gi0-0-0-0,xr_name=CustomIntf0/0/0/0;",
	}
	n := newTestNode(
		[]string{bind},
		makeEndpoints("Gi0-0-0-0"),
		userEnv,
	)

	if err := n.genInterfacesEnv(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// User-defined env should take precedence over both mapping and default
	xrIntf := n.Cfg.Env["XR_INTERFACES"]
	if !strings.Contains(xrIntf, "CustomIntf0/0/0/0") {
		t.Errorf("expected user env override to win, got: %s", xrIntf)
	}
}

func TestGenInterfacesEnv_MalformedJSON(t *testing.T) {
	path := writeTempMapping(t, `{not valid json}`)
	bind := path + ":/etc/xrd/" + xrdIntfMappingFile + ":ro"

	n := newTestNode(
		[]string{bind},
		makeEndpoints("Gi0-0-0-0"),
		nil,
	)

	err := n.genInterfacesEnv()
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestGenInterfacesEnv_MissingFile(t *testing.T) {
	bind := "/nonexistent/path/XrdIntfMapping.json:/etc/xrd/" + xrdIntfMappingFile + ":ro"

	n := newTestNode(
		[]string{bind},
		makeEndpoints("Gi0-0-0-0"),
		nil,
	)

	err := n.genInterfacesEnv()
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
