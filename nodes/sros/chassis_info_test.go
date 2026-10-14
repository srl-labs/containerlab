// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sros

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChassisInfoComponentWarnings(t *testing.T) {
	info, err := parseChassisInfo([]byte(`{
		"IXR-x": {
			"cpm": ["cpm-ixr-x"],
			"iom": ["imm6-qsfpdd+48-sfp56"],
			"iom_slots": 1,
			"mda": [null],
			"sfm": ["sfm-ixr-x"]
		},
		"SAR-Hx": {
			"cpm": ["iom-sar-1x"],
			"fixed_mdas": {"2": "fixed-mda"},
			"integrated": true,
			"mda": ["supported-mda"],
			"xiom": ["supported-xiom"]
		},
		"SAR-Mx": {"alias": "SAR-Hx"}
	}`))
	require.NoError(t, err)

	t.Run("supported configuration", func(t *testing.T) {
		warnings := info.componentWarnings("sar-mx", "", []*Component{{
			Slot: slotAName,
			Type: "iom-sar-1x",
			MDA: MDAS{
				{Slot: 1, Type: "supported-mda"},
				{Slot: 2, Type: "fixed-mda"},
			},
			XIOM: XIOMS{{Slot: 1, Type: "supported-xiom"}},
		}})

		assert.Empty(t, warnings)
	})

	t.Run("unsupported values warn", func(t *testing.T) {
		warnings := info.componentWarnings("IXR-x", "wrong-sfm", []*Component{
			{Slot: slotAName, Type: "wrong-cpm"},
			{Slot: "2", Type: "wrong-iom", MDA: MDAS{{Slot: 1, Type: "wrong-mda"}}},
		})

		assert.Len(t, warnings, 5)
		assert.Contains(t, warnings[0], "SFM type")
		assert.Contains(t, warnings[1], "component type")
		assert.Contains(t, warnings[2], "exceeds")
		assert.Contains(t, warnings[3], "component type")
		assert.Contains(t, warnings[4], "MDA type")
	})

	t.Run("fixed MDA mismatch warns", func(t *testing.T) {
		warnings := info.componentWarnings("SAR-Hx", "", []*Component{{
			Slot: slotAName,
			MDA:  MDAS{{Slot: 2, Type: "wrong-mda"}},
		}})

		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "does not match fixed MDA")
	})

	t.Run("nested XIOM MDA is not guessed", func(t *testing.T) {
		warnings := info.componentWarnings("SAR-Hx", "", []*Component{{
			Slot: slotAName,
			XIOM: XIOMS{{
				Slot: 1,
				Type: "supported-xiom",
				MDA:  MDAS{{Slot: 1, Type: "not-in-chassis-mda-list"}},
			}},
		}})

		assert.Empty(t, warnings)
	})
}

func TestParseChassisInfoRejectsInvalidJSON(t *testing.T) {
	_, err := parseChassisInfo([]byte(`{"SR-1":`))
	require.Error(t, err)
}
