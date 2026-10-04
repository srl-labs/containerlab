package vr_sros

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodessros "github.com/srl-labs/containerlab/nodes/sros"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gopkg.in/yaml.v2"
)

func Test_applyPartialConfig_HonorsContextCancellationWhileUnhealthy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	var deadline time.Time
	var hasDeadline bool

	mockRt.EXPECT().
		IsHealthy(gomock.Any(), "n1").
		DoAndReturn(func(ctx context.Context, _ string) (bool, error) {
			deadline, hasDeadline = ctx.Deadline()
			cancel()
			return false, nil
		}).
		AnyTimes()

	s := &vrSROS{}
	s.VRNode = *clabnodes.NewVRNode(s, defaultCredentials, scrapliPlatformName)
	s.Cfg = &clabtypes.NodeConfig{ShortName: "n1", LongName: "n1"}
	s.OverwriteNode = s
	s.WithRuntime(mockRt)

	done := make(chan error, 1)
	go func() {
		done <- s.applyPartialConfig(
			ctx,
			"192.0.2.1",
			scrapliPlatformName,
			"admin",
			"admin",
			strings.NewReader("configure system name n1"),
		)
	}()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
		require.True(t, hasDeadline)
		assert.WithinDuration(t, time.Now().Add(readyTimeout), deadline, time.Second)
	case <-time.After(time.Second):
		t.Fatal("applyPartialConfig did not return after context cancellation")
	}
}

func TestAosCXInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints []*clablinks.EndpointVeth
		node      *vrSROS
		resultEps []string
	}{
		"alias-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "1/1/1",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "1/1/3",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "1/1/5",
					},
				},
			},
			node: &vrSROS{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "sros",
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
			node: &vrSROS{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "sros",
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
					t.Errorf("got error for endpoint %+v", gotEndpointErr)
				}
			}

			if !foundError {
				gotCheckErr := tc.node.CheckInterfaceName()
				if gotCheckErr != nil {
					foundError = true
					t.Errorf("got error for check %+v", gotCheckErr)
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

func Test_vrSROS_Init_withComponents_buildsVariant(t *testing.T) {
	dir := t.TempDir()
	cfg := &clabtypes.NodeConfig{
		ShortName: "sros1",
		LabDir:    dir,
		NodeType:  "ixr-e",
		Env:       map[string]string{},
		KindConfig: &KindConfig{Components: []*Component{
			{Slot: "A", Type: "cpm-ixr-e"},
			{
				Slot: "1",
				Type: "imm24-sfp++8-sfp28+2-qsfp28",
				MDA:  clabnodessros.MDAS{{Slot: 1, Type: "m24-sfp++8-sfp28+2-qsfp28"}},
			},
		}},
	}
	mgmt := &clabtypes.MgmtNet{IPv4Subnet: "172.20.20.0/24", IPv6Subnet: "2001:db8::/64"}
	s := new(vrSROS)
	err := s.Init(cfg, clabnodes.WithMgmtNet(mgmt))
	require.NoError(t, err)
	assert.Contains(t, s.Cfg.Cmd, "cp: chassis=ixr-e slot=A card=cpm-ixr-e ___ "+
		"lc: max_nics=34 chassis=ixr-e slot=1 card=imm24-sfp++8-sfp28+2-qsfp28 mda/1=m24-sfp++8-sfp28+2-qsfp28")
}

func Test_vrSROS_Init_withComponents_appliesSFM(t *testing.T) {
	cfg := &clabtypes.NodeConfig{
		ShortName: "sros1",
		LabDir:    t.TempDir(),
		NodeType:  "sr-2s",
		Env:       map[string]string{},
		KindConfig: &KindConfig{
			SFM:        "sfm-2s",
			Components: []*Component{{Slot: "A", Type: "cpm-2s"}, {Slot: "1", Type: "xcm-2s"}},
		},
	}
	mgmt := &clabtypes.MgmtNet{IPv4Subnet: "172.20.20.0/24", IPv6Subnet: "2001:db8::/64"}
	s := new(vrSROS)
	require.NoError(t, s.Init(cfg, clabnodes.WithMgmtNet(mgmt)))
	assert.Contains(t, s.Cfg.Cmd, "cp: chassis=sr-2s slot=A sfm=sfm-2s card=cpm-2s ___ "+
		"lc: chassis=sr-2s slot=1 sfm=sfm-2s card=xcm-2s")
}

func Test_vrSROS_Init_withMultipleCPMs_errors(t *testing.T) {
	dir := t.TempDir()
	cfg := &clabtypes.NodeConfig{
		ShortName: "sros1",
		LabDir:    dir,
		NodeType:  "sr-7",
		Env:       map[string]string{},
		KindConfig: &KindConfig{
			Components: []*Component{{Slot: "A", Type: "cpm5"}, {Slot: "B", Type: "cpm5"}},
		},
	}
	mgmt := &clabtypes.MgmtNet{IPv4Subnet: "172.20.20.0/24", IPv6Subnet: "2001:db8::/64"}
	s := new(vrSROS)
	err := s.Init(cfg, clabnodes.WithMgmtNet(mgmt))
	require.Error(t, err)
}

func Test_vrSROS_verifyNokiaSrosImage(t *testing.T) {
	ctx := context.Background()

	t.Run("nil_runtime_returns_nil", func(t *testing.T) {
		s := &vrSROS{}
		s.VRNode = *clabnodes.NewVRNode(s, defaultCredentials, scrapliPlatformName)
		s.Cfg = &clabtypes.NodeConfig{ShortName: "n1", Image: "img"}
		s.OverwriteNode = s
		err := s.verifyNokiaSrosImage(ctx)
		assert.NoError(t, err)
	})

	t.Run("inspect_not_implemented_returns_nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
		mockRt.EXPECT().
			InspectImage(gomock.Any(), "img").
			Return(nil, fmt.Errorf("InspectImage not implemented for Podman runtime"))
		s := &vrSROS{}
		s.VRNode = *clabnodes.NewVRNode(s, defaultCredentials, scrapliPlatformName)
		s.Cfg = &clabtypes.NodeConfig{ShortName: "n1", Image: "img"}
		s.OverwriteNode = s
		s.WithRuntime(mockRt)
		err := s.verifyNokiaSrosImage(ctx)
		assert.NoError(t, err)
	})

	t.Run("image_with_srsim_label_returns_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
		mockRt.EXPECT().
			InspectImage(gomock.Any(), "img").
			Return(&clabruntime.ImageInspect{
				Config: clabruntime.ImageConfig{
					Labels: map[string]string{ociImageTitleLabel: srsimImageTitle},
				},
			}, nil)
		s := &vrSROS{}
		s.VRNode = *clabnodes.NewVRNode(s, defaultCredentials, scrapliPlatformName)
		s.Cfg = &clabtypes.NodeConfig{ShortName: "n1", Image: "img"}
		s.OverwriteNode = s
		s.WithRuntime(mockRt)
		err := s.verifyNokiaSrosImage(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nokia_srsim")
		assert.Contains(t, err.Error(), "n1")
	})

	t.Run("image_without_srsim_label_returns_nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
		mockRt.EXPECT().
			InspectImage(gomock.Any(), "img").
			Return(&clabruntime.ImageInspect{
				Config: clabruntime.ImageConfig{
					Labels: map[string]string{"other": "label"},
				},
			}, nil)
		s := &vrSROS{}
		s.VRNode = *clabnodes.NewVRNode(s, defaultCredentials, scrapliPlatformName)
		s.Cfg = &clabtypes.NodeConfig{ShortName: "n1", Image: "img"}
		s.OverwriteNode = s
		s.WithRuntime(mockRt)
		err := s.verifyNokiaSrosImage(ctx)
		assert.NoError(t, err)
	})
}

func TestKindConfigDecodesComponents(t *testing.T) {
	r := clabnodes.NewNodeRegistry()
	Register(r)
	e := r.Kind("nokia_sros")

	var components any
	require.NoError(t, yaml.Unmarshal([]byte(`
- slot: 1
  type: xcm-2s
  cpu: 4
  ram: 6
  max-nics: 5
`), &components))

	got, err := clabnodes.DecodeKindConfig(e, "n", "nokia_sros",
		[]clabtypes.KindConfigEntry{{Key: "components", Value: components, From: "nodes.n"}})
	require.NoError(t, err)
	assert.Equal(t, &KindConfig{Components: []*Component{
		{Slot: "1", Type: "xcm-2s", CPU: 4, RAM: 6, MaxNics: 5},
	}}, got)

	_, err = clabnodes.DecodeKindConfig(e, "n", "nokia_sros", []clabtypes.KindConfigEntry{{
		Key:   "components",
		Value: []any{map[any]any{"slot": 1, "env": map[any]any{"cpu": "4"}}},
		From:  "nodes.n",
	}})
	require.Error(t, err)
}

func Test_vrSROS_IsMultiContainer_withComponents(t *testing.T) {
	cfg := &clabtypes.NodeConfig{
		ShortName: "sros1",
		LabDir:    t.TempDir(),
		NodeType:  "sr-2s",
		Env:       map[string]string{},
		KindConfig: &KindConfig{
			Components: []*Component{{Slot: "A", Type: "cpm-2s"}, {Slot: "1", Type: "xcm-2s"}},
		},
	}
	mgmt := &clabtypes.MgmtNet{IPv4Subnet: "172.20.20.0/24", IPv6Subnet: "2001:db8::/64"}
	s := new(vrSROS)
	require.NoError(t, s.Init(cfg, clabnodes.WithMgmtNet(mgmt)))
	assert.False(t, s.IsMultiContainer(), "vSIM runs all cards in a single container")
}
