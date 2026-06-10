package sros

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestComponentSorting(t *testing.T) {
	tests := []struct {
		name     string
		input    []*clabtypes.Component
		expected []string // expected slot order
	}{
		{
			name: "random mix",
			input: []*clabtypes.Component{
				{Slot: "3"},
				{Slot: "b"},
				{Slot: "1"},
				{Slot: "A"},
				{Slot: "2"},
			},
			expected: []string{"1", "2", "3", "b", "A"},
		},
		{
			name: "iom/xcm only",
			input: []*clabtypes.Component{
				{Slot: "3"},
				{Slot: "1"},
				{Slot: "2"},
				{Slot: "10"},
			},
			expected: []string{"1", "2", "3", "10"},
		},
		{
			name: "cpm only",
			input: []*clabtypes.Component{
				{Slot: "b"},
				{Slot: "A"},
			},
			expected: []string{"b", "A"},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			n := &sros{}
			n.Cfg = &clabtypes.NodeConfig{
				Components: c.input,
			}

			n.sortComponents()

			if len(n.Cfg.Components) != len(c.expected) {
				t.Fatalf("expected %d components, got %d", len(c.expected), len(n.Cfg.Components))
			}

			for i, expectedSlot := range c.expected {
				actualSlot := n.Cfg.Components[i].Slot
				if actualSlot != expectedSlot {
					t.Errorf("expected slot %q, got %q", expectedSlot, actualSlot)
				}
			}
		})
	}
}

func Test_sros_verifyNokiaSrsimImage(t *testing.T) {
	ctx := context.Background()

	t.Run("nil_runtime_returns_nil", func(t *testing.T) {
		n := &sros{}
		n.DefaultNode = *clabnodes.NewDefaultNode(n)
		n.Cfg = &clabtypes.NodeConfig{ShortName: "n1", Image: "img"}
		err := n.verifyNokiaSrsimImage(ctx)
		assert.NoError(t, err)
	})

	t.Run("inspect_not_implemented_returns_nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
		mockRt.EXPECT().
			InspectImage(gomock.Any(), "img").
			Return(nil, fmt.Errorf("InspectImage not implemented for Podman runtime"))
		n := &sros{}
		n.DefaultNode = *clabnodes.NewDefaultNode(n)
		n.Cfg = &clabtypes.NodeConfig{ShortName: "n1", Image: "img"}
		n.WithRuntime(mockRt)
		err := n.verifyNokiaSrsimImage(ctx)
		assert.NoError(t, err)
	})

	t.Run("image_with_vrnetlab_label_returns_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
		mockRt.EXPECT().
			InspectImage(gomock.Any(), "img").
			Return(&clabruntime.ImageInspect{
				Config: clabruntime.ImageConfig{
					Labels: map[string]string{vrnetlabVersionLabel: "abc"},
				},
			}, nil)
		n := &sros{}
		n.DefaultNode = *clabnodes.NewDefaultNode(n)
		n.Cfg = &clabtypes.NodeConfig{ShortName: "n1", Image: "img"}
		n.WithRuntime(mockRt)
		err := n.verifyNokiaSrsimImage(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vrnetlab")
		assert.Contains(t, err.Error(), "n1")
	})

	t.Run("image_with_srsim_title_returns_nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
		mockRt.EXPECT().
			InspectImage(gomock.Any(), "img").
			Return(&clabruntime.ImageInspect{
				Config: clabruntime.ImageConfig{
					Labels: map[string]string{srosImageTitleLabel: srosImageTitle},
				},
			}, nil)
		n := &sros{}
		n.DefaultNode = *clabnodes.NewDefaultNode(n)
		n.Cfg = &clabtypes.NodeConfig{ShortName: "n1", Image: "img"}
		n.WithRuntime(mockRt)
		err := n.verifyNokiaSrsimImage(ctx)
		assert.NoError(t, err)
	})

	t.Run("image_without_correct_labels_returns_nil_and_warns", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
		mockRt.EXPECT().
			InspectImage(gomock.Any(), "img").
			Return(&clabruntime.ImageInspect{
				Config: clabruntime.ImageConfig{
					Labels: map[string]string{"other": "label"},
				},
			}, nil)
		n := &sros{}
		n.DefaultNode = *clabnodes.NewDefaultNode(n)
		n.Cfg = &clabtypes.NodeConfig{ShortName: "n1", Image: "img"}
		n.WithRuntime(mockRt)
		err := n.verifyNokiaSrsimImage(ctx)
		assert.NoError(t, err)
	})
}

func Test_sros_buildStartupConfig(t *testing.T) {
	t.Run("full_config_from_file", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "startup.cfg")
		const content = "system name foo\n"
		require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0o600))

		n := &sros{}
		n.DefaultNode = *clabnodes.NewDefaultNode(n)
		n.Cfg = &clabtypes.NodeConfig{
			ShortName:     "n1",
			LongName:      "lab-n1",
			StartupConfig: cfgPath,
		}

		cfg, err := n.buildStartupConfig(false) // full config, not partial
		require.NoError(t, err)
		assert.Contains(t, cfg, "system name foo")
	})

	t.Run("default_and_partial_path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
		mockRt.EXPECT().Mgmt().Return(&clabtypes.MgmtNet{MTU: 1500}).AnyTimes()

		issueCert := false
		n := &sros{}
		n.DefaultNode = *clabnodes.NewDefaultNode(n)
		n.Cfg = &clabtypes.NodeConfig{
			ShortName:     "n1",
			LongName:      "lab-n1",
			NodeType:      "sr-1",
			StartupConfig: "",
			Env: map[string]string{
				envSrosConfigMode: "model-driven",
			},
			Certificate: &clabtypes.CertificateConfig{Issue: &issueCert},
			TLSKey:      "",
			TLSCert:     "",
			TLSAnchor:   "",
			LabDir:      t.TempDir(),
		}
		n.WithRuntime(mockRt)

		cfg, err := n.buildStartupConfig(true) // no full config; use default + partial
		require.NoError(t, err)
		assert.NotEmpty(t, cfg)
		// Default config should include typical SR OS snippets (banner, system, etc.)
		assert.Contains(t, cfg, "Welcome to Nokia SR OS")
	})
}

func Test_sros_generateComponentConfig(t *testing.T) {
	t.Run("integrated_sr1_default", func(t *testing.T) {
		n := newSrosComponentConfigTestNode("sr-1", nil, nil)

		cfg := n.generateComponentConfig()

		assert.Contains(t, cfg, "/configure card 1 card-type iom-1 admin-state enable")
		assert.Contains(t, cfg, "/configure card 1 mda 1 mda-type me6-100gb-qsfp28 admin-state enable")
		assert.Contains(t, cfg, "/configure card 1 mda 2 mda-type me12-100gb-qsfp28 admin-state enable")
		assert.NotContains(t, cfg, "power-shelf")
		assert.NotContains(t, cfg, "power-module")
	})

	t.Run("integrated_sr1s_default", func(t *testing.T) {
		n := newSrosComponentConfigTestNode("sr-1s", nil, nil)

		cfg := n.generateComponentConfig()

		assert.Contains(t, cfg, "/configure card 1 card-type xcm-1s admin-state enable")
		assert.Contains(t, cfg, "/configure card 1 mda 1 mda-type s36-100gb-qsfp28 admin-state enable")
		assert.Contains(t, cfg, "power-shelf 1 power-shelf-type ps-a4-shelf-dc")
		assert.Equal(t, 4, strings.Count(cfg, "power-module-type ps-a-dc-6000"))
	})

	t.Run("integrated_env_overrides_preserve_default_mda_slots", func(t *testing.T) {
		n := newSrosComponentConfigTestNode(
			"sr-1",
			map[string]string{
				envNokiaSrosCard:       "env-card",
				envNokiaSrosMDA + "_1": "env-mda",
			},
			nil,
		)

		cfg := n.generateComponentConfig()

		assert.Contains(t, cfg, "/configure card 1 card-type env-card admin-state enable")
		assert.Contains(t, cfg, "/configure card 1 mda 1 mda-type env-mda admin-state enable")
		assert.Contains(t, cfg, "/configure card 1 mda 2 mda-type me12-100gb-qsfp28 admin-state enable")
	})

	t.Run("disabled_component_config_returns_empty", func(t *testing.T) {
		n := newSrosComponentConfigTestNode(
			"sr-1",
			map[string]string{envDisableComponentConfigGen: "true"},
			nil,
		)

		assert.Empty(t, n.generateComponentConfig())
	})

	t.Run("classic_config_returns_empty", func(t *testing.T) {
		n := newSrosComponentConfigTestNode(
			"sr-1",
			map[string]string{envSrosConfigMode: string(ConfigModeClassic)},
			nil,
		)

		assert.Empty(t, n.generateComponentConfig())
	})

	t.Run("distributed_components_still_generate", func(t *testing.T) {
		n := newSrosComponentConfigTestNode("sr-2s", nil, nil)
		n.rootCtrName = "clab-test-sr2s-a"
		n.rootComponents = []*clabtypes.Component{
			{Slot: slotAName, Type: "cpm-2s", SFM: "sfm-2s"},
			{
				Slot: "1",
				Type: "xcm-2s",
				SFM:  "sfm-2s",
				XIOM: clabtypes.XIOMS{
					{
						Slot: 1,
						Type: "iom-s-3.0t",
						MDA:  clabtypes.MDAS{{Slot: 1, Type: "ms18-100gb-qsfp28"}},
					},
				},
			},
		}

		cfg := n.generateComponentConfig()

		assert.Contains(t, cfg, "/configure card 1 card-type xcm-2s admin-state enable")
		assert.Contains(t, cfg, "/configure sfm 1 sfm-type sfm-2s admin-state enable")
		assert.Contains(t, cfg, "/configure card 1 xiom x1 xiom-type iom-s-3.0t admin-state enable")
		assert.Contains(t, cfg, "/configure card 1 xiom x1 mda 1 mda-type ms18-100gb-qsfp28 admin-state enable")
		assert.Contains(t, cfg, "power-shelf 1 power-shelf-type ps-a4-shelf-dc")
	})
}

func newSrosComponentConfigTestNode(
	nodeType string,
	env map[string]string,
	components []*clabtypes.Component,
) *sros {
	if env == nil {
		env = map[string]string{}
	}
	if _, ok := env[envSrosConfigMode]; !ok {
		env[envSrosConfigMode] = string(ConfigModeModelDriven)
	}

	n := &sros{}
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = &clabtypes.NodeConfig{
		ShortName:  "n1",
		LongName:   "clab-test-n1",
		NodeType:   nodeType,
		Env:        env,
		Components: components,
	}
	return n
}
