package vr_sros

import (
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSrosVariant(t *testing.T) {
	tests := map[string]struct {
		chassis    string
		components []*clabtypes.Component
		env        map[string]string
		want       string
		wantErr    bool
	}{
		"node-level-sfm-applied-to-every-segment": {
			chassis: "sr-2s",
			env:     map[string]string{"NOKIA_SROS_SFM": "sfm-2s"},
			components: []*clabtypes.Component{
				{Slot: "A", Type: "cpm-2s"},
				{Slot: "1", Type: "xcm-2s", SFM: "sfm-override"},
			},
			want: "cp: chassis=sr-2s slot=A sfm=sfm-2s card=cpm-2s ___ " +
				"lc: chassis=sr-2s slot=1 sfm=sfm-override card=xcm-2s",
		},
		"two-cpms-rejected": {
			chassis: "sr-7",
			components: []*clabtypes.Component{
				{Slot: "A", Type: "cpm5"},
				{Slot: "B", Type: "cpm5"},
			},
			wantErr: true,
		},
		"integrated-single-component": {
			chassis: "ixr-r6",
			components: []*clabtypes.Component{
				{
					Slot: "A",
					Type: "cpiom-ixr-r6",
					Env:  map[string]string{"cpu": "2", "ram": "4"},
					MDA:  clabtypes.MDAS{{Slot: 1, Type: "m6-10g-sfp++4-25g-sfp28"}},
				},
			},
			want: "cpu=2 ram=4 chassis=ixr-r6 slot=A card=cpiom-ixr-r6 mda/1=m6-10g-sfp++4-25g-sfp28",
		},
		"integrated-empty-slot-defaults-to-A": {
			chassis: "sr-1",
			components: []*clabtypes.Component{
				{Type: "iom-1", MDA: clabtypes.MDAS{{Slot: 1, Type: "me6-100gb-qsfp28"}}},
			},
			want: "chassis=sr-1 slot=A card=iom-1 mda/1=me6-100gb-qsfp28",
		},
		"distributed-cpm-and-lc": {
			chassis: "ixr-e",
			components: []*clabtypes.Component{
				{Slot: "A", Type: "cpm-ixr-e", Env: map[string]string{"cpu": "2", "ram": "4"}},
				{
					Slot: "1",
					Type: "imm24-sfp++8-sfp28+2-qsfp28",
					Env:  map[string]string{"cpu": "2", "ram": "4", "max_nics": "34"},
					MDA:  clabtypes.MDAS{{Slot: 1, Type: "m24-sfp++8-sfp28+2-qsfp28"}},
				},
			},
			want: "cp: cpu=2 ram=4 chassis=ixr-e slot=A card=cpm-ixr-e ___ " +
				"lc: cpu=2 ram=4 max_nics=34 chassis=ixr-e slot=1 card=imm24-sfp++8-sfp28+2-qsfp28 mda/1=m24-sfp++8-sfp28+2-qsfp28",
		},
		"distributed-sfm-and-xiom": {
			chassis: "sr-2s",
			components: []*clabtypes.Component{
				{Slot: "A", SFM: "sfm-2s", Type: "cpm-2s"},
				{
					Slot: "1",
					SFM:  "sfm-2s",
					Type: "xcm-2s",
					XIOM: clabtypes.XIOMS{
						{Slot: 1, Type: "iom-s-3.0t", MDA: clabtypes.MDAS{{Slot: 1, Type: "ms8-100gb-sfpdd+2-100gb-qsfp28"}}},
					},
				},
			},
			want: "cp: chassis=sr-2s slot=A sfm=sfm-2s card=cpm-2s ___ " +
				"lc: chassis=sr-2s slot=1 sfm=sfm-2s card=xcm-2s xiom/x1=iom-s-3.0t mda/x1/1=ms8-100gb-sfpdd+2-100gb-qsfp28",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := buildSrosVariant(tc.chassis, tc.components, tc.env)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
