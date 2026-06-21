package vr_sros

import (
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComponentInterfaceIndex(t *testing.T) {
	// distributed SR-2s: two line cards, each xcm-2s with one XIOM holding mda1 (18 ports) and
	// mda2 (24 ports) -> max_nics 42 per line card.
	sr2s := []*clabtypes.Component{
		{Slot: "A", Type: "cpm-2s"},
		{Slot: "1", Type: "xcm-2s", XIOM: clabtypes.XIOMS{
			{Slot: 1, Type: "iom-s-3.0t", MDA: clabtypes.MDAS{
				{Slot: 1, Type: "ms18-100gb-qsfp28"},
				{Slot: 2, Type: "ms24-10/100gb-sfpdd"},
			}},
		}},
		{Slot: "2", Type: "xcm-2s", XIOM: clabtypes.XIOMS{
			{Slot: 1, Type: "iom-s-3.0t", MDA: clabtypes.MDAS{
				{Slot: 1, Type: "ms18-100gb-qsfp28"},
				{Slot: 2, Type: "ms24-10/100gb-sfpdd"},
			}},
		}},
	}

	// distributed chassis with two direct-MDA line cards, 6 ports each.
	directLCs := []*clabtypes.Component{
		{Slot: "A", Type: "cpm5"},
		{Slot: "1", Type: "iom4-e", MDA: clabtypes.MDAS{{Slot: 1, Type: "me6-10gb-sfp+"}}},
		{Slot: "2", Type: "iom4-e", MDA: clabtypes.MDAS{{Slot: 1, Type: "me6-10gb-sfp+"}}},
	}

	// integrated sr-1 with two direct MDAs, card in the default (unset -> A) slot, ports on slot 1.
	integratedDirect := []*clabtypes.Component{
		{MDA: clabtypes.MDAS{
			{Slot: 1, Type: "me12-100gb-qsfp28"},
			{Slot: 2, Type: "me16-25gb-sfp28+2-100gb-qsfp28"},
		}},
	}

	// integrated ixr-x with an embedded IMM card and no MDA list (single implicit port group).
	integratedEmbedded := []*clabtypes.Component{
		{Slot: "A", Type: "cpm-ixr-x/imm6-qsfpdd+48-sfp56"},
	}

	tests := map[string]struct {
		components []*clabtypes.Component
		ifName     string
		want       int
		wantErr    bool
	}{
		"xiom lc1 mda1 first port":  {sr2s, "1/x1/1/1", 1, false},
		"xiom lc1 mda1 last port":   {sr2s, "1/x1/1/18", 18, false},
		"xiom lc1 mda2 first port":  {sr2s, "1/x1/2/1", 19, false},
		"xiom lc1 mda2 last port":   {sr2s, "1/x1/2/24", 42, false},
		"xiom lc2 mda1 first port":  {sr2s, "2/x1/1/1", 43, false},
		"xiom lc2 mda2 first port":  {sr2s, "2/x1/2/1", 61, false},
		"direct lc1 port":           {directLCs, "1/1/3", 3, false},
		"direct lc2 port offset":    {directLCs, "2/1/1", 7, false},
		"connector lc1 c1":          {sr2s, "1/x1/1/c1/1", 1, false},
		"connector lc1 c18":         {sr2s, "1/x1/1/c18/1", 18, false},
		"connector lc1 mda2 c1":     {sr2s, "1/x1/2/c1/1", 19, false},
		"connector lc2 c1":          {sr2s, "2/x1/1/c1/1", 43, false},
		"breakout subport rejected": {sr2s, "1/x1/1/c1/2", 0, true},
		// xiom omitted in the alias must map the same as the explicit-xiom form.
		"xiom omitted mda1":         {sr2s, "1/1/c3/1", 3, false},
		"xiom explicit mda1":        {sr2s, "1/x1/1/c3/1", 3, false},
		"xiom omitted mda2":         {sr2s, "1/2/c18/1", 36, false},
		"xiom omitted lc2 mda2":     {sr2s, "2/x1/2/c24/1", 84, false},
		"integrated direct mda1":    {integratedDirect, "1/1/c1/1", 1, false},
		"integrated direct mda1b":   {integratedDirect, "1/1/c10/1", 10, false},
		"integrated direct mda2":    {integratedDirect, "1/2/c1/1", 13, false},
		"integrated embedded c1":    {integratedEmbedded, "1/1/c1/1", 1, false},
		"integrated embedded c34":   {integratedEmbedded, "1/1/c34/1", 34, false},
		"integrated bad slot":       {integratedDirect, "2/1/c1/1", 0, true},
		"unknown slot":              {directLCs, "3/1/1", 0, true},
		"unknown mda":               {directLCs, "1/2/1", 0, true},
		"xiom alias on direct card": {directLCs, "1/x1/1/1", 0, true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := componentInterfaceIndex(tc.components, tc.ifName)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
