package core

import (
	"os"
	"path/filepath"
	"testing"

	clabnodesiol "github.com/srl-labs/containerlab/nodes/iol"
	clabnodessros "github.com/srl-labs/containerlab/nodes/sros"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeKindConfigTopo(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "topo.clab.yml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))

	return path
}

func srosKindConfig(t *testing.T, c *CLab, node string) *clabnodessros.KindConfig {
	t.Helper()

	require.Contains(t, c.Nodes, node)
	kc, ok := c.Nodes[node].Config().KindConfig.(*clabnodessros.KindConfig)
	require.True(t, ok, "node %s kind config is %T", node, c.Nodes[node].Config().KindConfig)

	return kc
}

func TestKindConfigInheritance(t *testing.T) {
	path := writeKindConfigTopo(t, `
name: kc
topology:
  defaults:
    kind: nokia_srsim
    image: nokia_srsim:test
    config-mode: classic
  kinds:
    nokia_srsim:
      config-mode: mixed
      gen-component-config: false
      sfm: sfm-2s
  groups:
    chassis:
      type: sr-2s
      sfm: sfm-group
      components:
        - slot: A
          type: cpm-2s
        - slot: 1
          type: xcm-2s
  nodes:
    kind-only: {}
    grouped:
      group: chassis
    node-override:
      group: chassis
      config-mode: model-driven
      gen-component-config: true
      components:
        - slot: A
          type: cpm-2s
`)

	c, err := NewContainerLab(WithTopoPath(path, nil))
	require.NoError(t, err)

	t.Run("kind_overrides_defaults", func(t *testing.T) {
		kc := srosKindConfig(t, c, "kind-only")
		assert.Equal(t, clabnodessros.ConfigModeMixed, kc.ConfigMode)
		assert.False(t, kc.GenComponentConfig)
		assert.Equal(t, "sfm-2s", kc.SFM)
		assert.Empty(t, kc.Components)
	})

	t.Run("group_overrides_kind", func(t *testing.T) {
		kc := srosKindConfig(t, c, "grouped")
		assert.Equal(t, clabnodessros.ConfigModeMixed, kc.ConfigMode)
		assert.False(t, kc.GenComponentConfig)
		assert.Equal(t, "sfm-group", kc.SFM)
		require.Len(t, kc.Components, 2)
		assert.Equal(t, "xcm-2s", kc.Components[1].Type)
	})

	t.Run("node_overrides_group_with_whole_value", func(t *testing.T) {
		kc := srosKindConfig(t, c, "node-override")
		assert.Equal(t, clabnodessros.ConfigModeModelDriven, kc.ConfigMode)
		assert.True(t, kc.GenComponentConfig)
		assert.Equal(t, "sfm-group", kc.SFM)
		require.Len(t, kc.Components, 1, "node components replace group components")
		assert.Equal(t, "cpm-2s", kc.Components[0].Type)
	})

	t.Run("multi_container_follows_inherited_components", func(t *testing.T) {
		assert.False(t, c.Nodes["kind-only"].IsMultiContainer())
		assert.True(t, c.Nodes["grouped"].IsMultiContainer())
		assert.False(t, c.Nodes["node-override"].IsMultiContainer())
	})
}

func TestKindConfigDefaultsApplied(t *testing.T) {
	path := writeKindConfigTopo(t, `
name: kc
topology:
  defaults:
    config-mode: classic
  nodes:
    sim:
      kind: nokia_srsim
      image: nokia_srsim:test
`)

	c, err := NewContainerLab(WithTopoPath(path, nil))
	require.NoError(t, err)

	kc := srosKindConfig(t, c, "sim")
	assert.Equal(t, clabnodessros.ConfigModeClassic, kc.ConfigMode, "inherited from defaults")
	assert.True(t, kc.GenComponentConfig, "unset keys keep the kind's defaults")
}

func TestKindConfigPerKind(t *testing.T) {
	path := writeKindConfigTopo(t, `
name: kc
topology:
  kinds:
    nokia_srsim:
      image: nokia_srsim:test
      config-mode: classic
    cisco_iol:
      image: cisco_iol:test
      pid-offset: 64
  nodes:
    sim:
      kind: nokia_srsim
    iol1:
      kind: cisco_iol
      mgmt-intf: Ethernet1/0
`)

	c, err := NewContainerLab(WithTopoPath(path, nil))
	require.NoError(t, err)

	assert.Equal(t, clabnodessros.ConfigModeClassic, srosKindConfig(t, c, "sim").ConfigMode)

	iolCfg, ok := c.Nodes["iol1"].Config().KindConfig.(*clabnodesiol.KindConfig)
	require.True(t, ok)
	require.NotNil(t, iolCfg.PidOffset)
	assert.Equal(t, 64, *iolCfg.PidOffset)
	assert.Equal(t, "Ethernet1/0", iolCfg.MgmtIntf)
	assert.Empty(t, iolCfg.BootstrapConfig)
}

func TestKindConfigInheritanceErrors(t *testing.T) {
	tests := map[string]struct {
		topo    string
		wantErr string
	}{
		"defaults_key_unsupported_by_node_kind": {
			topo: `
name: kc
topology:
  defaults:
    config-mode: classic
  nodes:
    iol1:
      kind: cisco_iol
      image: cisco_iol:test
`,
			wantErr: `node "iol1": kind "cisco_iol" does not support key "config-mode" (set in defaults)`,
		},
		"group_key_unsupported_by_node_kind": {
			topo: `
name: kc
topology:
  groups:
    g:
      pid-offset: 10
  nodes:
    sim:
      kind: nokia_srsim
      image: nokia_srsim:test
      group: g
`,
			wantErr: `node "sim": kind "nokia_srsim" does not support key "pid-offset" (set in groups.g)`,
		},
		"kinds_block_key_for_other_kind": {
			topo: `
name: kc
topology:
  kinds:
    cisco_iol:
      config-mode: classic
  nodes:
    iol1:
      kind: cisco_iol
      image: cisco_iol:test
`,
			wantErr: `kind "cisco_iol" does not support key "config-mode" (set in kinds.cisco_iol)`,
		},
		"invalid_inherited_value_names_origin": {
			topo: `
name: kc
topology:
  kinds:
    cisco_iol:
      pid-offset: abc
  nodes:
    iol1:
      kind: cisco_iol
      image: cisco_iol:test
`,
			wantErr: `node "iol1": invalid value for key "pid-offset" (set in kinds.cisco_iol)`,
		},
		"unused_group_with_unknown_key": {
			topo: `
name: kc
topology:
  groups:
    unused:
      not-a-key: 1
  nodes:
    n1:
      kind: linux
      image: alpine:3
`,
			wantErr: `no kind supports key "not-a-key" (set in groups.unused)`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewContainerLab(WithTopoPath(writeKindConfigTopo(t, tc.topo), nil))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestKindConfigKindBlockCaseInsensitive(t *testing.T) {
	path := writeKindConfigTopo(t, `
name: kc
topology:
  kinds:
    Nokia_SRSIM:
      image: nokia_srsim:test
      config-mode: classic
  nodes:
    sim:
      kind: nokia_srsim
`)

	c, err := NewContainerLab(WithTopoPath(path, nil))
	require.NoError(t, err)

	assert.Equal(t, clabnodessros.ConfigModeClassic, srosKindConfig(t, c, "sim").ConfigMode)
	assert.Equal(t, "nokia_srsim:test", c.Nodes["sim"].Config().Image)
}
