// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sros

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
)

const integratedSrosCardSlot = "1"

type integratedSrosDefaultComponent struct {
	cardType     string
	mdas         MDAS
	allowedSlots []string
}

var integratedSrosDefaultComponents = map[string]integratedSrosDefaultComponent{
	"sr-1": {
		cardType:     "iom-1",
		allowedSlots: []string{slotAName},
		mdas: MDAS{
			{Slot: 1, Type: "me6-100gb-qsfp28"},
			{Slot: 2, Type: "me12-100gb-qsfp28"},
		},
	},
	"sr-1s": {
		cardType:     "xcm-1s",
		allowedSlots: []string{slotAName},
		mdas: MDAS{
			{Slot: 1, Type: "s36-100gb-qsfp28"},
		},
	},
	"ixr-r6": {
		cardType:     "iom-ixr-r6",
		allowedSlots: []string{slotAName, slotBName},
		mdas: MDAS{
			{Slot: 1, Type: "m6-10g-sfp++1-100g-qsfp28"},
		},
	},
	"ixr-e2": {
		cardType:     "imm2-qsfpdd+2-qsfp28+24-sfp28",
		allowedSlots: []string{slotAName},
		mdas: MDAS{
			{Slot: 1, Type: "m2-qsfpdd+2-qsfp28+24-sfp28"},
		},
	},
	"ixr-e2c": {
		cardType:     "imm12-sfp28+2-qsfp28",
		allowedSlots: []string{slotAName},
		mdas: MDAS{
			{Slot: 1, Type: "m12-sfp28+2-qsfp28"},
		},
	},
	"ixr-e2n": {
		cardType:     "imm4-sfp+4-sfp+",
		allowedSlots: []string{slotAName},
		mdas: MDAS{
			{Slot: 1, Type: "m4-sfp+4-sfp+"},
		},
	},
	"ixr-e2n-s": {
		cardType:     "imm4-sfp+4-sfp+-s",
		allowedSlots: []string{slotAName},
		mdas: MDAS{
			{Slot: 1, Type: "m4-sfp+4-sfp+-s"},
		},
	},
	"ixr-e3c": {
		cardType:     "imm4-qsfp28+16-sfp28+8-sfp56",
		allowedSlots: []string{slotAName},
		mdas: MDAS{
			{Slot: 1, Type: "m4-qsfp28+16-sfp28+8-sfp56"},
		},
	},
	"ixr-e3x": {
		cardType:     "imm16-sfp112+15-sfp56+6-qsfpdd",
		allowedSlots: []string{slotAName},
		mdas: MDAS{
			{Slot: 1, Type: "m16-sfp112+15-sfp56+6-qsfpdd"},
		},
	},
	"ixr-e2sc": {
		cardType:     "imm16-sfp++4-sfp28",
		allowedSlots: []string{slotAName},
		mdas: MDAS{
			{Slot: 1, Type: "m16-sfp++4-sfp28"},
		},
	},
	"ixr-ec": {
		cardType:     "imm4-1g-tx+20-1g-sfp+6-10g-sfp+",
		allowedSlots: []string{slotAName},
		mdas: MDAS{
			{Slot: 1, Type: "m4-1g-tx+20-1g-sfp+6-10g-sfp+"},
		},
	},
}

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

// buildComponentCfgLines turns root components into a slice of config lines (card, sfm, xiom,
// xiomMda, mda).
// CPM slots (A, B) are skipped. Components without Type are skipped (caller may log).
func buildComponentCfgLines(components []*Component, sfm string) []componentCfgLine {
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
		if sfm != "" {
			lines = append(lines, componentCfgLine{Kind: "sfm", Slot: slot, Type: sfm})
		}
		for _, xiom := range component.XIOM {
			if xiom.Type != "" {
				lines = append(
					lines,
					componentCfgLine{
						Kind:     "xiom",
						Slot:     slot,
						Type:     xiom.Type,
						XiomSlot: xiom.Slot,
					},
				)
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
				lines = append(
					lines,
					componentCfgLine{Kind: "mda", Slot: slot, Type: mda.Type, MdaSlot: mda.Slot},
				)
			}
		}
	}
	return lines
}

func buildIntegratedComponentCfgLines(nodeType string, override *Component) []componentCfgLine {
	component, ok := integratedSrosDefaultComponents[canonicalSrosNodeType(nodeType)]
	if !ok {
		return nil
	}
	if override == nil {
		override = &Component{}
	}

	defaultMdas := component.mdas

	if len(override.XIOM) > 0 {
		defaultMdas = nil
	}

	card := &Component{
		Slot: integratedSrosCardSlot,
		Type: component.cardType,
		XIOM: override.XIOM,
		MDA:  mergeIntegratedMdas(defaultMdas, override.MDA),
	}

	return buildComponentCfgLines([]*Component{card}, "")
}

func mergeIntegratedMdas(defaults, overrides MDAS) MDAS {
	bySlot := map[int]string{}
	for _, mda := range slices.Concat(defaults, overrides) {
		if mda.Slot > 0 && mda.Type != "" {
			bySlot[mda.Slot] = mda.Type
		}
	}

	slots := slices.Sorted(maps.Keys(bySlot))
	mdas := make(MDAS, 0, len(slots))
	for _, slot := range slots {
		mdas = append(mdas, MDA{Slot: slot, Type: bySlot[slot]})
	}
	return mdas
}

func canonicalSrosNodeType(nodeType string) string {
	return strings.ToLower(strings.TrimSpace(nodeType))
}

func integratedSrosDefault(nodeType string) (integratedSrosDefaultComponent, bool) {
	component, ok := integratedSrosDefaultComponents[canonicalSrosNodeType(nodeType)]
	return component, ok
}

func isIntegratedSrosNodeType(nodeType string) bool {
	_, ok := integratedSrosDefault(nodeType)
	return ok
}

func integratedSrosAllowedSlots(nodeType string) []string {
	component, ok := integratedSrosDefault(nodeType)
	if !ok || len(component.allowedSlots) == 0 {
		return []string{slotAName}
	}
	return component.allowedSlots
}

func isRedundantIntegratedSrosComponents(nodeType string, components []*Component) bool {
	if !isIntegratedSrosNodeType(nodeType) || len(components) < 2 {
		return false
	}
	for _, c := range components {
		if strings.TrimSpace(c.Slot) == "" || !integratedSrosSlotAllowed(nodeType, c.Slot) {
			return false
		}
	}
	return true
}

func integratedSrosSlotAllowed(nodeType, slot string) bool {
	slot = strings.ToUpper(strings.TrimSpace(slot))
	if slot == "" {
		slot = standaloneSlotName
	}
	return slices.Contains(integratedSrosAllowedSlots(nodeType), slot)
}

func (n *sros) generateComponentConfig() string {
	if !n.kindSpecificCfg().GenComponentConfig {
		return ""
	}
	if n.isConfigClassic() {
		return ""
	}

	if isIntegratedSrosNodeType(n.Cfg.NodeType) {
		return n.generateIntegratedComponentConfig()
	}

	components := n.rootComponents
	if len(components) == 0 {
		return ""
	}

	for _, c := range components {
		slot := strings.ToUpper(strings.TrimSpace(c.Slot))
		if slot == slotAName || slot == slotBName {
			continue
		}
		if c.Type == "" {
			log.Warn(
				"SR-SIM node has no type set for component in slot, skipping component SR OS config generation.",
				"node",
				n.Cfg.ShortName,
				"slot",
				slot,
			)
		}
	}

	lines := buildComponentCfgLines(components, n.kindSpecificCfg().SFM)
	return n.componentConfigFromLines(lines)
}

func (n *sros) componentConfigFromLines(lines []componentCfgLine) string {
	powerConfig := n.generatePowerConfig()
	if len(lines) == 0 && powerConfig == "" {
		return ""
	}

	var config strings.Builder
	for _, l := range lines {
		config.WriteString(l.String())
	}
	config.WriteString(powerConfig)
	return config.String()
}

func (n *sros) generateIntegratedComponentConfig() string {
	card := strings.TrimSpace(n.Cfg.Env[envNokiaSrosCard])
	if card != "" {
		log.Info(
			"Card type override set on integrated chassis type, skipping component SR OS config generation",
			"node", n.Cfg.ShortName,
			"type", n.Cfg.NodeType,
			"card", card,
		)
		return ""
	}

	lines := buildIntegratedComponentCfgLines(n.Cfg.NodeType, n.integratedComponentOverride())
	return n.componentConfigFromLines(lines)
}

// integratedComponentOverride returns the component that overrides the integrated defaults:
// the single component of a standalone node, or the component matching the slot of a
// redundant CPM node.
func (n *sros) integratedComponentOverride() *Component {
	if n.rootCtrName == "" {
		if len(n.kindSpecificCfg().Components) == 1 {
			return n.kindSpecificCfg().Components[0]
		}
		return nil
	}

	slot := n.Cfg.Env[envNokiaSrosSlot]
	for _, c := range n.rootComponents {
		if strings.EqualFold(strings.TrimSpace(c.Slot), slot) {
			return c
		}
	}
	return nil
}
