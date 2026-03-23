package vr_sros

import (
	"context"
	"fmt"
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

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

func Test_vrSROS_Init_withComponents_warns(t *testing.T) {
	dir := t.TempDir()
	cfg := &clabtypes.NodeConfig{
		ShortName:  "sros1",
		LabDir:     dir,
		Env:        map[string]string{},
		Components: []*clabtypes.Component{{Slot: "A"}},
	}
	mgmt := &clabtypes.MgmtNet{IPv4Subnet: "172.20.20.0/24", IPv6Subnet: "2001:db8::/64"}
	s := new(vrSROS)
	err := s.Init(cfg, clabnodes.WithMgmtNet(mgmt))
	require.NoError(t, err)
	assert.NotEmpty(t, s.Cfg.Components)
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
