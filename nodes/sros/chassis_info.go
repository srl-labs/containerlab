// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sros

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// DefaultChassisInfoPath is the path to chassis_info.json inside SROS/SR SIM container images.
const DefaultChassisInfoPath = "/opt/nokia/chassis_info.json"

// chassisInfoFileLicense is prepended to the chassis_info.json file written to LabDir.
const chassisInfoFileLicense = "// Copyright 2026 Nokia\n// Licensed under the BSD 3-Clause License.\n// SPDX-License-Identifier: BSD-3-Clause\n\n"

type chassisInfoEntry struct {
	Alias      string            `json:"alias"`
	CPM        []string          `json:"cpm"`
	IOM        []string          `json:"iom"`
	IOMSlots   int               `json:"iom_slots"`
	MDA        []*string         `json:"mda"`
	FixedMDAs  map[string]string `json:"fixed_mdas"`
	SFM        []string          `json:"sfm"`
	XIOM       []string          `json:"xiom"`
	FixedHWCfg string            `json:"fixed_hw_cfg"`
	Integrated bool              `json:"integrated"`
	Redundant  bool              `json:"redundant"`
}

type chassisInfo map[string]chassisInfoEntry

func parseChassisInfo(content []byte) (chassisInfo, error) {
	var info chassisInfo
	if err := json.Unmarshal(content, &info); err != nil {
		return nil, fmt.Errorf("parse chassis_info.json: %w", err)
	}
	return info, nil
}

func (i chassisInfo) entry(nodeType string) (chassisInfoEntry, bool) {
	key := ""
	for candidate := range i {
		if strings.EqualFold(candidate, strings.TrimSpace(nodeType)) {
			key = candidate
			break
		}
	}
	if key == "" {
		return chassisInfoEntry{}, false
	}

	seen := map[string]struct{}{}
	for {
		if _, ok := seen[key]; ok {
			return chassisInfoEntry{}, false
		}
		seen[key] = struct{}{}

		entry := i[key]
		if entry.Alias == "" {
			return entry, true
		}

		key = ""
		for candidate := range i {
			if strings.EqualFold(candidate, entry.Alias) {
				key = candidate
				break
			}
		}
		if key == "" {
			return chassisInfoEntry{}, false
		}
	}
}

// componentWarnings returns best-effort validation warnings. It deliberately skips checks
// that chassis_info.json cannot express, such as which MDA types belong under a specific XIOM.
func (i chassisInfo) componentWarnings(nodeType, sfm string, components []*Component) []string {
	entry, ok := i.entry(nodeType)
	if !ok {
		return []string{
			fmt.Sprintf(
				"chassis type %q is not present in chassis_info.json; component compatibility was not checked",
				nodeType,
			),
		}
	}

	var warnings []string
	if sfm != "" && len(entry.SFM) > 0 && !slices.Contains(entry.SFM, sfm) {
		warnings = append(warnings, fmt.Sprintf(
			"SFM type %q is not listed for chassis type %q",
			sfm,
			nodeType,
		))
	}

	for _, component := range components {
		if component == nil {
			continue
		}
		slot := strings.ToUpper(strings.TrimSpace(component.Slot))
		if slot == "" {
			slot = slotAName
		}

		switch {
		case slot == slotAName || slot == slotBName:
			if slot == slotBName && !entry.Redundant {
				warnings = append(warnings, fmt.Sprintf(
					"component slot %s is not listed as supported for chassis type %q",
					slot,
					nodeType,
				))
			}
			if component.Type != "" &&
				entry.FixedHWCfg != "" &&
				component.Type != entry.FixedHWCfg {
				warnings = append(warnings, fmt.Sprintf(
					"component type %q in slot %s does not match fixed hardware %q for chassis type %q",
					component.Type,
					slot,
					entry.FixedHWCfg,
					nodeType,
				))
			} else if component.Type != "" &&
				entry.FixedHWCfg == "" &&
				len(entry.CPM) > 0 &&
				!slices.Contains(entry.CPM, component.Type) {
				warnings = append(warnings, fmt.Sprintf(
					"component type %q in slot %s is not listed for chassis type %q",
					component.Type,
					slot,
					nodeType,
				))
			}
		default:
			numericSlot, err := strconv.Atoi(slot)
			if err == nil {
				if entry.IOMSlots > 0 && numericSlot > entry.IOMSlots {
					warnings = append(warnings, fmt.Sprintf(
						"component slot %s exceeds the %d IOM slots listed for chassis type %q",
						slot,
						entry.IOMSlots,
						nodeType,
					))
				}
				if component.Type != "" &&
					len(entry.IOM) > 0 &&
					!slices.Contains(entry.IOM, component.Type) {
					warnings = append(warnings, fmt.Sprintf(
						"component type %q in slot %s is not listed for chassis type %q",
						component.Type,
						slot,
						nodeType,
					))
				}
			}
		}

		for _, xiom := range component.XIOM {
			if xiom.Type != "" &&
				len(entry.XIOM) > 0 &&
				!slices.Contains(entry.XIOM, xiom.Type) {
				warnings = append(warnings, fmt.Sprintf(
					"XIOM type %q in component slot %s is not listed for chassis type %q",
					xiom.Type,
					slot,
					nodeType,
				))
			}
		}

		if entry.Integrated || (slot != slotAName && slot != slotBName) {
			warnings = append(warnings, validateMDAs(nodeType, entry, component.MDA)...)
		}
	}

	return warnings
}

func validateMDAs(nodeType string, entry chassisInfoEntry, mdas MDAS) []string {
	var warnings []string
	for _, mda := range mdas {
		if fixed, exists := entry.FixedMDAs[strconv.Itoa(mda.Slot)]; exists {
			if mda.Type != fixed {
				warnings = append(warnings, fmt.Sprintf(
					"MDA type %q in slot %d does not match fixed MDA %q for chassis type %q",
					mda.Type,
					mda.Slot,
					fixed,
					nodeType,
				))
			}
			continue
		}
		if mda.Type != "" && !containsChassisType(entry.MDA, mda.Type) {
			warnings = append(warnings, fmt.Sprintf(
				"MDA type %q in slot %d is not listed for chassis type %q",
				mda.Type,
				mda.Slot,
				nodeType,
			))
		}
	}
	return warnings
}

func containsChassisType(types []*string, value string) bool {
	for _, typ := range types {
		if typ != nil && *typ == value {
			return true
		}
	}
	return false
}
