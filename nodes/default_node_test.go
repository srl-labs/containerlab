package nodes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/types"
)

func TestGenerateConfigs(t *testing.T) {
	defCfg := "default config"
	oldCfg := "old config"
	newCfg := "new config"

	tests := map[string]struct {
		cfg        *types.NodeConfig
		err        error
		preExists  bool
		postExists bool
		template   string
		out        string
	}{
		"suppress-true-first-start": {
			cfg: &types.NodeConfig{
				SuppressStartupConfig: true,
			},
			err:        nil,
			preExists:  false,
			postExists: false,
			template:   defCfg,
		},
		"suppress-true-existing-lab": {
			cfg: &types.NodeConfig{
				SuppressStartupConfig: true,
			},
			err:        nil,
			preExists:  true,
			postExists: true,
			out:        oldCfg,
			template:   defCfg,
		},
		"suppress-false-first-start": {
			cfg: &types.NodeConfig{
				SuppressStartupConfig: false,
			},
			err:        nil,
			preExists:  false,
			postExists: true,
			out:        defCfg,
			template:   defCfg,
		},
		"suppress-false-existing-lab": {
			cfg: &types.NodeConfig{
				SuppressStartupConfig: false,
			},
			err:        nil,
			preExists:  true,
			postExists: true,
			out:        oldCfg,
			template:   defCfg,
		},
		"startup-config-first-start": {
			cfg: &types.NodeConfig{
				StartupConfig: "other",
			},
			err:        nil,
			preExists:  false,
			postExists: true,
			out:        newCfg,
			template:   newCfg,
		},
		"startup-config-existing-lab": {
			cfg: &types.NodeConfig{
				StartupConfig: "other",
			},
			err:        nil,
			preExists:  true,
			postExists: true,
			out:        oldCfg,
			template:   newCfg,
		},
		"enforce-startup-config-first-start": {
			cfg: &types.NodeConfig{
				StartupConfig:        "other",
				EnforceStartupConfig: true,
			},
			err:        nil,
			preExists:  false,
			postExists: true,
			out:        newCfg,
			template:   newCfg,
		},
		"enforce-startup-config-existing-lab": {
			cfg: &types.NodeConfig{
				StartupConfig:        "other",
				EnforceStartupConfig: true,
			},
			err:        nil,
			preExists:  true,
			postExists: true,
			out:        newCfg,
			template:   newCfg,
		},
		"enforce-startup-config-no-startup": {
			cfg: &types.NodeConfig{
				EnforceStartupConfig: true,
			},
			err: ErrNoStartupConfig,
		},
		"enforce-and-suppress-startup-config": {
			cfg: &types.NodeConfig{
				EnforceStartupConfig:  true,
				SuppressStartupConfig: true,
			},
			err: ErrIncompatibleOptions,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			node := &DefaultNode{Cfg: tc.cfg}
			dstFolder := tt.TempDir()
			dstFile := filepath.Join(dstFolder, "config")

			if tc.preExists {
				err := os.WriteFile(dstFile, []byte(oldCfg), 0666)
				if err != nil {
					tt.Errorf("Could not write existing config: %v", err)
				}
			}

			err := node.GenerateConfig(dstFile, tc.template)
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					tt.Errorf("got: %v, wanted: %v", err, tc.err)
				}
			}
			if tc.postExists {
				cnt, err := os.ReadFile(dstFile)
				if err != nil {
					tt.Fatal(err)
				}
				if string(cnt) != tc.out {
					tt.Errorf("got %v, wanted %v", string(cnt), tc.out)
				}
			} else {
				if _, err := os.Stat(dstFile); err == nil {
					tt.Errorf("config file created")
				}
			}
		})
	}
}

func TestInterfacesAliases(t *testing.T) { // skipcq: GO-R1005
	tests := map[string]struct {
		endpoints           []*links.EndpointVeth
		node                *DefaultNode
		endpointErrContains string
		checkErrContains    string
		resultEps           []string
	}{
		"basic-parse": {
			endpoints: []*links.EndpointVeth{
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ge-0/0/0",
					},
				},
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ge-0/0/2",
					},
				},
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ge-0/0/4",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &types.NodeConfig{
					ShortName: "juniper",
				},
				InterfaceRegexp: regexp.MustCompile(`(?:et|xe|ge)-0/0/(?P<port>\d+)$`),
				InterfaceOffset: 0,
			},
			endpointErrContains: "",
			checkErrContains:    "",
			resultEps: []string{
				"eth0", "eth2", "eth4",
			},
		},
		"parse-offset": {
			endpoints: []*links.EndpointVeth{
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "GigabitEthernet2",
					},
				},
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "Gi3",
					},
				},
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "GigabitEthernet 5",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &types.NodeConfig{
					ShortName: "cisco",
				},
				InterfaceRegexp:  regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
				InterfaceOffset:  2,
				FirstDataIfIndex: 1,
			},
			endpointErrContains: "",
			checkErrContains:    "",
			resultEps: []string{
				"eth1", "eth2", "eth4",
			},
		},
		"skip-parse": {
			endpoints: []*links.EndpointVeth{
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "eth1",
					},
				},
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "eth2",
					},
				},
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "eth4",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &types.NodeConfig{
					ShortName: "cisco-original",
				},
				InterfaceRegexp: regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
				InterfaceOffset: 2,
			},
			endpointErrContains: "",
			checkErrContains:    "",
			resultEps: []string{
				"eth1", "eth2", "eth4",
			},
		},
		"overlap": {
			endpoints: []*links.EndpointVeth{
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "ge-0/0/1",
					},
				},
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "eth1",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &types.NodeConfig{
					ShortName: "juniper-overlap",
				},
				InterfaceRegexp: regexp.MustCompile(`(?:et|xe|ge)-0/0/(?P<port>\d+)$`),
				InterfaceOffset: 0,
			},
			endpointErrContains: "",
			checkErrContains:    "overlapping interface names",
			resultEps:           []string{},
		},
		"out-of-bounds-index": {
			endpoints: []*links.EndpointVeth{
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "Gi1",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &types.NodeConfig{
					ShortName: "cisco-oob",
				},
				InterfaceRegexp:  regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`),
				InterfaceOffset:  2,
				FirstDataIfIndex: 1,
			},
			endpointErrContains: "0 ! >= 1",
			checkErrContains:    "",
			resultEps:           []string{},
		},
		"regexp-no-group": {
			endpoints: []*links.EndpointVeth{
				{
					EndpointGeneric: links.EndpointGeneric{
						IfaceName: "Gi2",
					},
				},
			},
			node: &DefaultNode{
				Cfg: &types.NodeConfig{
					ShortName: "cisco-noregexpgroup",
				},
				InterfaceRegexp: regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(\d+)$`),
				InterfaceOffset: 2,
			},
			endpointErrContains: "'port' capture group missing",
			checkErrContains:    "",
			resultEps:           []string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(*testing.T) {
			foundError := false
			tc.node.OverwriteNode = tc.node
			tc.node.InterfaceMappedPrefix = "eth"
			for _, ep := range tc.endpoints {
				gotEndpointErr := tc.node.AddEndpoint(ep)
				if gotEndpointErr != nil {
					foundError = true
					if tc.endpointErrContains != "" && !(strings.Contains(
						fmt.Sprint(gotEndpointErr), tc.endpointErrContains)) {
						t.Errorf("got error for endpoint %+v, want %s", gotEndpointErr, tc.endpointErrContains)
					}
				}
			}

			if tc.endpointErrContains != "" && !foundError {
				t.Errorf("got no error for endpoints, want %s", tc.endpointErrContains)
			}

			if !foundError {
				gotCheckErr := tc.node.CheckInterfaceName()
				if gotCheckErr != nil {
					foundError = true
					if tc.checkErrContains != "" && !(strings.Contains(
						fmt.Sprint(gotCheckErr), tc.checkErrContains)) {
						t.Errorf("got error for check %+v, want %s", gotCheckErr, tc.checkErrContains)
					}
				}

				if tc.checkErrContains != "" && !foundError {
					t.Errorf("got no error for check interface, want %s", tc.checkErrContains)
				}

				if !foundError {
					for idx, ep := range tc.node.Endpoints {
						if ep.GetIfaceName() != tc.resultEps[idx] {
							t.Errorf("got wrong mapped endpoint %q (%q), want %q",
								ep.GetIfaceName(), ep.GetIfaceAlias(), tc.resultEps[idx])
						}
					}
				}
			}
		})
	}
}
