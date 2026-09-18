package fortinet_fortigate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestFortigateInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints []*clablinks.EndpointVeth
		node      *fortigate
		resultEps []string
	}{
		"alias-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "port2",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "port4",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "port6",
					},
				},
			},
			node: &fortigate{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "fortigate",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth1", "eth3", "eth5",
			},
		},
		"original-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth2",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth4",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth6",
					},
				},
			},
			node: &fortigate{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "fortigate",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth2", "eth4", "eth6",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			foundError := false
			tc.node.OverwriteNode = tc.node
			tc.node.InterfaceMappedPrefix = "eth"
			tc.node.FirstDataIfIndex = 1
			for _, ep := range tc.endpoints {
				gotEndpointErr := tc.node.AddEndpoint(ep)
				if gotEndpointErr != nil {
					foundError = true
					tt.Errorf("got error for endpoint %+v", gotEndpointErr)
				}
			}

			if !foundError {
				gotCheckErr := tc.node.CheckInterfaceName()
				if gotCheckErr != nil {
					foundError = true
					tt.Errorf("got error for check %+v", gotCheckErr)
				}

				if !foundError {
					for idx, ep := range tc.node.Endpoints {
						if ep.GetIfaceName() != tc.resultEps[idx] {
							tt.Errorf("got wrong mapped endpoint %q (%q), want %q",
								ep.GetIfaceName(), ep.GetIfaceAlias(), tc.resultEps[idx])
						}
					}
				}
			}
		})
	}
}

// newInitializedFortigate returns a fortigate node initialized the same way
// the node registry does, so PreDeploy tests run against realistic state.
func newInitializedFortigate(t *testing.T, cfg *clabtypes.NodeConfig) *fortigate {
	t.Helper()

	n := new(fortigate)
	err := n.Init(
		cfg,
		clabnodes.WithMgmtNet(&clabtypes.MgmtNet{
			IPv4Subnet: "172.20.20.0/24",
			IPv6Subnet: "2001:db8::/64",
		}),
	)
	if err != nil {
		t.Fatalf("failed to initialize fortigate node: %v", err)
	}
	return n
}

func TestFortigatePreDeployNoLicense(t *testing.T) {
	labDir := t.TempDir()
	n := newInitializedFortigate(t, &clabtypes.NodeConfig{
		ShortName: "fgt",
		LabDir:    labDir,
	})

	err := n.PreDeploy(context.Background(), &clabnodes.PreDeployParams{})
	if err != nil {
		t.Fatalf("PreDeploy failed: %v", err)
	}

	// the /config bind must be present to support startup-config functionality
	wantConfigBind := filepath.Join(labDir, "config") + ":/config"
	if !containsString(n.Cfg.Binds, wantConfigBind) {
		t.Errorf("Binds = %v, want it to contain %q", n.Cfg.Binds, wantConfigBind)
	}

	// no license configured: no tftpboot bind and no copied license file
	wantTftpBind := filepath.Join(labDir, tftpDirName) + ":/" + tftpDirName
	if containsString(n.Cfg.Binds, wantTftpBind) {
		t.Errorf("Binds = %v, did not expect tftpboot bind without a license", n.Cfg.Binds)
	}
	if _, err := os.Stat(filepath.Join(labDir, tftpDirName, licenseFileName)); !os.IsNotExist(err) {
		t.Errorf("license file unexpectedly exists at %s",
			filepath.Join(labDir, tftpDirName, licenseFileName))
	}
}

func TestFortigatePreDeployWithLicense(t *testing.T) {
	labDir := t.TempDir()

	// source license file in a separate directory to prove it gets copied
	licenseSrc := filepath.Join(t.TempDir(), "my-license.lic")
	licenseContent := "FGT40xxxxxxxxx license body"
	if err := os.WriteFile(licenseSrc, []byte(licenseContent), 0o600); err != nil {
		t.Fatalf("failed to write source license: %v", err)
	}

	n := newInitializedFortigate(t, &clabtypes.NodeConfig{
		ShortName: "fgt",
		LabDir:    labDir,
		License:   licenseSrc,
	})

	err := n.PreDeploy(context.Background(), &clabnodes.PreDeployParams{})
	if err != nil {
		t.Fatalf("PreDeploy failed: %v", err)
	}

	// license must be copied into the node lab dir under the tftpboot mount point
	copiedLicense := filepath.Join(labDir, tftpDirName, licenseFileName)
	got, err := os.ReadFile(copiedLicense)
	if err != nil {
		t.Fatalf("license was not copied to %s: %v", copiedLicense, err)
	}
	if string(got) != licenseContent {
		t.Errorf("copied license content = %q, want %q", string(got), licenseContent)
	}

	// both the /config and the tftpboot binds must be mounted
	wantBinds := []string{
		filepath.Join(labDir, "config") + ":/config",
		filepath.Join(labDir, tftpDirName) + ":/" + tftpDirName,
	}
	for _, want := range wantBinds {
		if !containsString(n.Cfg.Binds, want) {
			t.Errorf("Binds = %v, want it to contain %q", n.Cfg.Binds, want)
		}
	}
}

func TestFortigatePreDeployMissingLicenseFile(t *testing.T) {
	labDir := t.TempDir()
	n := newInitializedFortigate(t, &clabtypes.NodeConfig{
		ShortName: "fgt",
		LabDir:    labDir,
		License:   filepath.Join(t.TempDir(), "does-not-exist.lic"),
	})

	err := n.PreDeploy(context.Background(), &clabnodes.PreDeployParams{})
	if err == nil {
		t.Fatal("expected PreDeploy to fail for a missing license file, got nil")
	}
	if !strings.Contains(err.Error(), "file copy") {
		t.Errorf("error = %v, want it to mention the failed file copy", err)
	}
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
