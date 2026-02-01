// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sros

import (
	"fmt"
	"strings"

	clabtypes "github.com/srl-labs/containerlab/types"
)

// componentCfgLine represents one SR OS config line for a component (card, sfm, xiom, mda).
// Adding a new component type = add a Kind and a case in String().
type componentCfgLine struct {
	Kind     string // "card", "sfm", "xiom", "xiomMda", "mda"
	Slot     string
	Type     string
	XiomSlot int
	MdaSlot  int
}

// String returns the SR OS CLI config line for this component.
func (l componentCfgLine) String() string {
	switch l.Kind {
	case "card":
		return fmt.Sprintf("/configure card %s card-type %s admin-state enable\n", l.Slot, l.Type)
	case "sfm":
		return fmt.Sprintf("/configure sfm %s sfm-type %s admin-state enable\n", l.Slot, l.Type)
	case "xiom":
		return fmt.Sprintf("/configure card %s xiom x%d xiom-type %s admin-state enable\n",
			l.Slot, l.XiomSlot, l.Type)
	case "xiomMda":
		return fmt.Sprintf("/configure card %s xiom x%d mda %d mda-type %s admin-state enable\n",
			l.Slot, l.XiomSlot, l.MdaSlot, l.Type)
	case "mda":
		return fmt.Sprintf("/configure card %s mda %d mda-type %s admin-state enable\n",
			l.Slot, l.MdaSlot, l.Type)
	default:
		return ""
	}
}

// buildComponentCfgLines turns root components into a slice of config lines (card, sfm, xiom, xiomMda, mda).
// CPM slots (A, B) are skipped. Components without Type are skipped (caller may log).
func buildComponentCfgLines(components []*clabtypes.Component) []componentCfgLine {
	var lines []componentCfgLine
	for _, component := range components {
		slot := strings.ToUpper(strings.TrimSpace(component.Slot))
		if slot == slotAName || slot == slotBName {
			continue
		}
		if component.Type == "" {
			continue
		}
		lines = append(lines, componentCfgLine{Kind: "card", Slot: slot, Type: component.Type})
		if component.SFM != "" {
			lines = append(lines, componentCfgLine{Kind: "sfm", Slot: slot, Type: component.SFM})
		}
		for _, xiom := range component.XIOM {
			if xiom.Type != "" {
				lines = append(lines, componentCfgLine{Kind: "xiom", Slot: slot, Type: xiom.Type, XiomSlot: xiom.Slot})
			}
			for _, mda := range xiom.MDA {
				if mda.Type != "" {
					lines = append(lines, componentCfgLine{
						Kind: "xiomMda", Slot: slot, Type: mda.Type,
						XiomSlot: xiom.Slot, MdaSlot: mda.Slot,
					})
				}
			}
		}
		for _, mda := range component.MDA {
			if mda.Type != "" {
				lines = append(lines, componentCfgLine{Kind: "mda", Slot: slot, Type: mda.Type, MdaSlot: mda.Slot})
			}
		}
	}
	return lines
}
